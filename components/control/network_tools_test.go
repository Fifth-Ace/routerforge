package main

import "testing"

func TestValidNetworkTarget(t *testing.T) {
	valid := []string{"example.com", "router.lan", "192.168.1.1", "2001:db8::1"}
	for _, target := range valid {
		if !validNetworkTarget(target) {
			t.Fatalf("expected valid target %q", target)
		}
	}
	invalid := []string{"", "../etc/passwd", "example.com;reboot", "a b", "-bad.example", "bad..example"}
	for _, target := range invalid {
		if validNetworkTarget(target) {
			t.Fatalf("expected invalid target %q", target)
		}
	}
}
