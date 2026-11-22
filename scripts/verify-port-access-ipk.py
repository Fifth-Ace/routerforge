#!/usr/bin/env python3
"""P43: check the actual packaged IPK, not just source build declarations."""
import argparse
import io
import json
from pathlib import Path
import re
import tarfile

EXPECTED_UI = {'index.html','module.css','ui-rebuild.css',
    'p39-compact.css','p53-ux.css','p54-workspace.css','p55-antiscan.css','svelte-status.js'}
ROOT = 'opt/share/routerforge/modules/port-access-manager/'
EXECUTABLES = {'opt/bin/routerforge-port-access-manager',
    'opt/bin/routerforge-port-access-watchdog',
    'opt/etc/init.d/S95routerforge-port-access-manager',
    ROOT + 'odin-engine.sh', ROOT + 'engine-gate.sh'}
REQUIRED = EXECUTABLES | {ROOT+'ui/'+name for name in EXPECTED_UI} | {
    ROOT+'manifest.json', 'opt/share/licenses/routerforge-port-access-manager/LICENSE'}


def unpack(tarbytes, label):
    with tarfile.open(fileobj=io.BytesIO(tarbytes), mode='r:gz') as tf:
        entries = {}
        for member in tf.getmembers():
            raw = member.name
            path = raw.removeprefix('./')
            if not path or path == '.':
                continue
            parts = path.split('/')
            if path.startswith('/') or '\\' in path or '..' in parts or any(not p for p in parts):
                raise ValueError(f'{label}: unsafe path {raw!r}')
            if member.issym() or member.islnk() or member.isdev() or member.isfifo():
                raise ValueError(f'{label}: unsafe entry {raw!r}')
            if path in entries:
                raise ValueError(f'{label}: duplicate entry {raw!r}')
            if not (member.isdir() or member.isfile()):
                raise ValueError(f'{label}: unsupported entry {raw!r}')
            if member.mode & 0o6000:
                raise ValueError(f'{label}: setuid/setgid {raw!r}')
            entries[path] = (member, tf.extractfile(member).read() if member.isfile() else None)
        return entries


def verify(path):
    with tarfile.open(path, 'r:gz') as outer:
        members = outer.getmembers()
        names = [x.name.removeprefix('./') for x in members]
        if set(names) != {'debian-binary','control.tar.gz','data.tar.gz'} or len(names) != 3:
            raise ValueError('IPK outer members differ from expected three files')
        if any(not x.isfile() for x in members):
            raise ValueError('IPK outer contains non-file member')
        archive = {x.name.removeprefix('./'): outer.extractfile(x).read() for x in members}
    if archive['debian-binary'] != b'2.0\n':
        raise ValueError('incorrect ipk format version')
    control = unpack(archive['control.tar.gz'], 'control')
    data = unpack(archive['data.tar.gz'], 'data')
    files = {k for k,(m,_) in data.items() if m.isfile()}
    missing = REQUIRED - files
    if missing:
        raise ValueError('missing packaged files: ' + ', '.join(sorted(missing)))
    for name in EXECUTABLES:
        m, payload = data[name]
        if not (m.mode & 0o111) or not payload:
            raise ValueError('missing executable permission or empty: ' + name)
    for name in ('opt/bin/routerforge-port-access-manager','opt/bin/routerforge-port-access-watchdog'):
        if data[name][1][:4] != b'\x7fELF':
            raise ValueError('invalid ELF magic: ' + name)
    for name in ('postinst','prerm'):
        if name not in control or not (control[name][0].mode & 0o111):
            raise ValueError('missing control executable: '+name)
    meta = control.get('control')
    if not meta or b'Package: routerforge-port-access-manager\n' not in meta[1] or b'Depends: routerforge-core\n' not in meta[1]:
        raise ValueError('wrong package metadata')
    manifest = json.loads(data[ROOT+'manifest.json'][1])
    if any(manifest.get(k) != v for k,v in {
        'id':'port-access-manager', 'socket':'/opt/var/run/routerforge-port-access-manager.sock',
        'api_base':'/api/modules/port-access-manager',
        'ui_entry':'/api/modules/port-access-manager/ui/index.html'}.items()):
        raise ValueError('manifest contract mismatch')
    html = data[ROOT+'ui/index.html'][1].decode('utf-8')
    scripts = re.findall(r'<script\s+src="([^\"]+)"', html)
    if scripts != ['svelte-status.js']:
        raise ValueError('expected exactly one compiled Svelte JS entrypoint')
    if any(ROOT+'ui/'+x not in files for x in scripts):
        raise ValueError('JS asset missing')
    if ROOT+'ui/operation-log.js' in files:
        raise ValueError('legacy operation log controller shipped')
    if any(x in files for x in (ROOT+'ui/config-editor.js',ROOT+'ui/engine-wizard.js')):
        raise ValueError('legacy engine controller shipped')
    if ROOT+'ui/odin-ui.js' in files:
        raise ValueError('legacy odin controller still packaged')
    if ROOT+'ui/app.js' in files:
        raise ValueError('legacy Port Access app.js still packaged')
    for name in ('p54-workspace.js','p55-antiscan.js'):
        if ROOT+'ui/'+name in files or '<script src=\"'+name+'\">' in html:
            raise ValueError('legacy layout controller still shipped: ' + name)
    for name in ('ui-rebuild.js','p39-compact.js','p53-ux.js'):
        if ROOT+'ui/'+name in files or '<script src=\"'+name+'\">' in html:
            raise ValueError('legacy layout controller still shipped: ' + name)
    bundle = data[ROOT+'ui/svelte-status.js'][1]
    # P43 / Svelte 5 single-root migration: these concrete compiled UI and API
    # markers replace obsolete independent-root exception strings. The old
    # PORT_ACCESS_*_MOUNT_MISSING errors were removed with the six-root bridge.
    # Keep fail-closed checks on the *actual data.tar.gz payload* instead of
    # accepting a source-only import or merely the presence of svelte-status.js.
    required_bundle_markers = {
        'Svelte root target': b'pa-svelte-layout',
        'Svelte root missing-target guard': b'PORT_ACCESS_LAYOUT_TARGET_MISSING',
        'native engine workspace': b'existing-engine-control',
        'engine preview': b'engine-wizard-preview',
        'Odin controls': b'pa-svelte-odin',
        'engineering controls': b'pa-svelte-engineering',
        'native status': b'pa-svelte-status',
        'native operation log': b'pa-log-state',
        'operation log API': b'/api/modules/port-access-manager/operation-logs',
    }
    if len(bundle) < 3000:
        raise ValueError('missing real compiled Svelte entrypoint')
    for label, token in required_bundle_markers.items():
        if token not in bundle:
            raise ValueError('compiled Svelte bundle missing ' + label)
    if b'PortAccessStatus.svelte' in bundle and b'<script>' in bundle:
        raise ValueError('uncompiled Svelte source shipped')
    # The normal build-port-access-manager-opkg.sh path always builds this
    # file immediately before packaging. When available, byte-compare it to
    # the shipped bundle to catch stale or substituted IPK payloads.
    build_bundle = (Path(__file__).resolve().parents[1]
                    / 'modules/port-access-manager/frontend/build/svelte-status.js')
    if build_bundle.is_file() and build_bundle.read_bytes() != bundle:
        raise ValueError('packaged Svelte bundle differs from compiled build output')
    print(f'P43_IPK_CONTENTS: PASS; FILES={len(files)}; SCRIPTS={len(scripts)}',flush=True)


def main():
    p = argparse.ArgumentParser()
    p.add_argument('package',type=Path)
    args = p.parse_args()
    try:
        verify(args.package)
    except (OSError, ValueError, tarfile.TarError, json.JSONDecodeError) as e:
        print('P43_IPK_CONTENTS: FAIL: '+str(e),flush=True)
        return 1
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
