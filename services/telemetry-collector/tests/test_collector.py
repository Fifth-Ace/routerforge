import datetime as dt
import importlib.util
import tempfile
import unittest
from pathlib import Path

loader=importlib.util.spec_from_file_location('collector',str(Path(__file__).parents[1] / 'collector.py'))
collector=importlib.util.module_from_spec(loader)
loader.loader.exec_module(collector)

class CollectorTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.db = collector.Store(Path(self.tmp.name)/'s.sqlite')
        self.now=dt.datetime(2026,10,21,12,0,tzinfo=dt.timezone.utc)
        self.register={'schema_version':1,'kind':'register','installation_id':'a'*32,'event_id':'b'*32,'core_version':'0.13.0~beta.9'}
    def test_registration_idempotent(self):
        self.assertTrue(self.db.receive(self.register,self.now))
        self.assertTrue(self.db.receive(self.register,self.now))
        other={**self.register,'event_id':'c'*32}
        self.assertTrue(self.db.receive(other,self.now))
        self.assertEqual(self.db.metrics(self.now)['registered_total'],1)
    def test_presence_requires_register(self):
        event={'schema_version':1,'kind':'presence','installation_id':'a'*32,'event_id':'d'*32}
        self.assertFalse(self.db.receive(event,self.now))
        self.db.receive(self.register,self.now)
        self.assertTrue(self.db.receive(event,self.now))
        self.assertTrue(self.db.receive(event,self.now))
    def test_diagnostic_aggregates_once(self):
        self.db.receive(self.register,self.now)
        diagnostic={'schema_version':1,'kind':'diagnostic','installation_id':'a'*32,'event_id':'d'*32,'core_version':'0.13.0~beta.9','architecture':'mipsel','cpu_load_band':'20-39','ram_load_band':'40-59','temperature_band_c':'50-59','health_code':'ok'}
        self.assertTrue(self.db.receive(diagnostic,self.now))
        self.assertTrue(self.db.receive(diagnostic,self.now))
        self.assertEqual(self.db.metrics(self.now)['architecture_reports'],[('mipsel',1)])
    def test_rejects_extra_field_and_invalid_identifiers(self):
        for candidate in ({**self.register,'hostname':'home'}, {**self.register,'installation_id':'not-random'}, {**self.register,'schema_version':True}, {**self.register,'core_version':'x'*100}):
            with self.assertRaises(ValueError):self.db.receive(candidate,self.now)
    def test_age_metrics(self):
        self.db.receive(self.register,self.now-dt.timedelta(days=31))
        m=self.db.metrics(self.now)
        self.assertEqual((m['registered_total'],m['active_7d'],m['active_30d']),(1,0,0))
        event={'schema_version':1,'kind':'presence','installation_id':'a'*32,'event_id':'d'*32}
        self.db.receive(event,self.now)
        self.assertEqual(self.db.metrics(self.now)['active_7d'],1)

if __name__=='__main__':unittest.main()
