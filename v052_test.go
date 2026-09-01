package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLateResponseDoesNotDoubleCountQuality(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{{Port: 40510, Profile: "System", Name: "Xbox DNS DoH", Protocol: "DoH", TimeoutMS: 100}})
	now := time.Now()
	q := DNSMessage{ID: 22, QName: "late.example", QType: 1}
	log := NewEventLogger("/tmp/dnsmon-test-late.log")

	s.RecordQuery(now, "UDP", 40510, 50000, 22, q)
	s.CleanupTransient(now.Add(2*time.Second), log)
	s.RecordResponse(now.Add(3*time.Second), 40510, 50000, 22, DNSMessage{ID: 22, QR: true, QName: q.QName, QType: 1, RCode: 0}, log)

	snap := s.Snapshot(10, 10, 10)
	ups := snap["upstreams"].([]UpstreamView)
	w := ups[0].Stats5m
	if w.Requests != 1 || w.Responses != 0 || w.Success != 0 || w.Timeouts != 1 || w.LateResponses != 1 {
		t.Fatalf("late response distorted quality stats: %#v", w)
	}
	if ups[0].Responses != 0 || ups[0].LateResponses != 1 {
		t.Fatalf("cumulative response counters wrong: responses=%d late=%d", ups[0].Responses, ups[0].LateResponses)
	}
	if snap["total_responses"].(uint64) != 0 || snap["total_late_responses"].(uint64) != 1 {
		t.Fatalf("global response counters wrong: %#v", snap)
	}
}

func TestUnmatchedResponseDoesNotCountAsResolverResponse(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{{Port: 40502, Profile: "System", Name: "Cloudflare DoT", Protocol: "DoT"}})
	now := time.Now()
	s.RecordResponse(now, 40502, 50000, 99, DNSMessage{ID: 99, QR: true, QName: "duplicate.example", QType: 1, RCode: 0}, NewEventLogger("/tmp/dnsmon-test-unmatched.log"))
	u := s.Snapshot(10, 10, 10)["upstreams"].([]UpstreamView)[0]
	if u.Responses != 0 || u.UnmatchedResponses != 1 {
		t.Fatalf("unexpected counters: responses=%d unmatched=%d", u.Responses, u.UnmatchedResponses)
	}
}

func TestResolverWindowUsesRequestCohort(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{{Port: 40502, Profile: "System", Name: "Cloudflare DoT", Protocol: "DoT"}})

	// Build a response that crosses from the minute immediately outside a 5-minute
	// window into the first minute inside it. Resolver quality must not contain a
	// response without its originating request.
	nowMinute := time.Now().Unix() / 60
	queryAt := time.Unix((nowMinute-5)*60+59, 0)
	responseAt := queryAt.Add(1500 * time.Millisecond)
	q := DNSMessage{ID: 7, QName: "boundary.example", QType: 1}
	s.RecordQuery(queryAt, "UDP", 40502, 50000, 7, q)
	s.RecordResponse(responseAt, 40502, 50000, 7, DNSMessage{ID: 7, QR: true, QName: q.QName, QType: 1, RCode: 0}, NewEventLogger("/tmp/dnsmon-test-boundary.log"))

	s.mu.RLock()
	w := s.windowStatsLocked(40502, 5, time.Unix(nowMinute*60+30, 0))
	s.mu.RUnlock()
	if w.Requests != 0 || w.Responses != 0 || w.Success != 0 {
		t.Fatalf("window contains orphan response: %#v", w)
	}
}

func TestPendingCount(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{{Port: 40508, Profile: "System", Name: "Google DoH", Protocol: "DoH"}})
	now := time.Now()
	s.RecordQuery(now, "UDP", 40508, 50000, 3, DNSMessage{ID: 3, QName: "pending.example", QType: 1})
	u := s.Snapshot(10, 10, 10)["upstreams"].([]UpstreamView)[0]
	if u.Stats5m.Pending != 1 {
		t.Fatalf("pending=%d, want 1", u.Stats5m.Pending)
	}
}

func TestNeverRunDiagnosticOmitsLastRun(t *testing.T) {
	b, err := json.Marshal(DiagnosticView{Ran: false})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"ran":false}` {
		t.Fatalf("unexpected JSON: %s", b)
	}
}
