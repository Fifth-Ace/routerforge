package main

import (
	"net"
	"testing"
)

func TestParseFlowExplorerWithoutNAT(t *testing.T) {
	line := "ipv4 2 tcp 6 431999 ESTABLISHED src=192.168.1.10 dst=1.1.1.1 sport=50000 dport=443 packets=10 bytes=1000 src=1.1.1.1 dst=192.168.1.10 sport=443 dport=50000 packets=8 bytes=800 [ASSURED] mark=0 use=1"
	entry, ok := parseFlowExplorer(line)
	if !ok {
		t.Fatal("expected flow")
	}
	if entry.Protocol != "TCP" || entry.State != "ESTABLISHED" {
		t.Fatalf("protocol/state=%s/%s", entry.Protocol, entry.State)
	}
	if entry.NAT.Detected {
		t.Fatalf("unexpected NAT: %+v", entry.NAT)
	}
	if entry.EffectiveDestination != "1.1.1.1" {
		t.Fatalf("effective destination=%q", entry.EffectiveDestination)
	}
}

func TestParseFlowExplorerDNAT(t *testing.T) {
	line := "ipv4 2 tcp 6 120 ESTABLISHED src=10.0.0.10 dst=203.0.113.20 sport=53000 dport=443 packets=4 bytes=400 src=192.168.10.20 dst=10.0.0.10 sport=8443 dport=53000 packets=3 bytes=300 mark=0 use=1"
	entry, ok := parseFlowExplorer(line)
	if !ok {
		t.Fatal("expected flow")
	}
	if !entry.NAT.Detected || entry.NAT.TranslatedDestination != "192.168.10.20" {
		t.Fatalf("DNAT=%+v", entry.NAT)
	}
	if entry.NAT.TranslatedDestinationPort != "8443" {
		t.Fatalf("translated dport=%q", entry.NAT.TranslatedDestinationPort)
	}
	if entry.EffectiveDestination != "192.168.10.20" {
		t.Fatalf("effective destination=%q", entry.EffectiveDestination)
	}
}

func TestParseFlowExplorerSNAT(t *testing.T) {
	line := "ipv4 2 udp 17 30 src=192.168.1.50 dst=8.8.8.8 sport=55555 dport=53 packets=2 bytes=120 src=8.8.8.8 dst=198.51.100.5 sport=53 dport=40000 packets=2 bytes=160 mark=0 use=1"
	entry, ok := parseFlowExplorer(line)
	if !ok {
		t.Fatal("expected flow")
	}
	if !entry.NAT.Detected || entry.NAT.TranslatedSource != "198.51.100.5" {
		t.Fatalf("SNAT=%+v", entry.NAT)
	}
	if entry.NAT.TranslatedSourcePort != "40000" {
		t.Fatalf("translated sport=%q", entry.NAT.TranslatedSourcePort)
	}
}

func TestFlowExplorerFiltersOriginalAndTranslatedTuples(t *testing.T) {
	line := "ipv4 2 tcp 6 120 ESTABLISHED src=10.0.0.10 dst=203.0.113.20 sport=53000 dport=443 src=192.168.10.20 dst=10.0.0.10 sport=8443 dport=53000 mark=0 use=1"
	entry, ok := parseFlowExplorer(line)
	if !ok {
		t.Fatal("expected flow")
	}
	if !flowExplorerMatches(entry, flowExplorerFilter{Protocol: "TCP", Destination: "192.168.10.20"}) {
		t.Fatal("translated destination filter should match")
	}
	if flowExplorerMatches(entry, flowExplorerFilter{Source: "192.0.2.99"}) {
		t.Fatal("unrelated source should not match")
	}
}

func TestExplainFlowRouteUsesEffectiveDestination(t *testing.T) {
	entry := flowExplorerEntry{EffectiveDestination: "192.168.10.20"}
	routes := []routeEntry{
		{
			Interface:   "eth0",
			Destination: "0.0.0.0",
			Gateway:     "192.168.1.1",
			Mask:        "0.0.0.0",
			Prefix:      0,
			Metric:      100,
			Table:       "main",
		},
		{
			Interface:   "br0",
			Destination: "192.168.10.0",
			Gateway:     "0.0.0.0",
			Mask:        "255.255.255.0",
			Prefix:      24,
			Metric:      0,
			Table:       "main",
		},
	}
	result := explainFlowRoute(entry, routes)
	if !result.Available || result.Interface != "br0" || result.Prefix != 24 {
		t.Fatalf("route=%+v", result)
	}
	if net.ParseIP(result.Target) == nil {
		t.Fatalf("target=%q", result.Target)
	}
}
