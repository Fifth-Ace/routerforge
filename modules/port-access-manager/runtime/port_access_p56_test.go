package main

import "testing"

func TestP56MultiplePortsAndTrust(t *testing.T) {
	s := odinSettings{WAN: "eth3", IP: "192.168.1.2", Port: 3389, Ports: []int{3389, 8443, 22}, Trusted: []string{"198.51.100.8/32", "203.0.113.0/24"}, Knock: [3]int{7777, 8888, 6666}, Window: 15, TTL: 28800}
	if err := s.valid(); err != nil {
		t.Fatal(err)
	}
	ports, err := s.protectedPorts()
	if err != nil || len(ports) != 3 {
		t.Fatalf("ports: %v %v", ports, err)
	}
	legacy := s
	legacy.Ports = nil
	legacy.Trusted = nil
	if err := legacy.valid(); err != nil {
		t.Fatalf("legacy TPORT: %v", err)
	}
	bad := s
	bad.Ports = []int{3389, 3389}
	if bad.valid() == nil {
		t.Fatal("duplicate service port accepted")
	}
	bad = s
	bad.Ports = []int{3389, 7777}
	if bad.valid() == nil {
		t.Fatal("knock/service overlap accepted")
	}
	bad = s
	bad.Trusted = []string{"0.0.0.0/0"}
	if bad.valid() == nil {
		t.Fatal("public wildcard trusted")
	}
	bad = s
	bad.Trusted = []string{"203.0.113.5/24"}
	if bad.valid() == nil {
		t.Fatal("noncanonical trusted CIDR accepted")
	}
}
