package events

import (
	"testing"
	"time"
)

func TestQueryNewestFirstAndFilters(t *testing.T) {
	base := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)
	ring := NewRing(8)
	ring.Add(Event{Time: base, Component: "core", Severity: Info, Type: "core.ready", Message: "ready"})
	ring.Add(Event{Time: base.Add(time.Minute), Component: "dns", Severity: Warning, Type: "dns.timeout", ObjectID: "policy1", Message: "timeout"})
	ring.Add(Event{Time: base.Add(2 * time.Minute), Component: "dns", Severity: Critical, Type: "dns.down", ObjectID: "policy1", Message: "down"})
	ring.Add(Event{Time: base.Add(3 * time.Minute), Component: "dns", Severity: Info, Type: "dns.recovered", ObjectID: "policy1", Recovery: true, Message: "recovered"})

	got := ring.Query(Query{
		Component: "DNS",
		Severity:  []Severity{Warning, Critical},
		ObjectID:  "policy1",
		Limit:     2,
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d: %#v", len(got), got)
	}
	if got[0].Type != "dns.down" || got[1].Type != "dns.timeout" {
		t.Fatalf("expected newest-first filtered events, got %#v", got)
	}
}

func TestQueryRecoveryAndTimeWindow(t *testing.T) {
	base := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)
	ring := NewRing(4)
	ring.Add(Event{Time: base, Component: "service", Severity: Critical, Recovery: false, Message: "down"})
	ring.Add(Event{Time: base.Add(time.Minute), Component: "service", Severity: Info, Recovery: true, Message: "up"})

	recovery := true
	got := ring.Query(Query{
		Recovery: &recovery,
		Since:    base.Add(30 * time.Second),
		Until:    base.Add(90 * time.Second),
	})
	if len(got) != 1 || got[0].Message != "up" {
		t.Fatalf("unexpected recovery query: %#v", got)
	}
}

func TestRingLenAndCapacity(t *testing.T) {
	ring := NewRing(2)
	if ring.Capacity() != 2 || ring.Len() != 0 {
		t.Fatalf("unexpected empty ring metadata capacity=%d len=%d", ring.Capacity(), ring.Len())
	}
	ring.Add(Event{Message: "one"})
	ring.Add(Event{Message: "two"})
	ring.Add(Event{Message: "three"})
	if ring.Capacity() != 2 || ring.Len() != 2 {
		t.Fatalf("unexpected bounded ring metadata capacity=%d len=%d", ring.Capacity(), ring.Len())
	}
}
