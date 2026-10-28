package dnsrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func bootFixtureO3(t *testing.T) (*StartupManagerO3, *DurableManagerO2, *fixtureO2) {
	t.Helper()
	m, f := newDurableO2(t)
	s := &StartupManagerO3{Durable: m, Ready: m.Ready, NoCompetingGuard: func() error { return nil }}
	return s, m, f
}
func activateO3(t *testing.T, m *DurableManagerO2) {
	t.Helper()
	rev := revisionO2(t, m)
	p, e := m.Prepare(context.Background(), rev)
	if e != nil || !p.Ready {
		t.Fatalf("prepare: %+v %v", p, e)
	}
	result, e := m.Apply(context.Background(), rev, p.Token)
	if e != nil || result.State != "active" {
		t.Fatalf("apply: %+v %v", result, e)
	}
}
func TestDPR2O3NoLedgerDoesNotAdopt(t *testing.T) {
	s, _, f := bootFixtureO3(t)
	f.zones["alpha.dpr2n.test"] = true // foreign/unrecorded, must not adopt or delete
	out, e := s.Recover(context.Background())
	if e != nil || out.State != "unmanaged" || out.Changed || !f.zones["alpha.dpr2n.test"] {
		t.Fatalf("unmanaged: %+v %v", out, e)
	}
}
func TestDPR2O3RebootRestoresOwnedZones(t *testing.T) {
	s, m, f := bootFixtureO3(t)
	activateO3(t, m)
	f.zones = map[string]bool{} // simulate Keenetic runtime-only DNS entries vanished on reboot
	preview, e := s.Inspect(context.Background())
	if e != nil || preview.State != "repairable_missing_owned_zones" || len(preview.Missing) != 2 || !preview.ReadOnly {
		t.Fatalf("preview: %+v %v", preview, e)
	}
	out, e := s.Recover(context.Background())
	if e != nil || out.State != "recovered_active" || len(out.Restored) != 2 || !out.Changed || !out.WatchdogArmed {
		t.Fatalf("recover: %+v %v", out, e)
	}
	if !f.zones["alpha.dpr2n.test"] || !f.zones["beta.dpr2n.test"] {
		t.Fatal("zones not restored")
	}
	ledger, _, e := m.Ledger.Load()
	if e != nil || ledger == nil || ledger.State != "active" {
		t.Fatalf("ownership lost %+v %v", ledger, e)
	}
	out, e = s.Recover(context.Background())
	if e != nil || out.State != "already_active_verified" || out.Changed {
		t.Fatalf("idempotence %+v %v", out, e)
	}
}
func TestDPR2O3PartialRebootRestoresOnlyMissing(t *testing.T) {
	s, m, f := bootFixtureO3(t)
	activateO3(t, m)
	delete(f.zones, "alpha.dpr2n.test")
	out, e := s.Recover(context.Background())
	if e != nil || len(out.Restored) != 1 || out.Restored[0] != "alpha.dpr2n.test" || !f.zones["alpha.dpr2n.test"] {
		t.Fatalf("partial: %+v %v", out, e)
	}
}
func TestDPR2O3RefusesForeignOwnerAndPlanDrift(t *testing.T) {
	s, m, f := bootFixtureO3(t)
	activateO3(t, m)
	f.foreign = true
	out, e := s.Recover(context.Background())
	if e == nil || out.State != "blocked_foreign_drift" || !out.RecoveryRequired {
		t.Fatalf("foreign accepted %+v %v", out, e)
	}
	f.foreign = false
	cfg, rev, e := m.Store.Load()
	if e != nil {
		t.Fatal(e)
	}
	cfg.Upstreams[0].Address = "192.0.2.123:55354"
	if _, e = m.Store.Save(cfg, rev); e != nil {
		t.Fatal(e)
	}
	out, e = s.Recover(context.Background())
	if e == nil || out.State != "stale_policy" || !out.RecoveryRequired {
		t.Fatalf("stale accepted %+v %v", out, e)
	}
}
func TestDPR2O3PendingLedgerBlocked(t *testing.T) {
	s, m, f := bootFixtureO3(t)
	f.failAdd = "alpha.dpr2n.test"
	f.failRemove = "beta.dpr2n.test"
	rev := revisionO2(t, m)
	p, _ := m.Prepare(context.Background(), rev)
	_, _ = m.Apply(context.Background(), rev, p.Token)
	out, e := s.Recover(context.Background())
	if e == nil || out.State != "pending_recovery_required" || !out.RecoveryRequired {
		t.Fatalf("pending accepted %+v %v", out, e)
	}
}
func TestDPR2O3WatchdogGateAndMissingListener(t *testing.T) {
	s, m, f := bootFixtureO3(t)
	activateO3(t, m)
	f.zones = map[string]bool{}
	s.NoCompetingGuard = func() error { return ErrStale }
	out, e := s.Recover(context.Background())
	if e == nil || out.Changed || len(f.zones) != 0 {
		t.Fatalf("guard failed %+v %v", out, e)
	}
	s.NoCompetingGuard = func() error { return nil }
	s.Ready = func(string, string) bool { return false }
	out, e = s.Recover(context.Background())
	if e == nil || out.Changed || len(f.zones) != 0 {
		t.Fatalf("listener gate failed %+v %v", out, e)
	}
}
func TestDPR2O3RegistrationFailureRollsBackAndPreservesLedger(t *testing.T) {
	s, m, f := bootFixtureO3(t)
	activateO3(t, m)
	f.zones = map[string]bool{}
	f.failAdd = "alpha.dpr2n.test"
	out, e := s.Recover(context.Background())
	if e == nil || out.State != "rolled_back_after_failure" || len(f.zones) != 0 {
		t.Fatalf("rollback %+v %v", out, e)
	}
	ledger, _, e := m.Ledger.Load()
	if e != nil || ledger == nil || ledger.State != "active" {
		t.Fatalf("ledger %+v %v", ledger, e)
	}
}
func TestDPR2O3StatusRouteDisabledByDefault(t *testing.T) {
	m, _ := newDurableO2(t)
	api := NewAPI(m.Store, "")
	r := httptest.NewRequest(http.MethodGet, "/v1/keenetic/startup/status", nil)
	w := httptest.NewRecorder()
	api.Handler().ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatalf("O3 unexpectedly enabled: %d", w.Code)
	}
}
func TestDPR2O3StatusRouteReadOnly(t *testing.T) {
	s, m, _ := bootFixtureO3(t)
	activateO3(t, m)
	api := NewAPI(m.Store, "")
	api.StartupO3 = s
	r := httptest.NewRequest(http.MethodGet, "/v1/keenetic/startup/status", nil)
	w := httptest.NewRecorder()
	api.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status HTTP=%d %s", w.Code, w.Body.String())
	}
	var doc StartupResultO3
	if e := json.Unmarshal(w.Body.Bytes(), &doc); e != nil || doc.State != "present_verified_inventory" || !doc.ReadOnly || doc.Changed {
		t.Fatalf("status %+v %v", doc, e)
	}
	p := httptest.NewRequest(http.MethodPost, "/v1/keenetic/startup/status", strings.NewReader("{}"))
	w = httptest.NewRecorder()
	api.Handler().ServeHTTP(w, p)
	if w.Code != 405 {
		t.Fatal("mutation allowed")
	}
}
