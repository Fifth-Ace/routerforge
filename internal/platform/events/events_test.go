package events

import "testing"

func TestRingBounded(t *testing.T) {
	r := NewRing(2)
	r.Add(Event{Message: "one"})
	r.Add(Event{Message: "two"})
	r.Add(Event{Message: "three"})
	got := r.Snapshot(10)
	if len(got) != 2 || got[0].Message != "two" || got[1].Message != "three" {
		t.Fatalf("unexpected ring snapshot: %#v", got)
	}
}
