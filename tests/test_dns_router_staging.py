"""P0.2: offline DNS Router package builder concurrency/atomicity regressions.

The tests exercise the real POSIX shell builder against a miniature throwaway
fixture repository and mocked Go/npm; no router, GitHub, network or real builds.
"""
from __future__ import annotations

import hashlib
import io
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest


BUNDLE_ROOT = Path(__file__).resolve().parents[2]
BUNDLED_BUILDER = BUNDLE_ROOT / 'payload' / 'scripts' / 'build-dns-router-experimental.sh'
BUILDER = (BUNDLED_BUILDER if BUNDLED_BUILDER.is_file() else Path(__file__).resolve().parents[1] / 'scripts' / 'build-dns-router-experimental.sh')
SHELL_SUPPORT = (
    sys.platform != 'win32'
    and all(shutil.which(x) for x in ('sh', 'flock', 'tar', 'mktemp', 'python3'))
)


def write(path: Path, content: str, executable: bool = False) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding='utf-8')
    if executable:
        path.chmod(0o755)


def fixture_repo(root: Path) -> tuple[Path, dict[str, str]]:
    builder = root / 'scripts' / 'build-dns-router-experimental.sh'
    builder.parent.mkdir(parents=True, exist_ok=True)
    builder.write_bytes(BUILDER.read_bytes())
    write(root / 'scripts' / 'target-env.sh', '''#!/bin/sh
routerforge_target_init() { RF_OPKG_ARCH="$1"; export RF_OPKG_ARCH; }
routerforge_go() {
  out=''; previous=''
  for part in "$@"; do
    if [ "$previous" = '-o' ]; then out="$part"; fi
    previous="$part"
  done
  [ -n "$out" ] || { echo 'MOCK_GO_NO_OUTPUT' >&2; return 7; }
  mkdir -p "$(dirname "$out")"
  sleep "${RF_TEST_GO_SLEEP:-0.2}"
  printf '#!/bin/sh\\necho MOCK_DNS_BINARY\\n' > "$out"
  chmod 755 "$out"
}
''')
    write(root / 'scripts' / 'collect-dns-lite-licenses.py', '''import pathlib, sys
out = pathlib.Path(sys.argv[2]); out.mkdir(parents=True, exist_ok=True)
(out / 'license.txt').write_text('fixture')
''')
    write(root / 'scripts' / 'verify-beta-dns-router-package.py', '''import os, pathlib, sys
if os.environ.get('RF_TEST_VALIDATION_FAIL') == '1':
    raise SystemExit('MOCK_DEV_VALIDATION_FAIL')
p=pathlib.Path(sys.argv[1])
assert p.is_file() and p.stat().st_size > 0
''')
    write(root / 'modules' / 'dns-router-runtime' / 'packaging' / 'example.json', '{}')
    for file in ('S95routerforge-dns-router', 'S95routerforge-dns-router-dev',
                 '40-routerforge-dns-controller.sh', 'postinst-dev', 'prerm-dev'):
        write(root / 'modules' / 'dns-router-runtime' / 'packaging' / file,
              '#!/bin/sh\nexit 0\n', True)
    write(root / 'modules' / 'dns-router-runtime' / 'packaging' / 'QUIC-GO-LICENSE', 'test-license')
    write(root / 'LICENSE', 'test-license')
    for channel, directory in [('dev', 'dns-router-lite'), ('experimental', 'dns-router-runtime')]:
        ui = root / 'modules' / directory / 'frontend'
        write(ui / 'index.html', '<html><script defer src="svelte-mode.js"></script></html>')
        write(ui / 'module.css', 'body{display:block}')
        if channel == 'dev':
            write(ui / 'p22-visual.css', 'body{display:block}')
            write(ui / 'p25-settings.css', 'body{display:block}')
        else:
            write(ui / 'app.js', '// experimental runtime fixture')
        write(ui / 'package-lock.json', '{"lockfileVersion":3,"packages":{}}')
    fake_bin = root / 'fakebin'
    write(fake_bin / 'npm', '''#!/bin/sh
set -eu
if [ "$1" = ci ]; then
  [ -s package-lock.json ] || { echo 'MOCK_NPM_LOCKFILE_REQUIRED' >&2; exit 9; }
  mkdir -p node_modules
  exit 0
fi
[ "$1" = run ] && [ "$2" = build ] || { echo 'MOCK_NPM_UNEXPECTED_COMMAND' >&2; exit 9; }
# A shared frontend build must not run twice simultaneously.
mkdir .mock_build_active || { echo 'MOCK_NPM_UI_COLLISION' >&2; exit 17; }
trap 'rmdir .mock_build_active' 0
mkdir -p build
sleep "${RF_TEST_NPM_SLEEP:-0.3}"
python3 - <<'PY_MOCK_JS'
from pathlib import Path
kind = 'DNS_LITE_SVELTE_MODE_MOUNT_MISSING' if 'dns-router-lite' in str(Path.cwd()) else 'DNS_RUNTIME_SVELTE_MODE_MOUNT_MISSING'
Path('build/svelte-mode.js').write_text('// ' + kind + '\\n' + 'x' * 3500)
PY_MOCK_JS
''', True)
    env = dict(os.environ, PATH=str(fake_bin) + os.pathsep + os.environ.get('PATH', ''),
               ROUTERFORGE_UPX='0', RF_TEST_GO_SLEEP='0.3', RF_TEST_NPM_SLEEP='0.3')
    return builder, env


def package_ui_modes(path: Path) -> dict[str, int]:
    with tarfile.open(path, 'r:gz') as pkg:
        blob = pkg.extractfile(next(m for m in pkg if m.name.lstrip('./') == 'data.tar.gz')).read()
    with tarfile.open(fileobj=io.BytesIO(blob), mode='r:gz') as data:
        return {m.name.lstrip('./'): m.mode for m in data if m.isfile()}


def package_content(path: Path) -> tuple[str, dict[str, bytes]]:
    with tarfile.open(path, 'r:gz') as pkg:
        control_blob = pkg.extractfile(next(m for m in pkg if m.name.lstrip('./') == 'control.tar.gz')).read()
        data_blob = pkg.extractfile(next(m for m in pkg if m.name.lstrip('./') == 'data.tar.gz')).read()
    with tarfile.open(fileobj=io.BytesIO(control_blob), mode='r:gz') as control:
        meta = control.extractfile(next(m for m in control if m.name.lstrip('./') == 'control')).read().decode()
    with tarfile.open(fileobj=io.BytesIO(data_blob), mode='r:gz') as data:
        files = {m.name.lstrip('./'): data.extractfile(m).read() for m in data if m.isfile()}
    return meta, files


class DNSPackageStagingSourceTests(unittest.TestCase):
    def test_atomic_output_and_private_staging(self):
        s = BUILDER.read_text(encoding='utf-8')
        self.assertIn('mktemp -d ', s)
        self.assertIn('mktemp ', s)
        self.assertIn('flock -x -w 300 8', s)
        self.assertIn('flock -x -w 300 9', s)
        self.assertIn('DNS_ROUTER_UI_LOCK_TIMEOUT', s)
        self.assertIn('mv -f "$OUT_TMP" "$OUT"', s)
        self.assertIn('python3 - "$OUT_TMP"', s)
        self.assertIn('"$OUT_TMP" "$TARGET" "$VERSION"', s)
        self.assertNotIn('WORK="$ROOT/dist/${PACKAGE}-experimental-${TARGET}-work"', s)
        self.assertNotIn('rm -rf "$WORK"\nmkdir -p', s)


@unittest.skipUnless(SHELL_SUPPORT, 'Linux/POSIX shell, flock and tar required for offline package acceptance')
class DNSPackageStagingIntegrationTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='rf-p02-stage-')
        self.root = Path(self.tmp.name)
        self.builder, self.env = fixture_repo(self.root)

    def tearDown(self):
        self.tmp.cleanup()

    def cmd(self, channel: str, version: str, extra: dict | None = None):
        env = dict(self.env, ROUTERFORGE_CHANNEL=channel)
        if extra:
            env.update(extra)
        return ['sh', str(self.builder), version, 'aarch64-3.10'], env

    def run_build(self, channel: str, version: str, extra: dict | None = None):
        cmd, env = self.cmd(channel, version, extra)
        return subprocess.run(cmd, text=True, capture_output=True, env=env, timeout=35)

    def assert_no_work(self):
        leftovers = list((self.root / 'dist').glob('.routerforge-dns-router-*-work.*'))
        leftovers += list((self.root / 'dist').glob('.routerforge-dns-router-package.*'))
        self.assertEqual([], leftovers, f'leftover staging: {leftovers}')

    def test_parallel_dev_and_experimental_isolated(self):
        cmd_a, env_a = self.cmd('dev', '0.0.0-p02-dev')
        cmd_b, env_b = self.cmd('experimental', '0.0.0-p02-exp')
        p_a = subprocess.Popen(cmd_a, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env_a)
        p_b = subprocess.Popen(cmd_b, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env_b)
        out_a, err_a = p_a.communicate(timeout=35)
        out_b, err_b = p_b.communicate(timeout=35)
        self.assertEqual(0, p_a.returncode, out_a + err_a)
        self.assertEqual(0, p_b.returncode, out_b + err_b)
        for channel, version in [('dev','0.0.0-p02-dev'),('experimental','0.0.0-p02-exp')]:
            out = self.root / 'dist' / f'routerforge-dns-router_{version}_aarch64-3.10.ipk'
            self.assertTrue(out.is_file())
            meta, files = package_content(out)
            self.assertIn('Version: ' + version, meta)
            ui = 'opt/share/routerforge/modules/dns-router/ui/'
            asset = files[ui + 'svelte-mode.js']
            self.assertIn(('DNS_LITE_' if channel == 'dev' else 'DNS_RUNTIME_').encode(), asset)
            modes = package_ui_modes(out)
            for web_file in ('index.html', 'module.css', 'svelte-mode.js'):
                self.assertTrue(modes[ui + web_file] & 0o004,
                                f'UI asset lost world-readable mode: {web_file}')
            self.assertIn(('"mode": "independent-dns"' if channel == 'dev' else '"mode": "manual-shadow-dns-opt-in"').encode(), files['opt/share/routerforge/modules/dns-router/manifest.json'])
        self.assert_no_work()

    def test_parallel_same_ui_different_versions_serialize(self):
        builds = [self.cmd('dev', f'0.0.0-p02-serial-{x}') for x in ('a','b')]
        ps = [subprocess.Popen(cmd, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env) for cmd, env in builds]
        for p in ps:
            out, err = p.communicate(timeout=35)
            self.assertEqual(0, p.returncode, out + err)
            self.assertIn('DNS_ROUTER_LOCKS_ACQUIRED', out)
        for x in ('a','b'):
            self.assertTrue((self.root/'dist'/f'routerforge-dns-router_0.0.0-p02-serial-{x}_aarch64-3.10.ipk').is_file())
        self.assert_no_work()

    def test_failed_validation_preserves_previous_output(self):
        target = self.root/'dist'/'routerforge-dns-router_0.0.0-p02-atomic_aarch64-3.10.ipk'
        ok = self.run_build('dev','0.0.0-p02-atomic')
        self.assertEqual(0,ok.returncode,ok.stdout+ok.stderr)
        digest = hashlib.sha256(target.read_bytes()).hexdigest()
        failed = self.run_build('dev','0.0.0-p02-atomic', {'RF_TEST_VALIDATION_FAIL':'1'})
        self.assertNotEqual(0, failed.returncode)
        self.assertIn('MOCK_DEV_VALIDATION_FAIL', failed.stdout + failed.stderr)
        self.assertEqual(digest, hashlib.sha256(target.read_bytes()).hexdigest())
        self.assert_no_work()
        retry = self.run_build('dev','0.0.0-p02-atomic')
        self.assertEqual(0,retry.returncode,retry.stdout+retry.stderr)

    def test_symlink_output_lock_refused(self):
        dist=self.root/'dist';dist.mkdir(parents=True,exist_ok=True)
        sentinel=self.root/'no-touch.txt';sentinel.write_text('untouched')
        lock=dist/'.routerforge-dns-router_0.0.0-p02-block_aarch64-3.10.ipk.lock'
        lock.symlink_to(sentinel)
        p=self.run_build('dev','0.0.0-p02-block')
        self.assertNotEqual(0,p.returncode)
        self.assertIn('DNS_ROUTER_SYMLINKED_BUILD_PATH_REJECTED',p.stdout+p.stderr)
        self.assertEqual('untouched',sentinel.read_text())


if __name__ == '__main__':
    unittest.main()
