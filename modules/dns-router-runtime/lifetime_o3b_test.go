package dnsrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testGateO3B() error { return nil }

func TestDPR2O3BLifetimeCleanupActiveAndRetainLedger(t *testing.T) {
	m, f := newDurableO2(t)
	activateO3(t, m)
	out, err := cleanupOnDaemonExitO3B(context.Background(), m.Ledger, f, testGateO3B)
	if err != nil || out.State != "dead_listener_zones_removed" || !out.ExactBaseline || len(out.Zones) != 2 || len(f.zones) != 0 {
		t.Fatalf("unsafe cleanup: %+v err=%v remaining=%v", out, err, f.zones)
	}
	l, _, err := m.Ledger.Load()
	if err != nil || l == nil || l.State != "active" {
		t.Fatalf("ownership lost %+v %v", l, err)
	}
	out, err = cleanupOnDaemonExitO3B(context.Background(), m.Ledger, f, testGateO3B)
	if err != nil || out.State != "baseline_already_restored" || !out.ExactBaseline {
		t.Fatalf("not idempotent: %+v %v", out, err)
	}
}

func TestDPR2O3BLifetimeCleanupPendingAndDeactivating(t *testing.T) {
	for _, state := range []string{"pending", "deactivating"} {
		t.Run(state, func(t *testing.T) {
			m, f := newDurableO2(t)
			activateO3(t, m)
			l, sha, err := m.Ledger.Load()
			if err != nil {
				t.Fatal(err)
			}
			l.State = state
			if _, err = m.Ledger.SaveCAS(*l, sha); err != nil {
				t.Fatal(err)
			}
			out, err := cleanupOnDaemonExitO3B(context.Background(), m.Ledger, f, testGateO3B)
			if err != nil || !out.ExactBaseline || len(f.zones) != 0 {
				t.Fatalf("cleanup failed %+v %v %+v", out, err, f.zones)
			}
			l, _, err = m.Ledger.Load()
			if err != nil || l == nil || l.State != state {
				t.Fatalf("incorrect state %+v %v", l, err)
			}
		})
	}
}

func TestDPR2O3BLifetimeCleanupRejectsForeignAndNoOwnership(t *testing.T) {
	m, f := newDurableO2(t)
	f.zones["alpha.dpr2n.test"] = true
	out, err := cleanupOnDaemonExitO3B(context.Background(), m.Ledger, f, testGateO3B)
	if err != nil || out.State != "no_ownership" || !f.zones["alpha.dpr2n.test"] {
		t.Fatalf("adopted unowned %+v %v", out, err)
	}
	delete(f.zones, "alpha.dpr2n.test")
	activateO3(t, m)
	f.foreign = true
	_, err = cleanupOnDaemonExitO3B(context.Background(), m.Ledger, f, testGateO3B)
	if err == nil || !f.zones["alpha.dpr2n.test"] || !f.zones["beta.dpr2n.test"] {
		t.Fatalf("foreign ownership ignored: %v %+v", err, f.zones)
	}
}

func TestDPR2O3BLifetimeIdentityAndPathValidation(t *testing.T) {
	start, err := procStartO3B(1)
	if err != nil || start == "" {
		t.Fatalf("linux proc start missing: %q %v", start, err)
	}
	j := lifetimeJournalO3B{Schema: 1, Parent: 5, ParentStart: start, LedgerPath: "/tmp/rf-test/config.json.managed-state.json", Listen: "192.168.10.1:55358", Started: 123}
	if err := validLifetimeJournalO3B(j); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/etc/passwd.managed-state.json", "/tmp/evil/ledger.managed-state.json", "/tmp/rf-test/../evil.managed-state.json"} {
		j.LedgerPath = path
		if err := validLifetimeJournalO3B(j); err == nil {
			t.Fatalf("accepted unsafe journal %q", path)
		}
	}
}

func TestDPR2O3BAPIPreventsStoppingOwnedForwarderOrSavingPolicy(t *testing.T) {
	m, _ := newDurableO2(t)
	activateO3(t, m)
	api := NewAPI(m.Store, "")
	api.LedgerO = m.Ledger
	api.DurableO2 = m
	api.Lifecycle = NewLifecycleManager(m.Store, ShadowOptions{})
	_, rev, err := m.Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"stop", "apply"} {
		req := httptest.NewRequest(http.MethodPost, "/v1/shadow/control", strings.NewReader(`{"operation":"`+operation+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-RouterForge-Action", "shadow-control")
		req.Header.Set("If-Match", rev)
		w := httptest.NewRecorder()
		api.Handler().ServeHTTP(w, req)
		if w.Code != 409 {
			t.Fatalf("listener %s accepted HTTP %d: %s", operation, w.Code, w.Body.String())
		}
	}
	cfg, _, err := m.Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(cfg)
	req := httptest.NewRequest(http.MethodPut, "/v1/config", strings.NewReader(string(raw)))
	req.Header.Set("If-Match", rev)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-RouterForge-Action", "save")
	w := httptest.NewRecorder()
	api.Handler().ServeHTTP(w, req)
	if w.Code != 409 {
		t.Fatalf("saved policy over active owner: %d %s", w.Code, w.Body.String())
	}
}
