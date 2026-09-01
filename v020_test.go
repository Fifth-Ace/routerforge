package main

import (
	"testing"
	"time"
)

func TestFallbackMatrix(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{
		{Port: 40510, Profile: "System", Name: "Xbox DNS DoH", Protocol: "DoH", ProceedMS: 500},
		{Port: 40504, Profile: "System", Name: "Yandex DoT", Protocol: "DoT", ProceedMS: 500},
	})
	d := DNSMessage{QName: "example.org", QType: 1}
	t0 := time.Unix(100, 0)
	s.RecordQuery(t0, "UDP", 40510, 50000, 1, d)
	s.RecordQuery(t0.Add(500*time.Millisecond), "UDP", 40504, 50001, 2, d)
	snap := s.Snapshot(10, 10, 10)
	edges := snap["fallback_edges"].([]FallbackEdge)
	if len(edges) != 1 || edges[0].FromPort != 40510 || edges[0].ToPort != 40504 || edges[0].Count != 1 {
		t.Fatalf("bad fallback edges: %#v", edges)
	}
}

func TestHistoryBucket(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{{Port: 40510, Profile: "System", Name: "Xbox DNS DoH", Protocol: "DoH"}})
	now := time.Now()
	s.RecordQuery(now, "UDP", 40510, 50000, 1, DNSMessage{QName: "example.org", QType: 1})
	h := s.History(1)
	if len(h) != 1 || h[0].Requests != 1 {
		t.Fatalf("bad history: %#v", h)
	}
}
