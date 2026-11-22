"""Release train regression checks; synthetic inputs, no GitHub I/O."""
import importlib.util
import json
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('stable_train', Path(__file__).resolve().parents[1] / 'scripts' / 'verify-stable-train.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class StableTrainP51Tests(unittest.TestCase):
    def cfg(self):
        return json.loads((Path(__file__).resolve().parents[1] / 'release' / 'channels' / 'stable.json').read_text(encoding='utf-8'))

    def test_current_floor_and_eleven_component_policy(self):
        self.assertGreaterEqual(module.check_config(self.cfg()), (0, 14, 0))
        self.assertIn('dns-router', {x['id'] for x in self.cfg()['components']})

    def test_first_stable_all_eleven_package_versions_exact(self):
        cfg = self.cfg()
        self.assertEqual(cfg['release_version'], '1.0.0')
        self.assertEqual(len(cfg['components']), 11)
        self.assertTrue(all(x['version'] == '1.0.0' for x in cfg['components']))
        for i in range(1, 11):
            wrong = self.cfg()
            wrong['components'][i]['version'] = '0.99.9'
            with self.subTest(component=wrong['components'][i]['id']):
                with self.assertRaisesRegex(ValueError, 'ALL11_VERSION_REQUIRED'):
                    module.check_config(wrong)

    def test_decline_regression(self):
        d = self.cfg()
        d['release_version'] = '0.13.0'
        d['components'][0]['version'] = '0.13.0'
        with self.assertRaisesRegex(ValueError, 'REGRESSION'):
            module.check_config(d)

    def test_no_duplicate_or_missing_component(self):
        d = self.cfg()
        d['components'] = d['components'][:-1]
        with self.assertRaisesRegex(ValueError, 'ELEVEN_COMPONENT'):
            module.check_config(d)

    def test_reject_republishing_same_version(self):
        with self.assertRaisesRegex(ValueError, 'NOT_NEWER'):
            module.check_config(self.cfg(), candidate=True, tags=['routerforge-v0.14.0', 'routerforge-v1.0.0', 'routerforge-v0.15.0-beta.5'])

    def test_approve_newer_version_only(self):
        d = self.cfg()
        d['release_version'] = '1.0.1'
        d['components'][0]['version'] = '1.0.1'
        self.assertEqual(module.check_config(d, candidate=True, tags=['routerforge-v0.14.0']), (1, 0, 1))

    def test_missing_release_tag_evidence_is_not_permission_to_publish(self):
        with self.assertRaisesRegex(ValueError, 'FLOOR_MISSING'):
            module.check_config(self.cfg(), candidate=True, tags=[])

if __name__ == '__main__':
    unittest.main()
