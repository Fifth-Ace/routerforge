package lite

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

func lifecycleFixture(t *testing.T) (*Manager, string, *modelKernel) {
	t.Helper()
	c := fixtureConfig(t)
	c.Service.AllowedNetworks = append(c.Service.AllowedNetworks, "192.168.10.0/24")
	c.Rules = append(c.Rules, dr.Rule{ID: "added172", Domain: "more.example", Match: "suffix", Upstream: "chosen", Priority: 100}, dr.Rule{ID: "added173", Domain: "last.example", Match: "exact", Upstream: "chosen", Priority: 100})
	if len(c.Rules) != 174 {
		t.Fatal("wrong regression model size")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	store := dr.NewConfigStore(path)
	_, rev, _ := store.Load()
	rev, err := store.Save(c, rev)
	if err != nil {
		t.Fatal(err)
	}
	var count atomic.Int32
	dest := dnsFixture(t, "LIFECYCLE", &count)
	g, k, _ := gateFixture(t)
	g.planFactory = func(listen string) (GatePlan, error) {
		_, port, e := net.SplitHostPort(listen)
		return GatePlan{Version: 1, Interface: "br0", RouterAddresses: []string{"192.168.10.1"}, Port: port, Families: []GateFamily{{Binary: "iptables", Set: gateSet4, Family: "inet", Prefixes: []string{"192.168.10.0/24"}}}}, e
	}
	m := &Manager{Path: path, Store: store, Gate: g, Factory: func(dr.Upstream) (upstream.Upstream, error) {
		return upstream.AddressToUpstream(dest, &upstream.Options{})
	}}
	t.Cleanup(func() { m.Close() })
	return m, rev, k
}
func resumedManager(t *testing.T, old *Manager) *Manager {
	t.Helper()
	g := NewGateManager()
	g.kernel = old.Gate.kernel
	g.candidates = old.Gate.candidates
	g.locate = old.Gate.locate
	g.planFactory = old.Gate.planFactory
	m := &Manager{Path: old.Path, Store: old.Store, Factory: old.Factory, Gate: g}
	t.Cleanup(func() { m.Close() })
	return m
}
func TestP19UpgradeRebootAndExplicitOFFPreserveIntent(t *testing.T) {
	for _, on := range []bool{false, true} {
		t.Run(map[bool]string{false: "OFF_OFF", true: "ON_ON"}[on], func(t *testing.T) {
			m, rev, _ := lifecycleFixture(t)
			if on {
				if e := m.control("start", rev); e != nil {
					t.Fatal(e)
				}
				if e := m.gateControl(context.Background(), "enable", rev); e != nil {
					t.Fatal(e)
				}
			} else {
				if e := m.control("stop", rev); e != nil {
					t.Fatal(e)
				}
			}
			before, err := os.ReadFile(m.intentStore().Path())
			if err != nil {
				t.Fatal(err)
			}
			// Execute the actual package lifecycle dispatch in an isolated path. The
			// operation it invokes is then applied to the real manager and DNS sockets.
			raw, err := os.ReadFile("../dns-router-runtime/packaging/prerm-dev")
			if err != nil {
				t.Fatal(err)
			}
			temp := t.TempDir()
			fake := filepath.Join(temp, "init")
			log := filepath.Join(temp, "op")
			if err = os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' \"$1\" > '"+log+"'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			script := strings.ReplaceAll(string(raw), "/opt/etc/init.d/S95routerforge-dns-router", fake)
			cmd := exec.Command("sh", "-c", script, "prerm", "upgrade")
			if out, e := cmd.CombinedOutput(); e != nil {
				t.Fatalf("package upgrade dispatch: %v %s", e, out)
			}
			operation, _ := os.ReadFile(log)
			if string(operation) != "maintenance" {
				t.Fatal("upgrade erases user ON/OFF choice")
			}
			if e := m.control(string(operation), rev); e != nil {
				t.Fatal(e)
			}
			after, _ := os.ReadFile(m.intentStore().Path())
			if string(before) != string(after) {
				t.Fatal("maintenance rewrote persistent user intent")
			}
			next := resumedManager(t, m)
			if e := next.Resume(); e != nil {
				t.Fatal(e)
			}
			if next.ActiveState() != on || next.Gate.Status()["active"] != on {
				t.Fatalf("wrong post-upgrade state: %v %v", next.ActiveState(), next.Gate.Status())
			}
			if on {
				if len(next.applied.Rules) != 174 {
					t.Fatal("domain model changed")
				}
				ask(t, next.applied.Service.Listen, "udp", "outside.example")
			}
			// A restart/reboot reads exactly the committed snapshot, never the draft.
			if e := next.Close(); e != nil {
				t.Fatal(e)
			}
			c, current, _ := next.Store.Load()
			c.Rules = nil
			if _, e := next.Store.Save(c, current); e != nil {
				t.Fatal(e)
			}
			reboot := resumedManager(t, next)
			if e := reboot.Resume(); e != nil {
				t.Fatal(e)
			}
			if reboot.ActiveState() != on || reboot.Gate.Status()["active"] != on {
				t.Fatal("reboot lost persisted intent")
			}
			if on && len(reboot.applied.Rules) != 174 {
				t.Fatal("unapplied draft activated on reboot")
			}
		})
	}
}
func TestP19FirstMigrationRecoveryDoesNotProveConsent(t *testing.T) {
	m, rev, _ := lifecycleFixture(t)
	c, _, _ := m.Store.Load()
	if e := atomicFile(m.snapshotPath(), dr.ExportContent(c)); e != nil {
		t.Fatal(e)
	}
	evidence := UpgradeEvidence{Active: true, AppliedRevision: rev}
	evidence.Connection.State = "disconnected"
	evidence.Gate.State = "RECOVERY_REQUIRED"
	state, e := PrepareUpgrade(m.Path, evidence)
	if e != nil {
		t.Fatal(e)
	}
	if !state.ServiceEnabled || state.InterceptionEnabled || state.Consent != "" {
		t.Fatalf("ambiguous migration activated Gate: %+v", state)
	}
	if e = os.Remove(m.snapshotPath()); e != nil {
		t.Fatal(e)
	} // r1267's old prerm.
	if e = m.Resume(); e != nil {
		t.Fatal(e)
	}
	if !m.ActiveState() || m.Gate.Status()["active"] != false {
		t.Fatal("migration did not preserve DNS-only ON")
	}
	ask(t, m.applied.Service.Listen, "tcp", "outside.example")
	if e = RestoreUpgradeMirror(m.Path); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(m.snapshotPath()); e != nil {
		t.Fatal("legacy rollback snapshot unavailable")
	}
}
func TestP19ListenerCrashRecoversAppliedNotDraft(t *testing.T) {
	m, rev, _ := lifecycleFixture(t)
	if e := m.control("start", rev); e != nil {
		t.Fatal(e)
	}
	if e := m.gateControl(context.Background(), "enable", rev); e != nil {
		t.Fatal(e)
	}
	c, draft, _ := m.Store.Load()
	c.Rules = nil
	if _, e := m.Store.Save(c, draft); e != nil {
		t.Fatal(e)
	}
	if e := m.engine.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if e := m.controllerTick(ctx); e != nil {
		t.Fatal(e)
	}
	if len(m.applied.Rules) != 174 || m.revision != rev || m.Gate.Status()["active"] != true {
		t.Fatal("listener crash recovery lost applied config or Gate ON")
	}
	ask(t, m.applied.Service.Listen, "udp", "outside.example")
}
func TestP19OFFHookCannotEnableAndFailedApplyRestoresGate(t *testing.T) {
	m, rev, k := lifecycleFixture(t)
	if e := m.Resume(); e != nil {
		t.Fatal(e)
	}
	if e := m.gateControl(context.Background(), "notify", ""); e != nil {
		t.Fatal(e)
	}
	if len(k.mutations) != 0 || m.ActiveState() {
		t.Fatal("OFF reload hook activated service")
	}
	if e := m.control("start", rev); e != nil {
		t.Fatal(e)
	}
	if e := m.gateControl(context.Background(), "enable", rev); e != nil {
		t.Fatal(e)
	}
	c, current, _ := m.Store.Load()
	c.Service.Listen = "192.168.254.254:55358"
	next, e := m.Store.Save(c, current)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.control("apply", next); e == nil {
		t.Fatal("unavailable listener accepted")
	}
	if m.revision != rev || m.Gate.Status()["active"] != true {
		t.Fatal("failed apply did not restore old engine and Gate")
	}
	state, e := m.intentStore().Load()
	if e != nil || state.AppliedRevision != rev || !state.InterceptionEnabled {
		t.Fatal("failed apply corrupted committed intent", e)
	}
}
func TestP19DeadListenerQuiescesWithoutXTablesLock(t *testing.T) {
	g, k, driver := gateFixture(t)
	if e := g.Enable(context.Background(), "192.168.10.1:55358"); e != nil {
		t.Fatal(e)
	}
	ct := driver.Conntrack.Table.(*modelCT)
	ct.flows = append(ct.flows, testFlow(t, "192.168.10.5", "9.9.9.9", 17, 55358, 1))
	k.mu.Lock()
	k.locks = -1
	k.mu.Unlock()
	g.SetProbe(func(context.Context, string, GatePlan) error { return errors.New("dead DNS listener") })
	if e := g.Reconcile(context.Background(), false); e == nil {
		t.Fatal("dead listener not detected")
	}
	if len(ct.flows) != 0 || strings.HasPrefix(k.packet("inet", "br0", "192.168.10.5", "9.9.9.9", "udp", 53), "ROUTERFORGE") {
		t.Fatal("dead DNS still receives flows during xtables lock")
	}
}

func TestP19CorruptDraftDoesNotBlockOFFAndInvalidSnapshotNeverStarts(t *testing.T) {
	m, rev, _ := lifecycleFixture(t)
	if err := m.control("start", rev); err != nil {
		t.Fatal(err)
	}
	if err := m.gateControl(context.Background(), "enable", rev); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.Path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.control("stop", ""); err != nil {
		t.Fatal("corrupt draft blocked explicit OFF", err)
	}
	state, err := m.intentStore().Load()
	if err != nil || state.ServiceEnabled || state.InterceptionEnabled || m.ActiveState() {
		t.Fatal("OFF not persisted", err)
	}
	state.ServiceEnabled = true
	if err = m.intentStore().Save(state); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := m.intentStore().SnapshotPath(state.AppliedRevision)
	if err = os.WriteFile(snapshot, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	next := resumedManager(t, m)
	if err = next.Resume(); err == nil || next.ActiveState() || next.Gate.Status()["active"] == true {
		t.Fatal("invalid applied snapshot activated", err)
	}
}

func TestP19LegacyHandoffCleansOnlyProvenOldRulesAndNATTuples(t *testing.T) {
	m, rev, kmodel := lifecycleFixture(t)
	c, _, _ := m.Store.Load()
	if err := atomicFile(m.snapshotPath(), dr.ExportContent(c)); err != nil {
		t.Fatal(err)
	}
	p, err := m.Gate.makePlan(c.Service.Listen)
	if err != nil {
		t.Fatal(err)
	}
	k := m.Gate.kernel.(*LinuxGate)
	kmodel.mu.Lock()
	for _, f := range p.Families {
		kmodel.chains[f.Binary][gateChain] = chainRules(p, f, true)
		kmodel.chains[f.Binary]["PREROUTING"] = append([][]string{jumpRule(p, true)}, kmodel.chains[f.Binary]["PREROUTING"]...)
		kmodel.sets[f.Set] = &modelSet{family: f.Family, members: map[string]time.Time{f.Prefixes[0]: time.Now().Add(time.Minute)}}
	}
	kmodel.mu.Unlock()
	ct := k.Conntrack.Table.(*modelCT)
	port, _ := strconv.Atoi(p.Port)
	ct.flows = []CTFlow{testFlow(t, "192.168.10.55", "9.9.9.9", 17, uint16(port), 1), testFlow(t, "192.168.20.55", "9.9.9.9", 17, 53, 2)}
	if err = m.Gate.StageLegacyCleanup(context.Background(), m.Path, c.Service.Listen); err != nil {
		t.Fatal(err)
	}
	evidence := UpgradeEvidence{Active: true, AppliedRevision: rev}
	evidence.Connection.State = "disconnected"
	evidence.Gate.State = "RECOVERY_REQUIRED"
	if _, err = PrepareUpgrade(m.Path, evidence); err != nil {
		t.Fatal(err)
	}
	// The legacy prerm may remove its snapshot but leave rules/conntrack behind.
	os.Remove(m.snapshotPath())
	if err = m.Resume(); err != nil {
		t.Fatal(err)
	}
	if !m.ActiveState() || m.Gate.Status()["active"] == true {
		t.Fatal("legacy migration enabled capture")
	}
	if len(ct.deleted) != 1 || len(ct.flows) != 1 {
		t.Fatal("legacy cleanup touched foreign flow")
	}
	if _, err = os.Stat(m.Path + ".legacy-gate-cleanup.json"); !os.IsNotExist(err) {
		t.Fatal("one-time cleanup not committed", err)
	}
	if got := kmodel.packet("inet", "br0", "192.168.10.55", "9.9.9.9", "udp", 53); got != "STOCK" {
		t.Fatal(got)
	}
}
func TestP19OfflineStopPreservesLastExplicitOFFAndRejectsLiveOwner(t *testing.T) {
	m, rev, _ := lifecycleFixture(t)
	if err := m.control("start", rev); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "not-created.sock")
	release, err := AcquireProcessLock(m.Path + ".daemon.lock")
	if err != nil {
		t.Fatal(err)
	}
	if err = StopOffline(m.Path, socket); err == nil {
		t.Fatal("offline stop raced live owner")
	}
	release()
	before, err := m.intentStore().Load()
	if err != nil || !before.ServiceEnabled {
		t.Fatal("refused stop changed intent", err)
	}
	if err = StopOffline(m.Path, socket); err != nil {
		t.Fatal(err)
	}
	after, err := m.intentStore().Load()
	if err != nil || after.ServiceEnabled || after.InterceptionEnabled {
		t.Fatal("offline OFF lost", err)
	}
}
