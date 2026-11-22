package dnsrouter

import (
	"fmt"
	"strings"
	"testing"
)

func p7Fixture() Config {
	return Config{SchemaVersion: 1,
		Upstreams: []Upstream{
			{ID: "a", Transport: "udp", Address: "9.9.9.9:53"},
			{ID: "b", Transport: "tcp", Address: "1.1.1.1:53"},
		},
		Groups:   []DNSGroup{{ID: "group-a", Name: "A", Servers: []string{"a"}}, {ID: "group-b", Name: "B", Servers: []string{"b"}}},
		Listener: &ListenerConfig{Mode: "lan", Listen: "192.168.10.1:55358", AllowedClients: []string{"192.168.10.1", "127.0.0.1"}},
		Rules: []Rule{
			{ID: "parent", Domain: "example.com", Match: "suffix", GroupID: "group-a", Upstream: "a"},
			{ID: "child", Domain: "api.example.com", Match: "exact", GroupID: "group-b", Upstream: "b"},
			{ID: "other", Domain: "other.net", Match: "suffix", GroupID: "group-b", Upstream: "b"},
		},
	}
}

func TestP7CoverageStrictGroupsAndOverride(t *testing.T) {
	c := p7Fixture()
	a := BuildNativeIngressAuditP7(c, "")
	if a.Covered != 3 || a.Unmapped != 0 || a.Shadowed != 0 || !a.AllRulesReachable {
		t.Fatalf("unexpected coverage: %+v", a)
	}
	if a.SafeToApply || a.ChangesApplied || a.NDMCInvoked || a.InventoryChecked {
		t.Fatalf("unsafe automatic activation: %+v", a)
	}
	if len(a.PlannedZones) != 2 {
		t.Fatalf("nested rule should reuse parent zone: %+v", a.PlannedZones)
	}
}
func TestP7UnmappedExactAndSubdomains(t *testing.T) {
	c := p7Fixture()
	c.Rules = c.Rules[1:]
	a := BuildNativeIngressAuditP7(c, "")
	if a.Unmapped != 1 || a.Covered != 1 || a.AllRulesReachable {
		t.Fatalf("exact-only zone should be unreachable: %+v", a)
	}
	c.Rules = append(c.Rules, Rule{ID: "isolated", Domain: "lonely.org", Match: "subdomains", Upstream: "a"})
	a = BuildNativeIngressAuditP7(c, "")
	if a.Unmapped != 2 {
		t.Fatalf("subdomain-only not automatically registered: %+v", a)
	}
}
func TestP7NativeInventoryConflictReadOnly(t *testing.T) {
	c := p7Fixture()
	snapshot := `{"server":[{"address":"192.168.1.1","port":53,"domain":"api.example.com"}]}`
	a := BuildNativeIngressAuditP7(c, snapshot)
	if !a.InventoryChecked || len(a.Conflicts) != 1 || !strings.Contains(a.Conflicts[0], "api.example.com") {
		t.Fatalf("missed native overlap: %+v", a)
	}
	if a.SafeToApply {
		t.Fatal("native conflict cannot enable apply")
	}
}
func TestP7BoundedZoneLimitUnchanged(t *testing.T) {
	c := p7Fixture()
	c.Rules = nil
	for i := 0; i < 9; i++ {
		c.Rules = append(c.Rules, Rule{ID: fmt.Sprintf("r%d", i), Domain: fmt.Sprintf("z%d.example.org", i), Match: "suffix", Upstream: "a"})
	}
	a := BuildNativeIngressAuditP7(c, "")
	if len(a.PlannedZones) != 9 || a.WithinExistingTransactionLimit || a.SafeToApply || !a.AllRulesReachable {
		t.Fatalf("preview cap wrongly applied to policy matcher: %+v", a)
	}
}
func TestP7RejectInvalidConfigurationWithoutPanicking(t *testing.T) {
	c := p7Fixture()
	c.Rules[0].Upstream = "not-found"
	a := BuildNativeIngressAuditP7(c, "")
	if a.Invalid < 1 || a.SafeToApply || a.AllRulesReachable {
		t.Fatalf("invalid config passed audit: %+v", a)
	}
}
