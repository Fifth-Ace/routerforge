"""RouterForge fail-closed hook self-tests. Never connect to network or modify the main checkout."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
OWNER = 'Fifth-Ace'
EMAIL = '248300012+Fifth-Ace@users.noreply.github.com'

class HookGateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='routerforge-identity-hooks-')
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        self.run_git('init', '-q')
        self.run_git('config', 'user.name', OWNER)
        self.run_git('config', 'user.email', EMAIL)
        hooks = self.repo / '.githooks'
        hooks.mkdir()
        for hook in ['pre-commit', 'pre-push']:
            src = ROOT / '.githooks' / hook
            dst = hooks / hook
            shutil.copy2(src, dst)
            dst.chmod(0o755)
        self.run_git('config', 'core.hooksPath', '.githooks')
        self.base = self.run_git('commit', '--allow-empty', '-m', 'owner fixture').stdout.strip()
        self.base = self.run_git('rev-parse', 'HEAD').stdout.strip()

    def run_git(self, *args, env=None, check=True):
        p = subprocess.run(['git', '-C', str(self.repo), *args], capture_output=True,
                           text=True, env=env, encoding='utf-8', errors='replace')
        if check and p.returncode:
            self.fail(f'git {args} failed: {p.stderr} {p.stdout}')
        return p

    def test_owner_commit_passes(self):
        self.run_git('commit', '--allow-empty', '-m', 'next owner commit')
        self.assertNotEqual(self.run_git('rev-parse', 'HEAD').stdout.strip(), self.base)

    def test_wrong_author_name_blocked_at_commit(self):
        env = dict(os.environ, GIT_AUTHOR_NAME='routerforge-dns-router-linux-arm64', GIT_AUTHOR_EMAIL=EMAIL)
        p = self.run_git('commit', '--allow-empty', '-m', 'wrong author', env=env, check=False)
        self.assertNotEqual(p.returncode, 0)
        self.assertIn('DENIED_AUTHOR', p.stderr)
        self.assertEqual(self.run_git('rev-parse', 'HEAD').stdout.strip(), self.base)

    def test_wrong_committer_blocked_at_commit(self):
        env = dict(os.environ, GIT_COMMITTER_NAME='RouterForge Delivery', GIT_COMMITTER_EMAIL=EMAIL)
        p = self.run_git('commit', '--allow-empty', '-m', 'wrong committer', env=env, check=False)
        self.assertNotEqual(p.returncode, 0)
        self.assertIn('DENIED_COMMITTER', p.stderr)

    def test_wrong_email_blocked_at_commit(self):
        env = dict(os.environ, GIT_AUTHOR_NAME=OWNER, GIT_AUTHOR_EMAIL='routerforge-ci@users.noreply.github.com')
        p = self.run_git('commit', '--allow-empty', '-m', 'wrong email', env=env, check=False)
        self.assertNotEqual(p.returncode, 0)
        self.assertIn('DENIED_AUTHOR', p.stderr)

    def run_prepush(self, local_ref, local_sha, remote_ref, remote_sha):
        line = f'{local_ref} {local_sha} {remote_ref} {remote_sha}\n'
        return subprocess.run(['sh', str(self.repo / '.githooks' / 'pre-push'), 'origin', 'local-fixture'],
                              input=line, text=True, encoding='utf-8', capture_output=True, cwd=self.repo)

    def test_owner_outgoing_pass(self):
        self.run_git('commit', '--allow-empty', '-m', 'second owner')
        next_sha = self.run_git('rev-parse', 'HEAD').stdout.strip()
        p = self.run_prepush('HEAD', next_sha, 'refs/heads/dev', self.base)
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertIn('RF_IDENTITY_PRE_PUSH: PASS', p.stdout)

    def test_commit_hook_bypass_caught_at_prepush(self):
        self.run_git('-c', 'core.hooksPath=/dev/null', '-c', 'user.name=RouterForge Delivery',
                     'commit', '--allow-empty', '-m', 'bad bypassed')
        bad_sha = self.run_git('rev-parse', 'HEAD').stdout.strip()
        p = self.run_prepush('HEAD', bad_sha, 'refs/heads/dev', self.base)
        self.assertNotEqual(p.returncode, 0)
        self.assertIn('BAD_AUTHOR_OR_COMMITTER', p.stderr)

    def test_main_push_blocked_even_for_owner(self):
        self.run_git('commit', '--allow-empty', '-m', 'owner commit')
        sha = self.run_git('rev-parse', 'HEAD').stdout.strip()
        p = self.run_prepush('HEAD', sha, 'refs/heads/main', self.base)
        self.assertNotEqual(p.returncode, 0)
        self.assertIn('MAIN_PUSH_FORBIDDEN', p.stderr)

    def test_new_ref_and_nonfastforward_blocked(self):
        self.run_git('commit', '--allow-empty', '-m', 'owner commit')
        sha = self.run_git('rev-parse', 'HEAD').stdout.strip()
        new = self.run_prepush('HEAD', sha, 'refs/heads/dev', '0'*40)
        self.assertNotEqual(new.returncode, 0)
        self.assertIn('NEW_REF_FORBIDDEN', new.stderr)
        back = self.run_prepush('HEAD', self.base, 'refs/heads/dev', sha)
        self.assertNotEqual(back.returncode, 0)
        self.assertIn('NON_FAST_FORWARD_FORBIDDEN', back.stderr)

    def test_mailmap_normalizes_technical_aliases(self):
        shutil.copy2(ROOT / '.mailmap', self.repo / '.mailmap')
        cases = [
            'RouterForge Delivery <routerforge-delivery@users.noreply.github.com>',
            'routerforge-dns-router-linux-arm64 <248300012+Fifth-Ace@users.noreply.github.com>',
            'FifthAce <248300012+Fifth-Ace@users.noreply.github.com>',
            'RouterForge FIX1 <routerforge-dev@users.noreply.github.com>',
        ]
        for case in cases:
            actual = self.run_git('check-mailmap', case).stdout.strip()
            self.assertEqual(actual, f'{OWNER} <{EMAIL}>')

if __name__ == '__main__':
    unittest.main()
