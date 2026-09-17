package main

import (
	"testing"
)

func TestParseFlowExplorerCapturesConntrackMark(t *testing.T) {
	line := "ipv4 2 tcp 6 120 ESTABLISHED src=192.168.1.2 dst=1.1.1.1 sport=50000 dport=443 src=1.1.1.1 dst=192.168.1.2 sport=443 dport=50000 mark=0xffffaab use=1"
	entry, ok := parseFlowExplorer(line)
	if !ok {
		t.Fatal("expected flow")
	}
	if entry.Mark != "0xffffaab" {
		t.Fatalf("mark=%q", entry.Mark)
	}
}

func TestMatchFlowSocketOutboundAndDNAT(t *testing.T) {
	outbound := flowExplorerEntry{
		Protocol: "TCP",
		Original: flowTuple{
			Source: "192.168.1.2", SourcePort: "50000",
			Destination: "1.1.1.1", DestinationPort: "443",
		},
	}
	outKey := flowSocketKey{
		Protocol: "TCP", LocalIP: "192.168.1.2", LocalPort: "50000",
		RemoteIP: "1.1.1.1", RemotePort: "443",
	}
	sockets := map[flowSocketKey]flowSocketRecord{
		outKey: {Inode: "123", Key: outKey},
	}
	match := matchFlowSocket(outbound, sockets)
	if match.Record.Inode != "123" || match.Match != "original-outbound-local" {
		t.Fatalf("outbound match=%+v", match)
	}

	dnat := flowExplorerEntry{
		Protocol: "TCP",
		Original: flowTuple{
			Source: "10.0.0.10", SourcePort: "53000",
			Destination: "203.0.113.20", DestinationPort: "443",
		},
		NAT: flowNATExplanation{
			Detected: true, Types: []string{"DNAT"},
			TranslatedDestination:     "192.168.10.20",
			TranslatedDestinationPort: "8443",
		},
	}
	dnatKey := flowSocketKey{
		Protocol: "TCP", LocalIP: "192.168.10.20", LocalPort: "8443",
		RemoteIP: "10.0.0.10", RemotePort: "53000",
	}
	sockets = map[flowSocketKey]flowSocketRecord{
		dnatKey: {Inode: "456", Key: dnatKey},
	}
	match = matchFlowSocket(dnat, sockets)
	if match.Record.Inode != "456" || match.Match != "dnat-inbound-local" {
		t.Fatalf("dnat match=%+v", match)
	}
}

func TestFlowPolicyMarkMatchesMask(t *testing.T) {
	if !flowPolicyMarkMatches("0xffffaab/0xffffffff", "0xffffaab") {
		t.Fatal("expected exact masked mark match")
	}
	if flowPolicyMarkMatches("0xffffaaa/0xffffffff", "0xffffaab") {
		t.Fatal("unexpected mark match")
	}
	if !flowPolicyMarkMatches("0x100/0xff00", "0x1ff") {
		t.Fatal("expected masked range match")
	}
}

func TestExplainFlowPolicyUsesKnownTupleFields(t *testing.T) {
	entry := flowExplorerEntry{
		Mark:                 "0xffffaab",
		EffectiveDestination: "8.8.8.8",
		Original: flowTuple{
			Source:      "192.168.1.2",
			Destination: "8.8.8.8",
		},
	}
	rules := []policyRule{
		{Family: "ipv4", Priority: 100, From: "192.168.1.0/24", To: "all", Table: "4098", Mark: "0xffffaab"},
		{Family: "ipv4", Priority: 200, From: "10.0.0.0/8", To: "all", Table: "100"},
		{Family: "ipv4", Priority: 32766, From: "all", To: "all", Table: "main"},
	}
	result := explainFlowPolicy(entry, rules)
	if !result.Available {
		t.Fatal("expected policy explanation")
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("candidates=%+v", result.Candidates)
	}
	if result.Candidates[0].Table != "4098" || result.Candidates[0].Priority != 100 {
		t.Fatalf("first candidate=%+v", result.Candidates[0])
	}
}

func TestFlowPolicyAddressMatches(t *testing.T) {
	if !flowPolicyAddressMatches("192.168.1.0/24", "192.168.1.50") {
		t.Fatal("CIDR should match")
	}
	if flowPolicyAddressMatches("192.168.1.0/24", "192.168.2.50") {
		t.Fatal("CIDR should not match")
	}
	if !flowPolicyAddressMatches("all", "203.0.113.1") {
		t.Fatal("all should match")
	}
}
