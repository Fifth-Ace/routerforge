package main

import (
	"testing"
	"time"
)

func TestP95NeverExceedsObservedMax(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{{Port: 40502, Profile: "System", Name: "Cloudflare DoT", Protocol: "DoT"}})
	now := time.Now()
	q := DNSMessage{ID: 1, QName: "one.example", QType: 1}
	s.RecordQuery(now, "UDP", 40502, 50000, 1, q)
	s.RecordResponse(now.Add(27*time.Millisecond), 40502, 50000, 1, DNSMessage{ID: 1, QR: true, QName: q.QName, QType: 1, RCode: 0}, NewEventLogger("/tmp/dnsmon-test-p95.log"))
	u := s.Snapshot(1, 1, 1)["upstreams"].([]UpstreamView)[0]
	if u.Stats5m.P95LatencyMS > u.Stats5m.MaxLatencyMS {
		t.Fatalf("p95=%v max=%v", u.Stats5m.P95LatencyMS, u.Stats5m.MaxLatencyMS)
	}
}

func TestFallbackRequiresSourceStillPending(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{
		{Port: 40508, Profile: "System", Name: "Google DoH", Protocol: "DoH", ProceedMS: 500},
		{Port: 40502, Profile: "System", Name: "Cloudflare DoT", Protocol: "DoT", ProceedMS: 500},
	})
	now := time.Now()
	q := DNSMessage{ID: 1, QName: "repeat.example", QType: 1}
	s.RecordQuery(now, "UDP", 40508, 50000, 1, q)
	s.RecordResponse(now.Add(40*time.Millisecond), 40508, 50000, 1, DNSMessage{ID: 1, QR: true, QName: q.QName, QType: 1, RCode: 0}, NewEventLogger("/tmp/dnsmon-test-fallback-pending.log"))
	s.RecordQuery(now.Add(500*time.Millisecond), "UDP", 40502, 50001, 2, DNSMessage{ID: 2, QName: q.QName, QType: 1})
	if edges := s.FallbackEdges(5); len(edges) != 0 {
		t.Fatalf("unexpected fallback after source already answered: %#v", edges)
	}
}

func TestHealthCauseLocalPath(t *testing.T) {
	st := &upstreamState{healthStatus: "DOWN", diagnostic: DiagnosticView{Ran: true, Status: "OK", Stage: "DNS"}}
	if got := healthCause(st); got != "LOCAL_PATH" {
		t.Fatalf("healthCause=%q", got)
	}
}
