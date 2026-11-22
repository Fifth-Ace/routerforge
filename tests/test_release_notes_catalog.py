import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


class NotesCatalogTests(unittest.TestCase):
    def test_separates_releases_and_preserves_full_standup(self):
        p = Path(__file__).resolve().parents[1] / 'scripts' / 'build-release-notes-catalog.py'
        spec = importlib.util.spec_from_file_location('notes_catalog', p)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            docs, dist = base / 'docs', base / 'dist'
            docs.mkdir()
            original = '# RouterForge 0.14.0\n\nНикто не просил. Мы сочли необходимым.\n'
            (docs / 'RELEASE_NOTES_0.14.0.md').write_text(original, encoding='utf-8')
            (docs / 'RELEASE_SUMMARY_0.14.0.md').write_text('# Кратко\n\n- Исправления\n', encoding='utf-8')
            (docs / 'RELEASE_NOTES_0.15.0-beta.3.md').write_text('# Beta 3\n\n- DNS\n', encoding='utf-8')
            names, index = module.build('stable', docs, dist)
            data = json.loads((dist / index).read_text(encoding='utf-8'))
            self.assertEqual([r['version'] for r in data['releases']], ['0.14.0'])
            story = data['releases'][0]['story']['asset']
            self.assertEqual((dist / story).read_text(encoding='utf-8'), original)
            self.assertEqual(len(names), 2)
            names, index = module.build('beta', docs, dist)
            data = json.loads((dist / index).read_text(encoding='utf-8'))
            self.assertEqual([r['version'] for r in data['releases']], ['0.15.0-beta.3'])
            self.assertEqual(len(names), 1)

    def test_version_one_publishes_only_technical_summary(self):
        p = Path(__file__).resolve().parents[1] / 'scripts' / 'build-release-notes-catalog.py'
        spec = importlib.util.spec_from_file_location('notes_catalog_modern', p)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            docs, dist = base / 'docs', base / 'dist'
            docs.mkdir()
            (docs / 'RELEASE_NOTES_1.0.0.md').write_text('# Technical\n\nOfficial notes\n', encoding='utf-8')
            (docs / 'RELEASE_SUMMARY_1.0.0.md').write_text('# Summary\n\nTechnical only\n', encoding='utf-8')
            names, index = module.build('stable', docs, dist)
            releases = json.loads((dist / index).read_text(encoding='utf-8'))['releases']
            self.assertEqual([r['version'] for r in releases], ['1.0.0'])
            self.assertIn('summary', releases[0])
            self.assertNotIn('story', releases[0])
            self.assertEqual(len(names), 1)
            self.assertEqual((dist / names[0]).read_text(encoding='utf-8'), '# Summary\n\nTechnical only\n')

if __name__ == '__main__':
    unittest.main()
