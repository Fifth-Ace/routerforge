"""P0.1 regression tests: frontends must never silently bypass their CI gates."""
from __future__ import annotations

import importlib.util
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SOURCE = Path(__file__).resolve().parents[1] / 'scripts' / 'ci-frontend-scope.py'
SPEC = importlib.util.spec_from_file_location('ci_frontend_scope', SOURCE)
assert SPEC and SPEC.loader
module = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(module)


class FrontendScopeTests(unittest.TestCase):
    def classify(self, *paths, full=False):
        return module.classify(list(paths), full_release=full)

    def test_lite_svelte_and_ui_tests(self):
        for path in ('modules/dns-router-lite/frontend/DnsP25Settings.svelte',
                     'modules/dns-router-lite/frontend-tests/ui.test.cjs',
                     'modules/dns-router-lite/frontend/module.css'):
            self.assertEqual(self.classify(path), dict.fromkeys(module.FLAGS, False) |
                             {'dns_lite_frontend': True})

    def test_runtime_only(self):
        x = self.classify('modules/dns-router-runtime/frontend/DnsRuntimeApp.svelte')
        self.assertEqual(x, dict.fromkeys(module.FLAGS, False) | {'dns_runtime_frontend': True})

    def test_nfqws_only(self):
        x = self.classify('modules/nfqws-manager/frontend/index.html')
        self.assertEqual(x, dict.fromkeys(module.FLAGS, False) | {'nfqws_frontend': True})

    def test_shared_dns_builder_triggers_both_channels(self):
        x = self.classify('scripts/build-dns-router-experimental.sh')
        self.assertTrue(x['dns_lite_frontend'] and x['dns_runtime_frontend'])
        self.assertFalse(x['nfqws_frontend'])

    def test_shared_packager_triggers_nfqws(self):
        self.assertTrue(self.classify('scripts/build-module-opkg.sh')['nfqws_frontend'])

    def test_multi_module_change_is_union(self):
        self.assertEqual(self.classify(
            'modules/dns-router-runtime/frontend/DnsRuntimeApp.svelte',
            'modules/nfqws-manager/frontend/index.html',
        ), {'dns_lite_frontend': False, 'dns_runtime_frontend': True, 'nfqws_frontend': True})

    def test_unrelated_go_and_docs_remain_unselected(self):
        self.assertFalse(any(self.classify('modules/monitoring-runtime/app_server.go').values()))
        self.assertFalse(any(self.classify('README.md').values()))

    def test_full_release_forces_all_checks(self):
        self.assertTrue(all(self.classify(full=True).values()))

    def test_workflow_changes_trigger_all_module_gates(self):
        self.assertTrue(all(self.classify('.github/workflows/ci.yml').values()))

    def test_rejects_non_repository_paths(self):
        for path in ('../secret', '/tmp/a', 'modules\\x'):
            with self.assertRaises(ValueError):
                self.classify(path)

    def test_cli_fails_closed_if_diff_missing(self):
        with tempfile.TemporaryDirectory() as d:
            output = Path(d) / 'output'
            run = subprocess.run([
                sys.executable, str(SOURCE), '--changed-files', str(Path(d) / 'missing'),
                '--github-output', str(output), '--full-release', 'false',
            ], capture_output=True, text=True)
            self.assertEqual(run.returncode, 2)
            self.assertFalse(output.exists())

    def test_cli_reports_exact_scope_to_actions_output(self):
        with tempfile.TemporaryDirectory() as d:
            changed, output = Path(d) / 'changed', Path(d) / 'output'
            changed.write_text('modules/dns-router-lite/frontend/DnsLiteCards.svelte\n')
            output.write_text('preexisting=value\n')
            run = subprocess.run([
                sys.executable, str(SOURCE), '--changed-files', str(changed),
                '--github-output', str(output), '--full-release', 'false',
            ], capture_output=True, text=True)
            self.assertEqual(run.returncode, 0, run.stderr)
            self.assertEqual(output.read_text().splitlines(), [
                'preexisting=value', 'dns_lite_frontend=true',
                'dns_runtime_frontend=false', 'nfqws_frontend=false',
            ])


if __name__ == '__main__':
    unittest.main()
