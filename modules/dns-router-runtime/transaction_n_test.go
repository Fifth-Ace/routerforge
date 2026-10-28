package dnsrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type fakeManagedNDMCN struct {
	mu            sync.Mutex
	zones         map[string]bool
	denyAdd       string
	denyBeforeAdd bool
	denyRemove    bool
	original      []string
	commands      []string
}

func (f *fakeManagedNDMCN) Command(_ context.Context, c string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, c)
	if c == "show ip name-server" {
		v := []map[string]any{{"address": "192.0.2.1", "port": 53, "domain": "", "global": 0}}
		for _, z := range f.original {
			v = append(v, map[string]any{"address": "192.168.10.1", "port": 55358, "domain": z, "global": 0})
		}
		for _, z := range []string{"alpha.dpr2n.test", "beta.dpr2n.test"} {
			if f.zones[z] {
				v = append(v, map[string]any{"address": "192.168.10.1", "port": 55358, "domain": z, "global": 0})
			}
		}
		raw, _ := json.Marshal(map[string]any{"server": v})
		return string(raw), nil
	}
	for _, z := range []string{"alpha.dpr2n.test", "beta.dpr2n.test"} {
		if c == "ip name-server 192.168.10.1:55358 "+z {
			if z == f.denyAdd && f.denyBeforeAdd {
				return "", errors.New("injected add was not performed")
			}
			f.zones[z] = true
			if z == f.denyAdd {
				return "", errors.New("partial add failed")
			}
			return "OK", nil
		}
		if c == "no ip name-server 192.168.10.1:55358 "+z {
			if f.denyRemove {
				return "", errors.New("remove failed")
			}
			delete(f.zones, z)
			return "OK", nil
		}
	}
	return "", fmt.Errorf("unexpected ndmc command: %s", c)
}

type fakeManagedGuardN struct{ done int }

func (f *fakeManagedGuardN) Finish() error { f.done++; return nil }
func makeManagedN(t *testing.T) (*ManagedTrialN, *fakeManagedNDMCN, *fakeManagedGuardN, string) {
	t.Helper()
	cfg := lConfig()
	cfg.Upstreams[0].Address = "192.168.10.1:55354"
	cfg.Rules = []Rule{{ID: "alpha", Domain: "alpha.dpr2n.test", Match: "suffix", Upstream: cfg.Upstreams[0].ID, Priority: 10}, {ID: "beta", Domain: "beta.dpr2n.test", Match: "suffix", Upstream: cfg.Upstreams[0].ID, Priority: 10}}
	s := NewConfigStore(filepath.Join(t.TempDir(), "config.json"))
	_, r, e := s.Load()
	if e != nil {
		t.Fatal(e)
	}
	r, e = s.Save(cfg, r)
	if e != nil {
		t.Fatal(e)
	}
	d := &fakeManagedNDMCN{zones: map[string]bool{}}
	guard := &fakeManagedGuardN{}
	runner := &ManagedTrialN{Store: s, Executor: d, Ready: func(rev, listener string) bool { return rev == r && listener == cfg.Listener.Listen }, Arm: func(context.Context, managedJournalN) (managedGuardN, error) { return guard, nil }, Probe: func(context.Context, []string) error { return nil }}
	return runner, d, guard, r
}
func TestDPR2NManagedMultiZoneRollback(t *testing.T) {
	m, d, guard, rev := makeManagedN(t)
	p, e := m.Prepare(context.Background(), rev)
	if e != nil || !p.Ready || len(p.Zones) != 2 || p.Token == "" || p.ChangesApplied {
		t.Fatalf("prepare=%+v err=%v", p, e)
	}
	if _, e = m.Run(context.Background(), rev, "sha256:wrong"); e == nil {
		t.Fatal("accepted stale plan token")
	}
	d.mu.Lock()
	if len(d.zones) != 0 {
		t.Fatal("mutation before valid token")
	}
	d.mu.Unlock()
	out, e := m.Run(context.Background(), rev, p.Token)
	if e != nil || out.State != "pass" || out.Registered != 2 || out.Verified != 2 || !out.BaselineRestored || !out.WatchdogArmed || out.DurableChange || guard.done != 1 || len(d.zones) != 0 {
		t.Fatalf("result=%+v err=%v guard=%+v zones=%v", out, e, guard, d.zones)
	}
}
func TestDPR2NManagedPreflightConflictAndCAS(t *testing.T) {
	m, d, guard, rev := makeManagedN(t)
	if _, e := m.Prepare(context.Background(), "sha256:stale"); !errors.Is(e, ErrStale) {
		t.Fatalf("bad CAS: %v", e)
	}
	d.original = []string{"other.alpha.dpr2n.test"}
	p, e := m.Prepare(context.Background(), rev)
	if e != nil || p.Ready || len(p.Conflicts) != 1 || p.Token != "" {
		t.Fatalf("collision=%+v err=%v", p, e)
	}
	if _, e = m.Run(context.Background(), rev, "sha256:wrong"); e == nil {
		t.Fatal("conflicting trial accepted")
	}
	if guard.done != 0 {
		t.Fatal("guard used for collision")
	}
	d.original = nil
	m.Ready = func(string, string) bool { return false }
	if _, e = m.Prepare(context.Background(), rev); e == nil {
		t.Fatal("inactive listener accepted")
	}
}
func TestDPR2NManagedPartialAndRollbackFailure(t *testing.T) {
	for _, tc := range []struct {
		name            string
		partial, remove bool
	}{{"partial_add", true, false}, {"failed_remove", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			m, d, guard, rev := makeManagedN(t)
			p, e := m.Prepare(context.Background(), rev)
			if e != nil {
				t.Fatal(e)
			}
			if tc.partial {
				d.denyAdd = "beta.dpr2n.test"
			}
			d.denyRemove = tc.remove
			result, e := m.Run(context.Background(), rev, p.Token)
			if e == nil {
				t.Fatal("bad transaction accepted")
			}
			if tc.remove {
				if result.State != "ambiguous" || guard.done != 0 {
					t.Fatalf("ambiguous=%+v", result)
				}
			} else {
				if result.State != "rolled_back_after_failure" || !result.BaselineRestored || guard.done != 1 {
					t.Fatalf("result=%+v", result)
				}
			}
		})
	}
}
func TestDPR2NManagedTokenBoundToInventoryAndConfig(t *testing.T) {
	m, d, _, rev := makeManagedN(t)
	p, e := m.Prepare(context.Background(), rev)
	if e != nil {
		t.Fatal(e)
	}
	// Unrelated DNS changes also require a fresh explicit confirmation.
	d.original = []string{"unrelated.example.net"}
	if _, e := m.Run(context.Background(), rev, p.Token); e == nil || !strings.Contains(e.Error(), "stale") {
		t.Fatalf("inventory token not invalidated: %v", e)
	}
	d.original = nil
	cfg, _, e := m.Store.Load()
	if e != nil {
		t.Fatal(e)
	}
	cfg.Rules[0].Domain = "changed.dpr2n.test"
	if _, e = m.Store.Save(cfg, rev); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Run(context.Background(), rev, p.Token); !errors.Is(e, ErrStale) {
		t.Fatalf("stale config not rejected: %v", e)
	}
}
func TestDPR2NAPIManagedGates(t *testing.T) {
	m, _, _, rev := makeManagedN(t)
	a := NewAPI(m.Store, "")
	exec := func(path, action, cas, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-RouterForge-Action", action)
		if cas != "" {
			req.Header.Set("If-Match", cas)
		}
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, req)
		return w
	}
	if w := exec("/v1/keenetic/domain-managed/prepare", "managed-prepare", rev, `{"operation":"prepare"}`); w.Code != 503 {
		t.Fatalf("default disabled: %d", w.Code)
	}
	a.ManagedN = m
	if w := exec("/v1/keenetic/domain-managed/prepare", "wrong", rev, `{"operation":"prepare"}`); w.Code != 403 {
		t.Fatalf("wrong action: %d", w.Code)
	}
	if w := exec("/v1/keenetic/domain-managed/prepare", "managed-prepare", "", `{"operation":"prepare"}`); w.Code != 428 {
		t.Fatalf("missing CAS: %d", w.Code)
	}
	w := exec("/v1/keenetic/domain-managed/prepare", "managed-prepare", rev, `{"operation":"prepare"}`)
	if w.Code != 200 {
		t.Fatalf("prepare HTTP %d: %s", w.Code, w.Body.String())
	}
	var plan ManagedPrepareN
	if e := json.Unmarshal(w.Body.Bytes(), &plan); e != nil {
		t.Fatal(e)
	}
	if w := exec("/v1/keenetic/domain-managed/trial", "managed-trial", rev, `{"operation":"trial","token":"sha256:wrong"}`); w.Code != 409 {
		t.Fatalf("bad token code=%d body=%s", w.Code, w.Body.String())
	}
	payload, _ := json.Marshal(map[string]any{"operation": "trial", "token": plan.Token})
	if w := exec("/v1/keenetic/domain-managed/trial", "managed-trial", rev, string(payload)); w.Code != 200 {
		t.Fatalf("success code=%d body=%s", w.Code, w.Body.String())
	}
}

// A failed add may never have created its zone. The rollback MUST NOT issue
// `no ip name-server` for that missing domain; this is important on Keenetic,
// where removing a nonexistent entry can itself return an error.
func TestDPR2NPartialAddBeforeMutationSkipsMissingRollback(t *testing.T) {
	m, d, guard, rev := makeManagedN(t)
	p, e := m.Prepare(context.Background(), rev)
	if e != nil {
		t.Fatal(e)
	}
	d.denyAdd = "beta.dpr2n.test"
	d.denyBeforeAdd = true
	out, e := m.Run(context.Background(), rev, p.Token)
	if e == nil || out.State != "rolled_back_after_failure" || !out.BaselineRestored || guard.done != 1 {
		t.Fatalf("result=%+v err=%v", out, e)
	}
	for _, cmd := range d.commands {
		if cmd == "no ip name-server 192.168.10.1:55358 beta.dpr2n.test" {
			t.Fatal("attempted to remove unregistered beta zone")
		}
	}
	if len(d.zones) != 0 {
		t.Fatalf("registrations remain: %+v", d.zones)
	}
}
