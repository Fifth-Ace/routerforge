package main

import (
	"errors"
	"testing"
	"time"
)

func TestDNSAuxiliaryCacheSuccessTTL(t *testing.T) {
	now := time.Unix(1000, 0)
	calls := 0

	c := newDNSDiscoveryCollector(5 * time.Minute)
	c.now = func() time.Time { return now }
	c.run = func(string, time.Duration) (string, error) {
		calls++
		if calls == 1 {
			return "first", nil
		}
		return "second", nil
	}

	if got := c.auxiliaryText("show ip policy", time.Second, false); got != "first" {
		t.Fatalf("first value=%q, want first", got)
	}
	now = now.Add(4*time.Minute + 59*time.Second)
	if got := c.auxiliaryText("show ip policy", time.Second, false); got != "first" {
		t.Fatalf("cached value=%q, want first", got)
	}
	if calls != 1 {
		t.Fatalf("calls=%d before TTL, want 1", calls)
	}

	now = now.Add(time.Second)
	if got := c.auxiliaryText("show ip policy", time.Second, false); got != "second" {
		t.Fatalf("refreshed value=%q, want second", got)
	}
	if calls != 2 {
		t.Fatalf("calls=%d after TTL, want 2", calls)
	}
}

func TestDNSAuxiliaryCacheForceRefresh(t *testing.T) {
	now := time.Unix(2000, 0)
	calls := 0

	c := newDNSDiscoveryCollector(5 * time.Minute)
	c.now = func() time.Time { return now }
	c.run = func(string, time.Duration) (string, error) {
		calls++
		if calls == 1 {
			return "first", nil
		}
		return "forced", nil
	}

	if got := c.auxiliaryText("show interface", time.Second, false); got != "first" {
		t.Fatalf("first value=%q", got)
	}
	now = now.Add(time.Minute)
	if got := c.auxiliaryText("show interface", time.Second, true); got != "forced" {
		t.Fatalf("forced value=%q, want forced", got)
	}
	if calls != 2 {
		t.Fatalf("calls=%d, want 2", calls)
	}
}

func TestDNSAuxiliaryCachePreservesLastGoodOnFailure(t *testing.T) {
	now := time.Unix(3000, 0)
	calls := 0

	c := newDNSDiscoveryCollector(5 * time.Minute)
	c.now = func() time.Time { return now }
	c.run = func(string, time.Duration) (string, error) {
		calls++
		switch calls {
		case 1:
			return "last-good", nil
		case 2:
			return "", errors.New("temporary failure")
		default:
			return "recovered", nil
		}
	}

	if got := c.auxiliaryText("show ip name-server", time.Second, false); got != "last-good" {
		t.Fatalf("first value=%q", got)
	}

	now = now.Add(5 * time.Minute)
	if got := c.auxiliaryText("show ip name-server", time.Second, false); got != "last-good" {
		t.Fatalf("failed refresh lost last-good value: %q", got)
	}
	if calls != 2 {
		t.Fatalf("calls=%d after failed refresh, want 2", calls)
	}

	now = now.Add(4*time.Minute + 59*time.Second)
	if got := c.auxiliaryText("show ip name-server", time.Second, false); got != "last-good" {
		t.Fatalf("value before retry=%q", got)
	}
	if calls != 2 {
		t.Fatalf("calls=%d before failure backoff expires, want 2", calls)
	}

	now = now.Add(time.Second)
	if got := c.auxiliaryText("show ip name-server", time.Second, false); got != "recovered" {
		t.Fatalf("recovery value=%q, want recovered", got)
	}
	if calls != 3 {
		t.Fatalf("calls=%d after recovery, want 3", calls)
	}
}

func TestDNSAuxiliaryCacheColdFailureRetriesAfterMinute(t *testing.T) {
	now := time.Unix(4000, 0)
	calls := 0

	c := newDNSDiscoveryCollector(5 * time.Minute)
	c.now = func() time.Time { return now }
	c.run = func(string, time.Duration) (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("cold failure")
		}
		return "recovered", nil
	}

	if got := c.auxiliaryText("show interface", time.Second, false); got != "" {
		t.Fatalf("cold failure value=%q, want empty", got)
	}

	now = now.Add(59 * time.Second)
	if got := c.auxiliaryText("show interface", time.Second, false); got != "" {
		t.Fatalf("value before cold retry=%q, want empty", got)
	}
	if calls != 1 {
		t.Fatalf("calls=%d before cold retry, want 1", calls)
	}

	now = now.Add(time.Second)
	if got := c.auxiliaryText("show interface", time.Second, false); got != "recovered" {
		t.Fatalf("cold recovery value=%q, want recovered", got)
	}
	if calls != 2 {
		t.Fatalf("calls=%d after cold retry, want 2", calls)
	}
}

func TestDNSDiscoveryPrimaryChangeUsesSnapshotCopy(t *testing.T) {
	c := newDNSDiscoveryCollector(5 * time.Minute)
	ups := []UpstreamMeta{{Port: 40500}}

	if !c.primaryChanged(ups, nil) {
		t.Fatal("first primary snapshot must be treated as changed")
	}

	ups[0].PolicyMark = 123
	fresh := []UpstreamMeta{{Port: 40500}}
	if c.primaryChanged(fresh, nil) {
		t.Fatal("caller mutation leaked into cached primary snapshot")
	}

	changed := []UpstreamMeta{{Port: 40501}}
	if !c.primaryChanged(changed, nil) {
		t.Fatal("primary upstream change must force auxiliary refresh")
	}
}
