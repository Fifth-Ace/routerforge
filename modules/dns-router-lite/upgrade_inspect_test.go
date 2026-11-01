package lite

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

func p191LegacyFixture(t *testing.T) (*GateManager, *modelKernel, *LinuxGate, string, UpgradeEvidence, []byte) {
	t.Helper()
	g, m, k := gateFixture(t)
	g.topology = func() (GateTopology, error) { return AnalyzeGateTopology(dstestTopology()) }
	g.candidates = func() (string, []string, error) { top, err := g.topology(); return top.IPv4, top.IPv6, err }
	c := fixtureConfig(t)
	c.Service.Listen = "192.168.10.1:55358"
	path := filepath.Join(t.TempDir(), "config.json")
	store := dr.NewConfigStore(path)
	_, rev, _ := store.Load()
	rev, err := store.Save(c, rev)
	if err != nil {
		t.Fatal(err)
	}
	raw := dr.ExportContent(c)
	if err = atomicFile(path+".service-active.json", raw); err != nil {
		t.Fatal(err)
	}
	e := UpgradeEvidence{Active: true, AppliedRevision: rev}
	e.Connection.State = "disconnected"
	e.Gate.State = "STOCK"
	return g, m, k, path, e, raw
}
func TestP191CleanStockMigrationNeverRequiresNewGateActivation(t *testing.T) {
	g, m, k, path, e, raw := p191LegacyFixture(t)
	// Even a currently unavailable bridge/listener IPv6 or an unrelated routed
	// overlap must not become a new-activation requirement for clean STOCK.
	g.topology = func() (GateTopology, error) {
		t.Fatal("clean migration required topology activation plan")
		return GateTopology{}, nil
	}
	g.candidates = func() (string, []string, error) { t.Fatal("clean migration built Gate plan"); return "", nil, nil }
	g.probe = func(context.Context, string, GatePlan) error { t.Fatal("migration ran activation probe"); return nil }
	ct := k.Conntrack.Table.(*modelCT)
	ct.flows = []CTFlow{testFlow(t, "192.168.10.55", "9.9.9.9", 17, 53, 1)} // Plain DNS is not a residual redirect.
	before, _ := os.ReadFile(path)
	first, err := g.PrepareLegacyUpgrade(context.Background(), path, e)
	if err != nil {
		t.Fatal(err)
	}
	if !first.ServiceEnabled || first.InterceptionEnabled || first.AppliedRevision != e.AppliedRevision || first.Consent != "" {
		t.Fatal("lost legacy ON/OFF/revision", first)
	}
	applied, err := (IntentStore{ConfigPath: path}).Applied(first)
	if err != nil || !bytes.Equal(dr.ExportContent(applied), raw) {
		t.Fatal("applied config changed", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("draft rewritten")
	}
	if _, err = os.Lstat(path + ".legacy-gate-cleanup.json"); !os.IsNotExist(err) {
		t.Fatal("clean migration staged unnecessary cleanup", err)
	}
	intentBefore, _ := os.ReadFile(path + ".intent.json")
	second, err := g.PrepareLegacyUpgrade(context.Background(), path, e)
	if err != nil || first != second {
		t.Fatal("retry not idempotent", second, err)
	}
	intentAfter, _ := os.ReadFile(path + ".intent.json")
	if !bytes.Equal(intentBefore, intentAfter) {
		t.Fatal("retry rewrote intent")
	}
	if len(m.mutations) != 0 || len(ct.deleted) != 0 || g.Status()["active"] != false {
		t.Fatal("preparer mutated firewall/conntrack or enabled Gate")
	}
}
func TestP191ResidualLegacyResourcesAndCachedTuplesUseHandoff(t *testing.T) {
	for _, which := range []string{"rules_without_jump", "rules_with_jump", "conntrack_only", "scoped_v6_rules_without_flows"} {
		t.Run(which, func(t *testing.T) {
			g, m, k, path, e, _ := p191LegacyFixture(t)
			p, err := g.legacyPlan("192.168.10.1:55358")
			if err != nil {
				t.Fatal(err)
			}
			f := p.Families[0]
			if which == "scoped_v6_rules_without_flows" {
				f = p.Families[1]
			}
			if which != "conntrack_only" {
				m.chains[f.Binary][gateChain] = chainRules(p, f, true)
				if which == "rules_with_jump" {
					m.chains[f.Binary]["PREROUTING"] = append([][]string{jumpRule(p, true)}, m.chains[f.Binary]["PREROUTING"]...)
				}
				m.sets[f.Set] = &modelSet{family: f.Family, members: map[string]time.Time{f.Prefixes[0]: time.Now().Add(time.Minute)}}
			}
			ct := k.Conntrack.Table.(*modelCT)
			foreign := testFlow(t, "192.168.20.5", "9.9.9.9", 17, 53, 2)
			ct.flows = []CTFlow{foreign}
			if which != "scoped_v6_rules_without_flows" {
				ct.flows = append(ct.flows, testFlow(t, "192.168.10.55", "9.9.9.9", 17, 55358, 1))
			}
			state, err := g.PrepareLegacyUpgrade(context.Background(), path, e)
			if err != nil {
				t.Fatal(err)
			}
			if state.InterceptionEnabled || !state.ServiceEnabled {
				t.Fatal("orphan rules inferred consent", state)
			}
			if len(m.mutations) != 0 || len(ct.deleted) != 0 {
				t.Fatal("read-only preparation mutated live rules/tuples")
			}
			if _, err = os.Stat(path + ".legacy-gate-cleanup.json"); err != nil {
				t.Fatal("residual resources not staged", err)
			}
			if _, err = g.PrepareLegacyUpgrade(context.Background(), path, e); err != nil {
				t.Fatal("residual handoff retry failed", err)
			}
			manager := &Manager{Path: path, Gate: g}
			g.SetOwnershipPath(path + ".gate-owned.json")
			if err = manager.finishLegacyCleanup(); err != nil {
				t.Fatal(err)
			}
			if len(ct.flows) != 1 || !bytes.Equal(ct.flows[0].ID, foreign.ID) {
				t.Fatal("cleanup affected foreign conntrack")
			}
			if _, exists := m.chains[f.Binary][gateChain]; exists {
				t.Fatal("legacy chain retained")
			}
			if g.Status()["active"] != false {
				t.Fatal("cleanup enabled Gate")
			}
		})
	}
}
func TestP191UnprovenOwnershipAndInspectionFailureFailClosed(t *testing.T) {
	for _, which := range []string{"foreign_chain", "orphan_family", "foreign_goto", "incomplete_dump", "isolated_set", "foreign_filter", "foreign_redirect", "linklocal_tuple", "conntrack_denied", "bad_revision", "bad_existing_intent"} {
		t.Run(which, func(t *testing.T) {
			g, m, k, path, e, _ := p191LegacyFixture(t)
			ct := k.Conntrack.Table.(*modelCT)
			switch which {
			case "orphan_family":
				m.chains["ip6tables"][gateChain] = [][]string{{"-A", gateChain, "-j", "RETURN"}}
				g.topology = func() (GateTopology, error) {
					raw := dstestTopology()
					raw[0].Addresses = raw[0].Addresses[:1]
					return AnalyzeGateTopology(raw)
				}
			case "incomplete_dump":
				original := k.Tables.Exec
				k.Tables.Exec = func(ctx context.Context, binary string, args ...string) (string, error) {
					if binary == "iptables" && strings.Contains(strings.Join(args, " "), "-t nat -S") {
						return "", nil
					}
					return original(ctx, binary, args...)
				}
			case "foreign_goto":
				p, _ := g.legacyPlan("192.168.10.1:55358")
				m.chains["iptables"][gateChain] = chainRules(p, p.Families[0], true)
				m.chains["iptables"]["PREROUTING"] = append(m.chains["iptables"]["PREROUTING"], []string{"-A", "PREROUTING", "-g", gateChain})
			case "foreign_chain":
				m.chains["iptables"][gateChain] = [][]string{{"-A", gateChain, "-j", "ACCEPT"}}
			case "isolated_set":
				m.sets[gateSet4] = &modelSet{family: "inet", members: map[string]time.Time{}}
			case "foreign_filter":
				m.filters["iptables"]["INPUT"] = [][]string{{"-A", "INPUT", "-m", "set", "--match-set", gateSet4, "src", "-j", "ACCEPT"}}
			case "foreign_redirect":
				m.chains["iptables"]["PREROUTING"] = append(m.chains["iptables"]["PREROUTING"], []string{"-A", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", "55358"})
				ct.flows = []CTFlow{testFlow(t, "192.168.10.5", "9.9.9.9", 17, 55358, 1)}
			case "linklocal_tuple":
				f := testFlow(t, "fe80::55", "fe80::52ff:20ff:fed3:831", 17, 55358, 1)
				f.Reply.Src = net.ParseIP("fe80::52ff:20ff:fed3:831")
				ct.flows = []CTFlow{f}
			case "conntrack_denied":
				ct.err = errors.New("netlink permission denied")
			case "bad_revision":
				e.AppliedRevision = "sha256:" + strings.Repeat("a", 64)
			case "bad_existing_intent":
				os.WriteFile(path+".intent.json", []byte("{broken"), 0600)
			}
			if _, err := g.PrepareLegacyUpgrade(context.Background(), path, e); err == nil {
				t.Fatal("unknown/foreign state accepted")
			}
			if len(m.mutations) != 0 || len(ct.deleted) != 0 {
				t.Fatal("rejected preflight mutated firewall")
			}
			if which != "bad_existing_intent" {
				if _, err := os.Stat(path + ".intent.json"); !os.IsNotExist(err) {
					t.Fatal("rejected migration wrote intent", err)
				}
			}
		})
	}
}
func TestP191ForeignDNSWithoutLegacyResourcesDoesNotBlockStockMigration(t *testing.T) {
	g, m, _, path, e, _ := p191LegacyFixture(t)
	foreign := []string{"-A", "PREROUTING", "-p", "udp", "--dport", "53", "-j", "AWGM-DNS"}
	m.chains["iptables"]["PREROUTING"] = append(m.chains["iptables"]["PREROUTING"], foreign)
	m.chains["iptables"]["AWGM-DNS"] = [][]string{{"-A", "AWGM-DNS", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", "1601"}}
	if _, err := g.PrepareLegacyUpgrade(context.Background(), path, e); err != nil {
		t.Fatal("new Gate conflict incorrectly blocked STOCK migration", err)
	}
	if len(m.mutations) != 0 {
		t.Fatal("foreign firewall changed")
	}
}
