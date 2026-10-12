import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

FILE = Path(__file__).resolve().parents[1] / 'scripts' / 'rf_delivery.py'
spec = importlib.util.spec_from_file_location('rf_delivery', FILE)
d = importlib.util.module_from_spec(spec)
spec.loader.exec_module(d)

class DeliveryTests(unittest.TestCase):
    def test_porcelain_preserves_leading_space(self):
        from unittest.mock import patch
        from types import SimpleNamespace
        sample = " M .github/workflows/ci.yml\n?? scripts/rf_delivery.py\n"
        with patch.object(
            d.subprocess,
            "run",
            return_value=SimpleNamespace(returncode=0, stdout=sample),
        ):
            paths = d.changed_paths(Path("."))
        self.assertEqual(
            paths,
            [".github/workflows/ci.yml", "scripts/rf_delivery.py"],
        )
    def test_strict_remote_sha(self):
        with patch.object(d,'run',return_value='a'*40+'\trefs/heads/dev'):
            self.assertEqual(d.remote_sha(),'a'*40)
        with patch.object(d,'run',return_value='unexpected'):
            with self.assertRaises(d.DeliveryError): d.remote_sha()
    def test_find_run_exact_scalar(self):
        with patch.object(d,'run',return_value='123456') as cmd:
            self.assertEqual(d.find_run('a'*40),'123456')
            self.assertIn('unique',cmd.call_args.args[0][-1])
        with patch.object(d,'run',return_value='123 456'),patch.object(d.time,'sleep',return_value=None):
            with self.assertRaises(d.DeliveryError):d.find_run('a'*40)
    def test_scope_gate(self):
        with patch.object(d,'git',side_effect=['a'*40,' M scripts/ok.py\n?? secrets.txt']),patch.object(d,'remote_sha',return_value='a'*40):
            with self.assertRaises(d.DeliveryError):d.gates(Path('.'),['scripts/ok.py'])
    def test_state_roundtrip(self):
        with tempfile.TemporaryDirectory() as t:
            root=Path(t);(root/'.git').mkdir()
            d.write_state(root,{'phase':'pushed','sha':'a'*40})
            self.assertEqual(__import__('json').loads(d.state_file(root).read_text())['phase'],'pushed')
if __name__=='__main__': unittest.main()
