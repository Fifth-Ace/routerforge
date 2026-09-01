package main

import (
	"testing"
	"time"
)

func TestQualityWarnsAtFivePercentFallback(t *testing.T) {
	w := WindowStats{Requests: 20, Responses: 20, Success: 20, QualityPct: 100, Fallbacks: 1, FallbackPct: 5}
	if got := qualityStatus(w); got != "WARN" {
		t.Fatalf("qualityStatus=%q, want WARN", got)
	}
}

func TestHistoryDoesNotFabricatePreStartZeroes(t *testing.T) {
	s := NewStore(8, 8)
	nowMinute := time.Now().Unix() / 60
	s.started = time.Unix((nowMinute-2)*60, 0)
	got := s.History(60)
	if len(got) != 3 {
		t.Fatalf("History(60) len=%d, want 3 points since process start", len(got))
	}
}

func TestLongHistoryIsAggregated(t *testing.T) {
	s := NewStore(8, 8)
	nowMinute := time.Now().Unix() / 60
	s.started = time.Unix((nowMinute-1439)*60, 0)
	got := s.History(1440)
	if len(got) > 97 {
		t.Fatalf("History(1440) len=%d, expected roughly <=96 aggregated points", len(got))
	}
}
