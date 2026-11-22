package lite

import (
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"os"
	"path/filepath"
	"testing"
)

func TestIntentMigrationNeedsNoInferredGateConsent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	s := IntentStore{ConfigPath: p}
	fresh, e := s.LoadOrMigrate()
	if e != nil || fresh.ServiceEnabled || fresh.InterceptionEnabled {
		t.Fatalf("fresh install: %+v %v", fresh, e)
	}
	c := fixtureConfig(t)
	if e = atomicFile(p+".service-active.json", dr.ExportContent(c)); e != nil {
		t.Fatal(e)
	}
	state, e := s.LoadOrMigrate()
	if e != nil || !state.ServiceEnabled || state.InterceptionEnabled {
		t.Fatalf("ambiguous legacy gate consent: %+v %v", state, e)
	}
	if e = os.Remove(p + ".service-active.json"); e != nil {
		t.Fatal(e)
	}
	restored, e := s.Applied(state)
	if e != nil || restored.Service.Listen != c.Service.Listen {
		t.Fatalf("old prerm must not destroy new applied snapshot: %v", e)
	}
	state.ServiceEnabled = false
	state.InterceptionEnabled = false
	if e = s.Save(state); e != nil {
		t.Fatal(e)
	}
	after, e := s.LoadOrMigrate()
	if e != nil || after.ServiceEnabled {
		t.Fatal("OFF was lost")
	}
}
func TestIntentRejectsCorruptSymlinkAndUnverifiedON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	s := IntentStore{ConfigPath: p}
	for _, state := range []Intent{{Version: 1, ServiceEnabled: true}, {Version: 1, InterceptionEnabled: true}, {Version: 2}} {
		if s.Save(state) == nil {
			t.Fatal("invalid intent accepted")
		}
	}
	if e := os.WriteFile(s.Path(), []byte("{broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := s.LoadOrMigrate(); e == nil {
		t.Fatal("corruption silently treated as OFF/fresh")
	}
	os.Remove(s.Path())
	outside := filepath.Join(t.TempDir(), "outside")
	os.WriteFile(outside, []byte("unchanged"), 0600)
	os.Symlink(outside, s.Path())
	if s.Save(Intent{Version: 1}) == nil {
		t.Fatal("intent symlink overwritten")
	}
	b, _ := os.ReadFile(outside)
	if string(b) != "unchanged" {
		t.Fatal("foreign file modified")
	}
}
func TestIntentKeepsPriorCommitWhenNewSnapshotWriteFails(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	s := IntentStore{ConfigPath: p}
	c := fixtureConfig(t)
	tmp := dr.NewConfigStore(filepath.Join(t.TempDir(), "c"))
	_, rev, _ := tmp.Load()
	rev, e := tmp.Save(c, rev)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Snapshot(c, rev); e != nil {
		t.Fatal(e)
	}
	state := Intent{Version: 1, ServiceEnabled: true, AppliedRevision: rev}
	if e = s.Save(state); e != nil {
		t.Fatal(e)
	}
	bad := state
	bad.AppliedRevision = "sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	file, _ := s.SnapshotPath(bad.AppliedRevision)
	os.Symlink("/does-not-exist", file)
	if s.Snapshot(c, bad.AppliedRevision) == nil {
		t.Fatal("failed snapshot stage accepted")
	}
	got, e := s.Load()
	if e != nil || got.AppliedRevision != rev {
		t.Fatal("previous intent lost after failed stage")
	}
	if _, e = s.Applied(got); e != nil {
		t.Fatal(e)
	}
}

func TestP19InterruptedStateStagingKeepsCommittedIntent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := IntentStore{ConfigPath: path}
	if err := store.Save(Intent{Version: 1}); err != nil {
		t.Fatal(err)
	}
	// A crash before rename leaves a partial temp file beside the committed file.
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".dns-lite-interrupted"), []byte(`{"version":1,"service_enabled":true`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil || got.ServiceEnabled {
		t.Fatal("partial stage became authoritative", got, err)
	}
	if err = store.Save(Intent{Version: 1}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(store.Path())
	if err != nil || fi.Mode().Perm() != 0600 {
		t.Fatal("unsafe state permissions", err)
	}
}
