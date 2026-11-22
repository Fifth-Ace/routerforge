#!/usr/bin/env python3
"""Negative regression tests for the read-only ALL11 source-inventory gate."""
from pathlib import Path
import importlib.util
import shutil
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / 'scripts/verify-full-svelte-migration.py'
spec = importlib.util.spec_from_file_location('svelte_inventory_gate', SCRIPT)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

FILES = ('index.html','package.json','main.js','NFQWSManagerApp.svelte','NfqwsRuntime.js',
         'NfqwsListEditorModel.js','NfqwsSelector.svelte','NfqwsAutoSetup.svelte')

class NativeInventoryTests(unittest.TestCase):
    def test_current_repository_has_all_eleven_source_entries(self):
        for name in module.FRONTENDS:
            self.assertEqual(module.inspect(name, ROOT), [], name)

    def with_modified_frontend(self, file, before, after):
        with tempfile.TemporaryDirectory() as temporary:
            frontend = Path(temporary)/'modules/nfqws-manager/frontend'
            frontend.mkdir(parents=True)
            actual = ROOT/'modules/nfqws-manager/frontend'
            for name in FILES:
                shutil.copyfile(actual/name, frontend/name)
            target = frontend/file
            contents = target.read_text(encoding='utf-8')
            self.assertIn(before, contents)
            target.write_text(contents.replace(before, after), encoding='utf-8')
            return module.inspect('nfqws-manager', Path(temporary))

    def test_missing_svelte_entrypoint_is_rejected(self):
        issues = self.with_modified_frontend('NFQWSManagerApp.svelte',
                    'startNFQWSRuntime({', 'startNFQWSRuntime(\'legacy\', {')
        self.assertIn('NFQWS_NATIVE_NAV_MISSING', issues)

    def test_missing_exact_list_sha_is_rejected(self):
        issues = self.with_modified_frontend('NfqwsListEditorModel.js',
                    'expected_sha256:reference.sha256', 'expected_sha256:null')
        self.assertIn('NFQWS_CAS_OR_POSTVERIFY_CONTRACT_MISSING', issues)

    def test_missing_verified_apply_is_rejected(self):
        issues = self.with_modified_frontend('NfqwsSelector.svelte',
                    'post_apply_verified', 'removed_verification')
        self.assertIn('NFQWS_CAS_OR_POSTVERIFY_CONTRACT_MISSING', issues)

if __name__ == '__main__':
    unittest.main(verbosity=2)
