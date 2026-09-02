package main

import "testing"

func TestClientsStableOrderIgnoresLiveRequestCounters(t *testing.T) {
	s := NewStore(32, 32)
	s.UpdateClientRegistry([]ClientInfo{
		{IP: "192.168.10.83", MAC: "02:00:00:00:00:83", Name: "Gaming-PC", Policy: "Policy2", Active: true},
		{IP: "192.168.10.20", MAC: "02:00:00:00:00:20", Name: "Workstation", Policy: "System", Active: true},
		{IP: "192.168.10.199", MAC: "02:00:00:00:00:c7", Name: "IP-Camera", Policy: "Policy1", Active: true},
	})

	// Make the live counters very different. Ordering must still be stable and
	// name-based rather than reshuffling as request counts change.
	s.mu.Lock()
	s.clientStats["02:00:00:00:00:83"].requests = 999
	s.clientStats["02:00:00:00:00:20"].requests = 1
	s.clientStats["02:00:00:00:00:c7"].requests = 50
	s.mu.Unlock()

	got := s.Clients(0)
	if len(got) != 3 {
		t.Fatalf("got %d clients, want 3", len(got))
	}
	want := []string{"Gaming-PC", "IP-Camera", "Workstation"}
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("position %d: got %q, want %q", i, got[i].Name, want[i])
		}
	}

	// Change counters again: the order should not change.
	s.mu.Lock()
	s.clientStats["02:00:00:00:00:20"].requests = 2000
	s.clientStats["02:00:00:00:00:83"].requests = 2
	s.mu.Unlock()
	got = s.Clients(0)
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("after counter change position %d: got %q, want %q", i, got[i].Name, want[i])
		}
	}
}
