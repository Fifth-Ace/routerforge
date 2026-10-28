package dnsrouter

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

type fakeGuardO2 struct{}

func (*fakeGuardO2) Finish() error { return nil }

type fixtureO2 struct {
	mu         sync.Mutex
	zones      map[string]bool
	failAdd    string
	failRemove string
	foreign    bool
}

func (f *fixtureO2) Command(_ context.Context, c string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c == "show ip name-server" {
		servers := []map[string]any{{"address": "192.0.2.1", "port": 53, "domain": "", "global": 0}}
		for _, d := range []string{"alpha.dpr2n.test", "beta.dpr2n.test"} {
			if f.zones[d] {
				host := "192.168.10.1"
				if f.foreign && d == "alpha.dpr2n.test" {
					host = "192.168.10.2"
				}
				servers = append(servers, map[string]any{"address": host, "port": 55358, "domain": d, "global": 0})
			}
		}
		b, _ := json.Marshal(map[string]any{"server": servers})
		return string(b), nil
	}
	for _, d := range []string{"alpha.dpr2n.test", "beta.dpr2n.test"} {
		if c == "ip name-server 192.168.10.1:55358 "+d {
			f.zones[d] = true
			if f.failAdd == d {
				return "", errors.New("injected partial add failure")
			}
			return "OK", nil
		}
		if c == "no ip name-server 192.168.10.1:55358 "+d {
			if f.failRemove == d {
				return "", errors.New("injected remove failure")
			}
			delete(f.zones, d)
			return "OK", nil
		}
	}
	return "", errors.New("unexpected ndmc command: " + c)
}
func newDurableO2(t *testing.T) (*DurableManagerO2, *fixtureO2) {
	t.Helper()
	f := &fixtureO2{zones: map[string]bool{}}
	store := NewConfigStore(filepath.Join(t.TempDir(), "dns-router-config.json"))
	_, emptyRevision, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := lConfig()
	cfg.Upstreams[0].Address = "192.168.10.1:55354"
	cfg.Rules = []Rule{
		{ID: "o2_alpha", Domain: "alpha.dpr2n.test", Match: "suffix", Upstream: cfg.Upstreams[0].ID, Priority: 10},
		{ID: "o2_beta", Domain: "beta.dpr2n.test", Match: "suffix", Upstream: cfg.Upstreams[0].ID, Priority: 20},
	}
	if _, err = store.Save(cfg, emptyRevision); err != nil {
		t.Fatal(err)
	}
	ledger := NewLedgerStoreO(filepath.Join(t.TempDir(), "ledger.json"))
	m := &DurableManagerO2{Store: store, Ledger: ledger, Executor: f, HostGate: func() error { return nil }, Probe: func(context.Context, []string) error { return nil }, Arm: func(context.Context, managedJournalN) (managedGuardN, error) { return &fakeGuardO2{}, nil }, ArmDisable: func(context.Context, durableJournalO2) (managedGuardN, error) { return &fakeGuardO2{}, nil }}
	m.Ready = func(rev, listen string) bool {
		_, current, err := store.Load()
		return err == nil && current == rev && listen == "192.168.10.1:55358"
	}
	return m, f
}
func revisionO2(t *testing.T, m *DurableManagerO2) string {
	t.Helper()
	_, rev, err := m.Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return rev
}
func TestDPR2O2ApplyDisableRoundTrip(t *testing.T) {
	m, f := newDurableO2(t)
	ctx := context.Background()
	rev := revisionO2(t, m)
	p, e := m.Prepare(ctx, rev)
	if e != nil || !p.Ready {
		t.Fatalf("prepare %+v %v", p, e)
	}
	o, e := m.Apply(ctx, rev, p.Token)
	if e != nil || o.State != "active" || !o.DurableChange || !o.WatchdogArmed {
		t.Fatalf("apply %+v %v", o, e)
	}
	stored, sha, e := m.Ledger.Load()
	if e != nil || stored.State != "active" || stored.Baseline == "" || !f.zones["alpha.dpr2n.test"] || !f.zones["beta.dpr2n.test"] {
		t.Fatalf("ledger %+v %v", stored, e)
	}
	if _, e = m.Prepare(ctx, rev); e == nil {
		t.Fatal("prepare accepted active ledger")
	}
	o, e = m.Disable(ctx, rev, sha)
	if e != nil || o.State != "disabled" || !o.BaselineRestored || !o.DurableChange {
		t.Fatalf("disable %+v %v", o, e)
	}
	stored, _, e = m.Ledger.Load()
	if e != nil || stored != nil || len(f.zones) != 0 {
		t.Fatalf("remaining %+v %v %v", stored, e, f.zones)
	}
}
func TestDPR2O2PartialAddRollbackAndLedgerCleanup(t *testing.T) {
	m, f := newDurableO2(t)
	f.failAdd = "beta.dpr2n.test"
	p, _ := m.Prepare(context.Background(), revisionO2(t, m))
	o, e := m.Apply(context.Background(), revisionO2(t, m), p.Token)
	if e == nil || o.State != "rolled_back_after_failure" || !o.BaselineRestored || len(f.zones) != 0 {
		t.Fatalf("rollback %+v %v %+v", o, e, f.zones)
	}
	x, _, e := m.Ledger.Load()
	if e != nil || x != nil {
		t.Fatalf("orphan ledger %+v %v", x, e)
	}
}
func TestDPR2O2RejectForeignOwnershipAndStaleCAS(t *testing.T) {
	m, f := newDurableO2(t)
	rev := revisionO2(t, m)
	p, _ := m.Prepare(context.Background(), rev)
	if _, e := m.Apply(context.Background(), rev, "wrong"); !errors.Is(e, ErrManagedTokenN) {
		t.Fatalf("token %v", e)
	}
	_, e := m.Apply(context.Background(), rev, p.Token)
	if e != nil {
		t.Fatal(e)
	}
	_, sha, e := m.Ledger.Load()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Disable(context.Background(), rev, "stale"); !errors.Is(e, ErrStale) {
		t.Fatalf("stale ledger %v", e)
	}
	f.foreign = true
	if _, e = m.Disable(context.Background(), rev, sha); e == nil {
		t.Fatal("foreign-owned zone was accepted")
	}
	x, _, e := m.Ledger.Load()
	if e != nil || x.State != "active" {
		t.Fatalf("ledger mutated %v %v", x, e)
	}
}
func TestDPR2O2RollbackFailureKeepsPendingLedger(t *testing.T) {
	m, f := newDurableO2(t)
	f.failAdd = "beta.dpr2n.test"
	f.failRemove = "alpha.dpr2n.test"
	p, _ := m.Prepare(context.Background(), revisionO2(t, m))
	o, e := m.Apply(context.Background(), revisionO2(t, m), p.Token)
	if e == nil || o.State != "ambiguous" {
		t.Fatalf("ambiguous %+v %v", o, e)
	}
	ledger, _, e := m.Ledger.Load()
	if e != nil || ledger.State != "pending" {
		t.Fatalf("pending %+v %v", ledger, e)
	}
}
func TestDPR2O2DisablePartialFailureRestoresActive(t *testing.T) {
	m, f := newDurableO2(t)
	rev := revisionO2(t, m)
	p, _ := m.Prepare(context.Background(), rev)
	_, e := m.Apply(context.Background(), rev, p.Token)
	if e != nil {
		t.Fatal(e)
	}
	_, sha, _ := m.Ledger.Load()
	f.failRemove = "alpha.dpr2n.test"
	o, e := m.Disable(context.Background(), rev, sha)
	if e == nil || o.State != "restored_after_failure" {
		t.Fatalf("restore %+v %v", o, e)
	}
	l, _, e := m.Ledger.Load()
	if e != nil || l.State != "active" || !f.zones["alpha.dpr2n.test"] || !f.zones["beta.dpr2n.test"] {
		t.Fatalf("state %+v %v %+v", l, e, f.zones)
	}
}
