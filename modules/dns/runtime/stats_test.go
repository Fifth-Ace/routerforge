package main

import (
	"testing"
	"time"
)

func TestFallbackDetection(t *testing.T) {
	s := NewStore(32, 8)
	s.UpdateDiscovery([]UpstreamMeta{
		{Port: 40510, Profile: "System", Protocol: "DoH", Name: "Xbox DNS DoH", ProceedMS: 500},
		{Port: 40504, Profile: "System", Protocol: "DoT", Name: "Yandex DoT", ProceedMS: 500},
	})
	d := DNSMessage{ID: 1, QName: "example.org", QType: 1}
	t0 := time.Unix(100, 0)
	s.RecordQuery(t0, "UDP", 40510, 50000, 1, d)
	s.RecordQuery(t0.Add(500*time.Millisecond), "UDP", 40504, 50001, 2, d)
	snap := s.Snapshot(10, 10, 10)
	if snap["total_fallbacks"].(uint64) != 1 {
		t.Fatalf("fallbacks=%v", snap["total_fallbacks"])
	}
	flow := snap["flow"].([]FlowEvent)
	if len(flow) != 2 || !flow[1].Fallback {
		t.Fatalf("flow=%#v", flow)
	}
}
