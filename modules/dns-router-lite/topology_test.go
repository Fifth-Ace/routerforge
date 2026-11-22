package lite

import (
	"context"
	"strings"
	"testing"
)

func dstestTopology() []TopologyInterface {
	const ll = "fe80::52ff:20ff:fed3:831/64"
	return []TopologyInterface{
		{Name: "br0", Up: true, Bridge: true, Lower: []string{"ra0", "eth2.1"}, Addresses: []string{"192.168.10.1/24", ll}},
		{Name: "eth2", Up: true, Addresses: []string{ll}},
		{Name: "ra0", Up: true, Master: "br0", Addresses: []string{ll}},
		{Name: "eth2.1", Up: true, Master: "br0", Lower: []string{"eth2"}, Addresses: []string{ll}},
	}
}
func TestP191DSTestScopedLinkLocalAndBridgeRelationships(t *testing.T) {
	topology := dstestTopology()
	topology = append(topology, TopologyInterface{Name: "independent", Up: true, Addresses: []string{"fe80::52ff:20ff:fed3:831/64"}})
	got, err := AnalyzeGateTopology(topology)
	if err != nil {
		t.Fatal("legitimate scoped duplicate rejected", err)
	}
	if got.IPv4 != "192.168.10.1/24" || len(got.IPv6) != 0 || len(got.LinkLocal) != 5 {
		t.Fatalf("wrong family separation: %+v", got)
	}
	for _, a := range got.LinkLocal {
		if a.BridgeRelated != (a.Interface != "independent") {
			t.Fatal("bridge/underlying scope lost", a)
		}
	}
}
func TestP191ActualIndependentRoutedOverlapStillRejected(t *testing.T) {
	for _, which := range []string{"ipv4", "ipv6_ula", "ipv6_global", "member_routed"} {
		t.Run(which, func(t *testing.T) {
			topology := dstestTopology()
			extra := TopologyInterface{Name: "vpn0", Up: true}
			switch which {
			case "ipv4":
				extra.Addresses = []string{"192.168.10.254/25"}
			case "member_routed":
				topology[2].Addresses = append(topology[2].Addresses, "192.168.10.254/24")
			case "ipv6_ula":
				topology[0].Addresses = append(topology[0].Addresses, "fd12:3456::1/64")
				extra.Addresses = []string{"fd12:3456::2/80"}
			case "ipv6_global":
				topology[0].Addresses = append(topology[0].Addresses, "2001:db8:123::1/64")
				extra.Addresses = []string{"2001:db8:123::2/48"}
			}
			topology = append(topology, extra)
			if _, err := AnalyzeGateTopology(topology); err == nil || !strings.Contains(err.Error(), "overlaps") {
				t.Fatal("unsafe routed overlap accepted", err)
			}
		})
	}
}
func TestP191IPv4OnlyGateAndExplicitIPv6Coverage(t *testing.T) {
	topology, err := AnalyzeGateTopology(dstestTopology())
	if err != nil {
		t.Fatal(err)
	}
	g, m, _ := gateFixture(t)
	g.candidates = func() (string, []string, error) { return topology.IPv4, topology.IPv6, nil }
	g.locate = func(name string) (string, error) {
		if name == "ip6tables" {
			t.Fatal("IPv4 activation required new IPv6 readiness")
		}
		return "/injected", nil
	}
	g.probe = func(_ context.Context, _ string, p GatePlan) error {
		if len(p.Families) != 1 || p.Families[0].Family != "inet" {
			t.Fatal("link-local included in activation", p)
		}
		return nil
	}
	if err = g.Enable(context.Background(), "192.168.10.1:55358"); err != nil {
		t.Fatal(err)
	}
	c := g.Status()["coverage"].(map[string]string)
	if c["ipv4"] != "active_last_verified" || c["ipv6_routed"] != "excluded_no_managed_routed_prefix" || c["ipv6_link_local"] != "excluded_unproven_interface_scope" || c["hardware_e2e"] != "PENDING" {
		t.Fatal("false IPv6 coverage", c)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mutation := range m.mutations {
		if strings.HasPrefix(mutation, "ip6tables") || strings.Contains(mutation, gateSet6) {
			t.Fatal("IPv6 unexpectedly activated", mutation)
		}
	}
}
func TestP191RoutedDualStackExcludesOnlyScopedSources(t *testing.T) {
	topology := dstestTopology()
	topology[0].Addresses = append(topology[0].Addresses, "fd12:3456::1/64", "2001:db8:123::1/64")
	got, err := AnalyzeGateTopology(topology)
	if err != nil {
		t.Fatal(err)
	}
	p, err := planFromCandidates("192.168.10.1:55358", got.IPv4, got.IPv6, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Families) != 2 || len(p.Families[1].Prefixes) != 2 {
		t.Fatal("routed IPv6 was lost", p)
	}
	for _, prefix := range p.Families[1].Prefixes {
		if strings.HasPrefix(prefix, "fe80:") {
			t.Fatal("link-local source in active set")
		}
	}
	if c := gateCoverage(p, true); c["ipv6_routed"] != "active_last_verified" || c["ipv6_link_local"] != "excluded_unproven_interface_scope" {
		t.Fatal(c)
	}
}
