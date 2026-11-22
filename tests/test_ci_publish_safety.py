"""CI-only P0.4B guard: source contract, not a simulation of GitHub execution."""
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / '.github/workflows/ci.yml'
GUARD = "&& !contains(github.event.head_commit.message, '[rf-ci-only]')"


class PublishSafetyContract(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow = WORKFLOW.read_text(encoding='utf-8')

    def test_no_publish_on_ci_only_commit_marker(self):
        snippet = self.workflow.split('\n  publish_dev:\n', 1)[1].split('\n  publish_stable:\n', 1)[0]
        self.assertIn(GUARD, snippet)
        self.assertEqual(snippet.count(GUARD), 1)
        self.assertIn("needs.test.result == 'success'", snippet)
        self.assertIn("github.ref == 'refs/heads/dev'", snippet)
        self.assertIn("github.event_name == 'push'", snippet)

    def test_stable_release_requires_explicit_full_dispatch_and_completed_test(self):
        section = self.workflow.split('\n  publish_stable:\n', 1)[1]
        self.assertIn("github.event_name == 'workflow_dispatch'", section)
        self.assertIn("github.ref == 'refs/heads/main'", section)
        self.assertIn("inputs.full_release == true", section)
        self.assertIn("inputs.publish_beta != true", section)
        self.assertIn("inputs.beta_validate_only != true", section)
        self.assertIn("needs.test.result == 'success'", section)
        self.assertIn("name: routerforge-stable-promotion", section)
        self.assertIn("uses: actions/download-artifact@v4", section)
        self.assertNotIn("event_name == 'push' && github.ref == 'refs/heads/main'", section)

    def test_full_validation_can_be_nonpublishing(self):
        self.assertIn('beta_validate_only:', self.workflow)
        self.assertIn('publish_beta:', self.workflow)
        self.assertIn('inputs.beta_validate_only != true', self.workflow)
        self.assertIn("github.event_name == 'workflow_dispatch' && inputs.full_release == true", self.workflow)

    def test_linux_staging_regressions_are_required(self):
        self.assertIn('P0.4 DNS staging isolation regression (Linux)', self.workflow)
        self.assertIn('test_dns_router_staging.py -v', self.workflow)
        self.assertIn('P0.4 CI-only release guard regression', self.workflow)


if __name__ == '__main__':
    unittest.main()
