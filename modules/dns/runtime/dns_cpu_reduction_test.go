package main

import (
	"testing"
	"time"
)

func TestSyntheticHealthProbeSkipsFreshObservedTraffic(t *testing.T) {
	now := time.Unix(1000, 0)
	base := UpstreamMeta{Port: 40502, Profile: "System", Name: "Cloudflare DoT", Protocol: "DoT"}

	if shouldSyntheticHealthProbe(UpstreamView{UpstreamMeta: base, LastObserved: now.Add(-30 * time.Second)}, now) {
		t.Fatal("fresh observed traffic should suppress synthetic health probe")
	}
	if !shouldSyntheticHealthProbe(UpstreamView{UpstreamMeta: base, LastObserved: now.Add(-91 * time.Second)}, now) {
		t.Fatal("stale observed traffic should allow synthetic health probe")
	}
	if !shouldSyntheticHealthProbe(UpstreamView{UpstreamMeta: base}, now) {
		t.Fatal("resolver without observed traffic should be probed")
	}
}

func TestHealthCandidatesSkipFreshResolvers(t *testing.T) {
	now := time.Unix(2000, 0)
	s := NewStore(8, 8)
	s.UpdateDiscovery([]UpstreamMeta{
		{Port: 40502, Profile: "System", Name: "Fresh", Protocol: "DoT"},
		{Port: 40503, Profile: "System", Name: "Stale", Protocol: "DoT"},
	})
	s.mu.Lock()
	s.upstreams[40502].lastObserved = now.Add(-20 * time.Second)
	s.upstreams[40503].lastObserved = now.Add(-2 * time.Minute)
	s.mu.Unlock()

	got := s.healthCandidates(now)
	if len(got) != 1 || got[0].Port != 40503 {
		t.Fatalf("health candidates=%#v want only stale resolver", got)
	}
}

func TestTopDomainsCacheTTL(t *testing.T) {
	now := time.Unix(3000, 0)
	s := NewStore(8, 8)
	s.mu.Lock()
	s.domainCounts["a.example"] = 10
	s.domainCounts["b.example"] = 1
	s.mu.Unlock()

	top := func(at time.Time) []DomainCount {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.topDomainsLocked(1, at)
	}

	first := top(now)
	if len(first) != 1 || first[0].Domain != "a.example" {
		t.Fatalf("initial top domains=%#v", first)
	}

	s.mu.Lock()
	s.domainCounts["b.example"] = 20
	s.mu.Unlock()

	cached := top(now.Add(5 * time.Second))
	if len(cached) != 1 || cached[0].Domain != "a.example" {
		t.Fatalf("cache did not hold within ttl: %#v", cached)
	}
	fresh := top(now.Add(topDomainsCacheTTL + time.Second))
	if len(fresh) != 1 || fresh[0].Domain != "b.example" {
		t.Fatalf("cache did not refresh after ttl: %#v", fresh)
	}
}

func TestCleanupLongLivedDedup(t *testing.T) {
	now := time.Unix(4000, 0)
	s := NewStore(8, 8)
	s.errorDedup["old"] = now.Add(-11 * time.Minute)
	s.errorDedup["keep"] = now.Add(-time.Minute)
	s.clientDedup["old"] = now.Add(-11 * time.Second)
	s.clientDedup["keep"] = now.Add(-time.Second)
	s.clientResponseDedup["old"] = now.Add(-11 * time.Second)
	s.clientResponseDedup["keep"] = now.Add(-time.Second)

	s.CleanupLongLived(now)

	if len(s.errorDedup) != 1 || s.errorDedup["keep"].IsZero() {
		t.Fatalf("error dedup cleanup=%#v", s.errorDedup)
	}
	if len(s.clientDedup) != 1 || s.clientDedup["keep"].IsZero() {
		t.Fatalf("client dedup cleanup=%#v", s.clientDedup)
	}
	if len(s.clientResponseDedup) != 1 || s.clientResponseDedup["keep"].IsZero() {
		t.Fatalf("client response dedup cleanup=%#v", s.clientResponseDedup)
	}
}
