package main

import (
	"testing"
	"time"
)

func TestRollingQuality(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{{Port: 40510, Profile: "System", Name: "Xbox DNS DoH", Protocol: "DoH", TimeoutMS: 1000}})
	now := time.Now()
	q := DNSMessage{ID: 10, QName: "example.org", QType: 1}
	s.RecordQuery(now, "UDP", 40510, 50000, 10, q)
	s.RecordResponse(now.Add(25*time.Millisecond), 40510, 50000, 10, DNSMessage{ID: 10, QR: true, QName: "example.org", QType: 1, RCode: 0}, NewEventLogger("/tmp/dnsmon-test-quality.log"))
	snap := s.Snapshot(10, 10, 10)
	ups := snap["upstreams"].([]UpstreamView)
	if len(ups) != 1 {
		t.Fatalf("upstreams=%d", len(ups))
	}
	w := ups[0].Stats5m
	if w.Requests != 1 || w.Responses != 1 || w.Success != 1 || w.QualityPct != 100 {
		t.Fatalf("bad window stats: %#v", w)
	}
	if w.P95LatencyMS < 20 || w.P95LatencyMS > 50 {
		t.Fatalf("unexpected p95: %v", w.P95LatencyMS)
	}
}

func TestTimeoutAccounting(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{{Port: 40510, Profile: "System", Name: "Xbox DNS DoH", Protocol: "DoH", TimeoutMS: 100}})
	now := time.Now()
	s.RecordQuery(now, "UDP", 40510, 50000, 22, DNSMessage{ID: 22, QName: "timeout.example", QType: 1})
	s.CleanupTransient(now.Add(2*time.Second), NewEventLogger("/tmp/dnsmon-test-timeout.log"))
	snap := s.Snapshot(10, 10, 10)
	if snap["total_timeouts"].(uint64) != 1 {
		t.Fatalf("timeouts=%v", snap["total_timeouts"])
	}
	ups := snap["upstreams"].([]UpstreamView)
	if ups[0].Stats5m.Timeouts != 1 || ups[0].Stats5m.QualityPct != 0 {
		t.Fatalf("stats=%#v", ups[0].Stats5m)
	}
}

func TestWindowFallbackEdges(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{
		{Port: 40510, Profile: "System", Name: "Xbox DNS DoH", Protocol: "DoH", ProceedMS: 500},
		{Port: 40504, Profile: "System", Name: "Comss DoT", Protocol: "DoT", ProceedMS: 500},
	})
	now := time.Now()
	d := DNSMessage{QName: "example.org", QType: 1}
	s.RecordQuery(now, "UDP", 40510, 50000, 1, d)
	s.RecordQuery(now.Add(500*time.Millisecond), "UDP", 40504, 50001, 2, d)
	edges := s.FallbackEdges(5)
	if len(edges) != 1 || edges[0].Count != 1 || edges[0].FromPort != 40510 || edges[0].ToPort != 40504 {
		t.Fatalf("edges=%#v", edges)
	}
}

func TestDiagnosticEndpoint(t *testing.T) {
	h, p, e, err := diagnosticEndpoint(UpstreamView{UpstreamMeta: UpstreamMeta{Protocol: "DoH", Target: "https://dns.google/dns-query"}})
	if err != nil || h != "dns.google" || p != "443" || e == "" {
		t.Fatalf("DoH endpoint: %q %q %q %v", h, p, e, err)
	}
	h, p, _, err = diagnosticEndpoint(UpstreamView{UpstreamMeta: UpstreamMeta{Protocol: "DoT", Target: "9.9.9.9"}})
	if err != nil || h != "9.9.9.9" || p != "853" {
		t.Fatalf("DoT endpoint: %q %q %v", h, p, err)
	}
}
