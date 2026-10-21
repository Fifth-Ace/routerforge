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

class CollectorV2Tests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.path = Path(self.tmp.name)/"database.sqlite"
        self.db = collector.Store(self.path)
        self.now = dt.datetime(2026, 10, 21, 12, 0, tzinfo=dt.timezone.utc)
        self.id = "1"*32
        self.register = {"schema_version":2,"kind":"register","installation_id":self.id,
                         "event_id":"2"*32,"core_version":"0.13.0~beta.9","release_channel":"beta"}

    def event(self, kind, event_id, **updates):
        o = {"schema_version":2,"kind":kind,"installation_id":self.id,
             "event_id":event_id*32,"core_version":"0.13.0~beta.9","release_channel":"beta"}
        if kind=="diagnostic":
            o.update({"architecture":"arm64","modules":[{"id":"dns","version":"0.13.0~beta.9"},
                                                     {"id":"monitoring","version":"0.12.0"}]})
        o.update(updates)
        return o

    def test_channels_and_upgrade_do_not_add_installation(self):
        self.db.receive(self.register,self.now)
        self.db.receive(self.event("presence","3",core_version="0.14.0",release_channel="stable"),self.now+dt.timedelta(hours=24))
        stats = self.db.metrics(self.now+dt.timedelta(hours=24))
        self.assertEqual(stats["registered_total"],1)
        self.assertEqual(stats["channels"],[("stable",1)])
        self.assertEqual(stats["versions"],[("stable","0.14.0",1)])
        self.assertTrue(self.db.receive(self.register,self.now+dt.timedelta(hours=24)))
        self.assertEqual(self.db.metrics(self.now+dt.timedelta(hours=24))["registered_total"],1)

    def test_module_snapshot_replace_and_dedupe(self):
        self.db.receive(self.register,self.now)
        first = self.event("diagnostic","4")
        self.db.receive(first,self.now)
        self.db.receive(first,self.now)
        stats=self.db.metrics(self.now)
        self.assertEqual(stats["telemetry_7d"],1)
        self.assertEqual(stats["module_stats"],[("dns",1),("monitoring",1)])
        second=self.event("diagnostic","5",modules=[{"id":"admin","version":"0.14.0"}])
        self.db.receive(second,self.now+dt.timedelta(days=1))
        stats=self.db.metrics(self.now+dt.timedelta(days=1))
        self.assertEqual(stats["module_stats"],[("admin",1)])
        self.assertEqual(stats["module_versions"],[("admin","0.14.0",1)])
        self.assertEqual(stats["architectures"],[("arm64",1)])
        self.assertNotIn("cpu",collector.dashboard(stats).lower())
        self.assertIn("Популярность модулей",collector.dashboard(stats))

    def test_no_diagnostic_outside_window(self):
        self.db.receive(self.register,self.now)
        self.db.receive(self.event("diagnostic","4"),self.now)
        stats=self.db.metrics(self.now+dt.timedelta(days=8))
        self.assertEqual(stats["telemetry_7d"],0)
        self.assertEqual(stats["module_stats"],[])

    def test_strict_allowlist_and_privacy(self):
        bad = [
            self.event("diagnostic","4",modules=[{"id":"extra","version":"1.0.0"}]),
            self.event("diagnostic","4",modules=[{"id":"dns","version":"1.0.0"},{"id":"dns","version":"1.0.0"}]),
            self.event("diagnostic","4",temperature_band_c="60-69"),
            self.event("diagnostic","4",modules=None),
            self.event("diagnostic","4",modules=[{"id":"dns","version":"bad spaces"}]),
            self.event("presence","4",cpu_load_band="80-100"),
            self.event("presence","4",release_channel="nightly"),
        ]
        for item in bad:
            with self.subTest(item=item):
                with self.assertRaises(ValueError):collector.validate(item)

    def test_legacy_database_migration_preserves_registrations(self):
        import sqlite3
        self.db.receive({"schema_version":1,"kind":"register","installation_id":"f"*32,
                         "event_id":"e"*32,"core_version":"0.12.0"},self.now)
        # Re-open as if old installation had never been upgraded.
        conn = sqlite3.connect(self.path)
        try:
            conn.execute("UPDATE installations SET release_channel='unknown' WHERE installation_id=?",("f"*32,))
            conn.commit()
        finally:
            conn.close()
        reopened=collector.Store(self.path)
        stats=reopened.metrics(self.now)
        self.assertEqual(stats["registered_total"],1)
        self.assertEqual(stats["channels"],[("unknown",1)])
        # v2 heartbeat can upgrade channel/version, without double registration.
        reopened.receive({**self.event("register","6"),"installation_id":"f"*32},self.now)
        self.assertEqual(reopened.metrics(self.now)["registered_total"],1)

    def test_upgrade_schema_from_original_sqlite_tables(self):
        import sqlite3
        self.path.unlink()
        conn = sqlite3.connect(self.path)
        try:
            conn.execute("CREATE TABLE installations(installation_id TEXT PRIMARY KEY,first_seen TEXT NOT NULL,last_seen TEXT NOT NULL,first_core_version TEXT NOT NULL)")
            conn.execute("INSERT INTO installations VALUES(?,?,?,?)",("f"*32,self.now.isoformat(),self.now.isoformat(),"0.12.0"))
            conn.commit()
        finally:
            conn.close()
        migrated=collector.Store(self.path)
        self.assertEqual(migrated.metrics(self.now)["registered_total"],1)
        with migrated.connect() as db:
            row=db.execute("SELECT current_core_version,release_channel FROM installations").fetchone()
        self.assertEqual(row,("0.12.0","unknown"))
