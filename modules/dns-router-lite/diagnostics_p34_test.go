package lite

import (
	"testing"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

func TestP34HistoryIsolationAndAlternateSelection(t *testing.T) {
	h := NewUpstreamHealthTracker()
	now := time.Date(2026, 11, 15, 12, 1, 0, 0, time.UTC)
	h.mu.Lock()
	a := h.entryLocked("up-a")
	b := h.entryLocked("up-b")
	p34Record(a, Trace{Assigned: "up-a", Used: "up-a", Success: true, LatencyMS: 20}, now, now)
	p34Record(a, Trace{Assigned: "up-a", Used: "up-a", Success: false}, now, now)
	p34Record(b, Trace{Assigned: "up-a", Used: "up-b", Success: true, LatencyMS: 60}, now, now)
	h.mu.Unlock()
	views := h.DiagnosticsP34([]dr.Upstream{{ID: "up-a"}, {ID: "up-b"}}, now)
	if len(views) != 2 {
		t.Fatalf("count=%d", len(views))
	}
	if v := views[0].LastFourBuckets; v.Requests != 2 || v.Errors != 1 || v.Alternate != 0 || v.AvgRTTMS != 20 {
		t.Fatalf("up-a %+v", v)
	}
	if v := views[1].LastFourBuckets; v.Requests != 1 || v.Errors != 0 || v.Alternate != 1 || v.AvgRTTMS != 60 {
		t.Fatalf("up-b %+v", v)
	}
}

func TestP34HistoryExpiresAndReusesRing(t *testing.T) {
	h := NewUpstreamHealthTracker()
	now := time.Date(2026, 11, 15, 12, 1, 0, 0, time.UTC)
	h.mu.Lock()
	e := h.entryLocked("up")
	p34Record(e, Trace{Success: true, LatencyMS: 9}, now.Add(-25*time.Hour), now)
	p34Record(e, Trace{Success: true, LatencyMS: 11}, now.Add(-23*time.Hour), now)
	p34Record(e, Trace{Success: true, LatencyMS: 13}, now, now)
	h.mu.Unlock()
	old := h.DiagnosticsP34([]dr.Upstream{{ID: "up"}}, now)
	if old[0].Last96Buckets.Requests != 2 {
		t.Fatalf("unexpected old count %+v", old[0].Last96Buckets)
	}
	newTime := now.Add(24 * time.Hour)
	next := h.DiagnosticsP34([]dr.Upstream{{ID: "up"}}, newTime)
	if next[0].Last96Buckets.Requests != 0 {
		t.Fatalf("old buckets leaked %+v", next[0])
	}
}

func TestP34NoFutureOrOutOfWindowTelemetry(t *testing.T) {
	e := &upstreamHealthEntry{}
	now := time.Date(2026, 11, 15, 12, 1, 0, 0, time.UTC)
	p34Record(e, Trace{Success: true}, now.Add(2*time.Minute), now)
	p34Record(e, Trace{Success: true}, now.Add(-25*time.Hour), now)
	for _, b := range e.p34History {
		if b.requests != 0 {
			t.Fatal("invalid timestamp accepted")
		}
	}
}
