package main

import (
	"testing"
	"time"
)

func TestPollBackoffGrowsAndCaps(t *testing.T) {
	b := newPollBackoff(time.Minute, 5*time.Minute)
	want := []time.Duration{2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute}
	for i, expected := range want {
		if got := b.next(false); got != expected {
			t.Fatalf("failure %d delay=%s want=%s", i+1, got, expected)
		}
	}
}

func TestPollBackoffSuccessResetsToBase(t *testing.T) {
	b := newPollBackoff(30*time.Second, 5*time.Minute)
	if got := b.next(false); got != time.Minute {
		t.Fatalf("first failure delay=%s want=1m", got)
	}
	if got := b.next(false); got != 2*time.Minute {
		t.Fatalf("second failure delay=%s want=2m", got)
	}
	if got := b.next(true); got != 30*time.Second {
		t.Fatalf("success delay=%s want=30s", got)
	}
	if got := b.next(false); got != time.Minute {
		t.Fatalf("failure after reset delay=%s want=1m", got)
	}
}

func TestPollBackoffNormalizesBounds(t *testing.T) {
	b := newPollBackoff(0, 0)
	if b.base != time.Second || b.max != time.Second || b.current != time.Second {
		t.Fatalf("normalized backoff=%+v", b)
	}
	if got := b.next(false); got != time.Second {
		t.Fatalf("capped normalized delay=%s want=1s", got)
	}
}
