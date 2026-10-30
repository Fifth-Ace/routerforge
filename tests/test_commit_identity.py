import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'scripts' / 'verify-commit-identity.py'
EMAIL = '248300012+Fifth-Ace@users.noreply.github.com'


class IdentityGateTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='rf-identity-')
        self.addCleanup(self.tmp.cleanup)
        self.repo = Path(self.tmp.name)
        self.git('init', '-q')
        self.git('config', 'user.name', 'Fifth-Ace')
        self.git('config', 'user.email', EMAIL)
        self.serial = 0
        self.base = self.commit('Fifth-Ace', EMAIL)

    def git(self, *args, env=None):
        return subprocess.run(['git', '-C', str(self.repo), *args],
                              text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                              encoding='utf-8', check=True, env=env).stdout.strip()

    def commit(self, name, email, committer_name=None, committer_email=None):
        self.serial += 1
        (self.repo / 'file.txt').write_text(str(self.serial), encoding='utf-8')
        self.git('add', 'file.txt')
        env = dict(os.environ)
        env.update(GIT_AUTHOR_NAME=name, GIT_AUTHOR_EMAIL=email,
                   GIT_COMMITTER_NAME=committer_name or name,
                   GIT_COMMITTER_EMAIL=committer_email or email)
        self.git('commit', '-q', '-m', 'fixture', env=env)
        return self.git('rev-parse', 'HEAD')

    def gate(self, *args):
        return subprocess.run([sys.executable, str(SCRIPT), *args], cwd=self.repo,
                              text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def test_allowed_head(self):
        self.assertEqual(self.gate('head', self.base).returncode, 0)

    def test_bad_author_name(self):
        bad = self.commit('RouterForge Delivery', EMAIL)
        self.assertIn('COMMIT_IDENTITY_DENIED', self.gate('head', bad).stderr)

    def test_bad_author_email(self):
        bad = self.commit('Fifth-Ace', 'routerforge-dev@users.noreply.github.com')
        self.assertIn('COMMIT_IDENTITY_DENIED', self.gate('head', bad).stderr)

    def test_bad_committer(self):
        bad = self.commit('Fifth-Ace', EMAIL, 'RouterForge CI', EMAIL)
        self.assertIn('COMMIT_IDENTITY_DENIED', self.gate('head', bad).stderr)

    def test_coauthor_trailer_denied(self):
        self.serial += 1
        (self.repo / 'file.txt').write_text(str(self.serial), encoding='utf-8')
        self.git('add', 'file.txt')
        self.git('commit', '-m', 'fixture', '-m', 'Co-authored-by: RouterForge Bot <bot@example.com>')
        sha = self.git('rev-parse', 'HEAD')
        self.assertIn('COAUTHOR_TRAILER_DENIED', self.gate('head', sha).stderr)

    def test_clean_range(self):
        good = self.commit('Fifth-Ace', EMAIL)
        self.assertEqual(self.gate('range', self.base, good).returncode, 0)

    def test_mixed_history_denied(self):
        self.commit('RouterForge Delivery', 'routerforge-delivery@users.noreply.github.com')
        head = self.commit('Fifth-Ace', EMAIL)
        self.assertIn('COMMIT_IDENTITY_DENIED', self.gate('range', self.base, head).stderr)

    def test_unrelated_or_reverse_range_denied(self):
        good = self.commit('Fifth-Ace', EMAIL)
        self.assertIn('PROMOTION_BASE_NOT_ANCESTOR', self.gate('range', good, self.base).stderr)

    def test_empty_range_denied(self):
        self.assertIn('EMPTY_PROMOTION_RANGE', self.gate('range', self.base, self.base).stderr)

    def test_nonexistent_sha_denied(self):
        self.assertNotEqual(self.gate('head', '0' * 40).returncode, 0)


if __name__ == '__main__':
    unittest.main()
