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

    def test_resume_committed_push_exact_parent(self):
        info={'phase':'committed','sha':'b'*40,'run_id':None}
        with tempfile.TemporaryDirectory() as t:
            root=Path(t);(root/'.git').mkdir()
            d.write_state(root,info)
            with patch.object(d,'precheck'), patch.object(d,'git',side_effect=['b'*40,'a'*40]), \
                 patch.object(d,'remote_sha',side_effect=['a'*40,'b'*40]), \
                 patch.object(d,'changed_paths',return_value=[]), \
                 patch.object(d,'run',return_value=0) as run, \
                 patch.object(d,'watch') as watch:
                d.resume(root)
                run.assert_called_once_with(['git','-C',str(root),'push','origin','HEAD:refs/heads/dev'],capture=False)
                watch.assert_called_once()
                self.assertEqual(watch.call_args.args[1]['phase'],'pushed')

    def test_resume_rejects_diverged_remote_without_push(self):
        with tempfile.TemporaryDirectory() as t:
            root=Path(t);(root/'.git').mkdir()
            d.write_state(root,{'phase':'committed','sha':'b'*40,'run_id':None})
            with patch.object(d,'precheck'),patch.object(d,'git',side_effect=['b'*40,'a'*40]), \
                 patch.object(d,'remote_sha',return_value='c'*40), \
                 patch.object(d,'run') as run:
                with self.assertRaisesRegex(d.DeliveryError,'REMOTE_DEV_MOVED'):
                    d.resume(root)
                run.assert_not_called()

    def test_resume_rejects_dirty_tree_before_push(self):
        with tempfile.TemporaryDirectory() as t:
            root=Path(t);(root/'.git').mkdir()
            d.write_state(root,{'phase':'committed','sha':'b'*40,'run_id':None})
            with patch.object(d,'precheck'),patch.object(d,'git',side_effect=['b'*40,'a'*40]), \
                 patch.object(d,'remote_sha',return_value='a'*40), \
                 patch.object(d,'changed_paths',return_value=['other.txt']), \
                 patch.object(d,'run') as run:
                with self.assertRaisesRegex(d.DeliveryError,'DIRTY_WORKTREE'):
                    d.resume(root)
                run.assert_not_called()

    def test_resume_push_network_error_does_not_duplicate_commit(self):
        with tempfile.TemporaryDirectory() as t:
            root=Path(t);(root/'.git').mkdir()
            d.write_state(root,{'phase':'committed','sha':'b'*40,'run_id':None})
            with patch.object(d,'precheck'),patch.object(d,'git',side_effect=['b'*40,'a'*40]),                  patch.object(d,'remote_sha',return_value='a'*40),                  patch.object(d,'changed_paths',return_value=[]),                  patch.object(d,'run',side_effect=d.DeliveryError('SIMULATED_NETWORK_DROP')),                  patch.object(d,'watch') as watch:
                with self.assertRaisesRegex(d.DeliveryError,'SIMULATED_NETWORK_DROP'):
                    d.resume(root)
                watch.assert_not_called()
            self.assertEqual(__import__('json').loads(d.state_file(root).read_text())['phase'],'committed')

    def test_resume_after_push_only_watches(self):
        with tempfile.TemporaryDirectory() as t:
            root=Path(t);(root/'.git').mkdir()
            d.write_state(root,{'phase':'pushed','sha':'b'*40,'run_id':'12345'})
            with patch.object(d,'precheck'),patch.object(d,'git',return_value='b'*40), \
                 patch.object(d,'remote_sha',return_value='b'*40), \
                 patch.object(d,'watch') as watch,patch.object(d,'run') as run:
                d.resume(root)
                watch.assert_called_once()
                run.assert_not_called()

    def test_resume_verified_is_idempotent(self):
        with tempfile.TemporaryDirectory() as t:
            root=Path(t);(root/'.git').mkdir()
            d.write_state(root,{'phase':'verified','sha':'b'*40,'run_id':'12345'})
            with patch.object(d,'precheck'),patch.object(d,'git',return_value='b'*40), \
                 patch.object(d,'remote_sha',return_value='b'*40), \
                 patch.object(d,'watch') as watch,patch.object(d,'run') as run:
                d.resume(root)
                watch.assert_not_called()
                run.assert_not_called()

    def test_watch_failed_ci_keeps_recoverable_state(self):
        with tempfile.TemporaryDirectory() as t:
            root=Path(t);(root/'.git').mkdir()
            info={'phase':'pushed','sha':'b'*40,'run_id':'12345'}
            with patch.object(d,'run',side_effect=d.DeliveryError('CI_FAILED')):
                with self.assertRaisesRegex(d.DeliveryError,'CI_FAILED'):
                    d.watch(root,info)
            saved=__import__('json').loads(d.state_file(root).read_text())
            self.assertEqual(saved['phase'],'pushed')
            self.assertEqual(saved['run_id'],'12345')

    def test_watch_validates_exact_sha(self):
        with tempfile.TemporaryDirectory() as t:
            root=Path(t);(root/'.git').mkdir()
            info={'phase':'pushed','sha':'b'*40,'run_id':'12345'}
            bad='{"headSha":"'+ 'c'*40 +'","event":"push","status":"completed","conclusion":"success"}'
            with patch.object(d,'run',side_effect=[0,bad]):
                with self.assertRaisesRegex(d.DeliveryError,'CI_EXACT_SHA'):
                    d.watch(root,info)
            self.assertEqual(__import__('json').loads(d.state_file(root).read_text())['phase'],'pushed')

    def test_find_run_retries_transient_empty(self):
        with patch.object(d,'run',side_effect=['','', '12345']),patch.object(d.time,'sleep',return_value=None) as wait:
            self.assertEqual(d.find_run('a'*40),'12345')
            self.assertEqual(wait.call_count,2)

if __name__=='__main__': unittest.main()
