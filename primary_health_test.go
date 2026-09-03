package main

import (
	"testing"
	"time"
)

func TestSnapshotPrimaryHealthIgnoresPolicyContext(t *testing.T) {
	s := NewStore(8, 8)
	s.UpdateDiscovery([]UpstreamMeta{
		{Port: 40500, Profile: "System", Protocol: "DoT", Name: "Google"},
		{Port: 40516, Profile: "Policy0", Protocol: "DoT", Name: "Google", PolicyMark: 0xffffaaa},
	})

	s.mu.Lock()
	s.upstreams[40500].healthStatus = "UP"
	s.upstreams[40516].healthStatus = "DOWN"
	s.upstreams[40516].lastRequest = time.Now()
	s.mu.Unlock()

	snap := s.Snapshot(0, 0, 0)
	if got := snap["active_down"].(int); got != 1 {
		t.Fatalf("legacy active_down=%d", got)
	}
	if got := snap["primary_active_down"].(int); got != 0 {
		t.Fatalf("primary_active_down=%d", got)
	}
	if got := snap["primary_upstream_count"].(int); got != 1 {
		t.Fatalf("primary_upstream_count=%d", got)
	}
}
