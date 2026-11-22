"""One-time 1.0.0 complete republish must not keep an old Core binary."""
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / 'scripts' / 'merge_release_index.py'
IDS = ('routerforge-core', 'dns', 'admin', 'monitoring', 'network-tools',
       'nfqws-manager', 'antiscan-manager', 'port-access-manager',
       'crowdsec-manager', 'dns-router', 'profiling')


class StableMergeAll11Tests(unittest.TestCase):
    def run_merge(self, version):
        with tempfile.TemporaryDirectory() as temp:
            folder = Path(temp)
            def entry(cid, hashchar):
                return {'id': cid, 'package': 'routerforge-' + cid,
                        'version': version, 'asset': 'routerforge-' + cid + '_' + version + '_aarch64-3.10.ipk',
                        'sha256': hashchar * 64, 'url': 'https://example.test/' + cid}
            before = {'schema_version': 1, 'channel': 'stable', 'target': 'aarch64-3.10',
                      'components': [entry(x, 'a') for x in IDS]}
            after = {'schema_version': 1, 'channel': 'stable', 'target': 'aarch64-3.10',
                     'components': [entry(x, 'b') for x in IDS]}
            for name, doc in [('before', before), ('after', after)]:
                (folder / (name + '.json')).write_text(json.dumps(doc), encoding='utf-8')
            args = [sys.executable, str(SCRIPT), '--candidate', str(folder / 'after.json'),
                    '--previous', str(folder / 'before.json'), '--output', str(folder / 'out.json'),
                    '--changed-list', str(folder / 'changed.txt'), '--checksums', str(folder / 'sums.txt')]
            subprocess.run(args, check=True, stdout=subprocess.PIPE, text=True)
            output = json.loads((folder / 'out.json').read_text(encoding='utf-8'))
            changed = (folder / 'changed.txt').read_text(encoding='utf-8').splitlines()
            return output['components'], changed

    def test_all11_first_stable_reuploads_core_and_all_modules(self):
        rows, changed = self.run_merge('1.0.0')
        self.assertEqual(len(changed), 11)
        self.assertTrue(all(row['sha256'] == 'b' * 64 for row in rows))
        self.assertTrue(any('routerforge-core_1.0.0_' in name for name in changed))

    def test_regular_stable_preserves_unchanged_version(self):
        rows, changed = self.run_merge('1.0.1')
        self.assertEqual(changed, [])
        self.assertTrue(all(row['sha256'] == 'a' * 64 for row in rows))


if __name__ == '__main__':
    unittest.main()
