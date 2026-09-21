package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestObservedClassifierRejectsPrivateDestinationAndIrrelevantPort(t *testing.T) {
	cfg := "TCP_PORTS=\"80,443\"\nUDP_PORTS=\"443\"\n"
	private := v2ObservedFlow{
		Protocol: "TCP", State: "SYN_SENT",
		Original:             v2ObservedFlowTuple{Source: "192.168.1.10", Destination: "192.168.1.20", DestinationPort: "443", Packets: 3},
		EffectiveDestination: "192.168.1.20",
	}
	if _, ok := v2ObservedClassify(private, cfg); ok {
		t.Fatal("private destination must be excluded")
	}
	irrelevant := private
	irrelevant.Original.Destination = "203.0.113.10"
	irrelevant.EffectiveDestination = "203.0.113.10"
	irrelevant.Original.DestinationPort = "22"
	if _, ok := v2ObservedClassify(irrelevant, cfg); ok {
		t.Fatal("port outside active policy must be excluded")
	}
}

func TestObservedClassifierFindsTCPAndUDPFailureSignals(t *testing.T) {
	cfg := "TCP_PORTS=\"80,443\"\nUDP_PORTS=\"443\"\n"
	tcp := v2ObservedFlow{
		Protocol: "TCP", State: "SYN_SENT",
		Original:             v2ObservedFlowTuple{Source: "192.168.1.10", Destination: "93.184.216.34", DestinationPort: "443", Packets: 3},
		Reply:                v2ObservedFlowTuple{Packets: 0},
		EffectiveDestination: "93.184.216.34",
	}
	item, ok := v2ObservedClassify(tcp, cfg)
	if !ok || item.Reason != "tcp_syn_no_reply" {
		t.Fatalf("tcp classification=%+v ok=%v", item, ok)
	}
	udp := v2ObservedFlow{
		Protocol: "UDP", State: "UNREPLIED", TimeoutSeconds: 10,
		Original:             v2ObservedFlowTuple{Source: "192.168.1.11", Destination: "1.1.1.1", DestinationPort: "443", Packets: 2},
		Reply:                v2ObservedFlowTuple{Packets: 0},
		EffectiveDestination: "1.1.1.1",
	}
	item, ok = v2ObservedClassify(udp, cfg)
	if !ok || item.Reason != "udp_no_reply" {
		t.Fatalf("udp classification=%+v ok=%v", item, ok)
	}
}

func TestObservedDedupRequiresDistinctOccurrences(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	cfg := "TCP_PORTS=\"443\"\n"
	flow := v2ObservedFlow{
		Protocol: "TCP", State: "SYN_SENT",
		Original:             v2ObservedFlowTuple{Source: "192.168.1.10", Destination: "93.184.216.34", DestinationPort: "443", Packets: 3},
		EffectiveDestination: "93.184.216.34",
	}
	doc := v2ObservedDefaultDocument()
	doc = v2ObservedMergeScan(doc, []v2ObservedFlow{flow}, cfg, nil, now)
	if len(doc.Entries) != 1 || doc.Entries[0].Count != 1 || !doc.Entries[0].Active {
		t.Fatalf("first scan=%+v", doc.Entries)
	}
	doc = v2ObservedMergeScan(doc, []v2ObservedFlow{flow}, cfg, nil, now.Add(time.Minute))
	if doc.Entries[0].Count != 1 {
		t.Fatalf("same active conntrack entry was counted twice: %+v", doc.Entries[0])
	}
	doc = v2ObservedMergeScan(doc, nil, cfg, nil, now.Add(2*time.Minute))
	if doc.Entries[0].Active {
		t.Fatalf("disappeared flow remained active: %+v", doc.Entries[0])
	}
	doc = v2ObservedMergeScan(doc, []v2ObservedFlow{flow}, cfg, nil, now.Add(3*time.Minute))
	if doc.Entries[0].Count != 2 {
		t.Fatalf("new occurrence was not counted: %+v", doc.Entries[0])
	}
	resp := v2ObservedResponseFor(doc, "", 0)
	if resp.Count != 1 || len(resp.Observed) != 1 {
		t.Fatalf("repeated failure was not surfaced: %+v", resp)
	}
}

func TestObservedIgnoreSuppressesWithoutDeletingEvidence(t *testing.T) {
	doc := v2ObservedDefaultDocument()
	doc.Entries = []v2ObservedTarget{{
		Key: "k", Device: "192.168.1.10", Target: "example.com", Destination: "93.184.216.34",
		Protocol: "TCP", Port: 443, Reason: "tcp_syn_no_reply", Count: 3,
		FirstSeen: "2026-09-21T10:00:00Z", LastSeen: "2026-09-21T10:05:00Z",
	}}
	doc.Ignored = []string{"example.com"}
	resp := v2ObservedResponseFor(doc, "", 0)
	if resp.Count != 0 || resp.SuppressedCount != 1 {
		t.Fatalf("ignored evidence leaked into visible candidates: %+v", resp)
	}
	if len(doc.Entries) != 1 {
		t.Fatal("ignore must not destroy evidence")
	}
}

func TestObservedStoreIsBoundedAndAtomicPathConfigurable(t *testing.T) {
	oldRoot, oldPath := v2ObservedRoot, v2ObservedPath
	t.Cleanup(func() { v2ObservedRoot, v2ObservedPath = oldRoot, oldPath })
	root := t.TempDir()
	v2ObservedRoot = root
	v2ObservedPath = filepath.Join(root, "observed-targets.json")
	doc := v2ObservedDefaultDocument()
	for i := 0; i < v2ObservedMaxEntries; i++ {
		doc.Entries = append(doc.Entries, v2ObservedTarget{
			Key: strings.Repeat("x", 4) + string(rune(i)), Destination: "93.184.216.34",
			Protocol: "TCP", Port: 443, Reason: "tcp_syn_no_reply", Count: 2,
		})
	}
	if err := writeV2ObservedDocument(doc); err != nil {
		t.Fatal(err)
	}
	loaded, err := readV2ObservedDocument()
	if err != nil || len(loaded.Entries) != v2ObservedMaxEntries {
		t.Fatalf("loaded=%d err=%v", len(loaded.Entries), err)
	}
}
