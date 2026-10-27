package dnsrouter

import (
	"net"
	"testing"
)

func TestDPR2KExperimentalLANRequiresExactACL(t *testing.T) {
	cfg := shadowConfig("udp")
	good := ShadowOptions{Listen: "192.168.10.1:55358", AllowLAN: true, AllowedClients: []string{"192.168.10.1", "127.0.0.1"}}
	f, err := NewShadowForwarder(cfg, good)
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"192.168.10.1", "127.0.0.1"} {
		if !f.allowSource(net.ParseIP(ip)) {
			t.Fatalf("allowed source %s rejected", ip)
		}
	}
	for _, ip := range []string{"192.168.10.2", "192.168.1.252", "203.0.113.1", "::1", "127.0.0.2"} {
		if f.allowSource(net.ParseIP(ip)) {
			t.Fatalf("foreign source %s accepted", ip)
		}
	}
	cases := []ShadowOptions{
		{Listen: "192.168.10.1:55358"},
		{Listen: "192.168.10.1:55358", AllowLAN: true},
		{Listen: "0.0.0.0:55358", AllowLAN: true, AllowedClients: []string{"192.168.10.1"}},
		{Listen: "[::]:55358", AllowLAN: true, AllowedClients: []string{"192.168.10.1"}},
		{Listen: "192.168.10.1:53", AllowLAN: true, AllowedClients: []string{"192.168.10.1"}},
		{Listen: "192.168.10.1:1024", AllowLAN: true, AllowedClients: []string{"192.168.10.1"}},
		{Listen: "192.168.10.1:55358", AllowLAN: true, AllowedClients: []string{"192.168.10.1", "192.168.10.1"}},
		{Listen: "192.168.10.1:55358", AllowLAN: true, AllowedClients: []string{"0.0.0.0"}},
		{Listen: "192.168.10.1:55358", AllowLAN: true, AllowedClients: []string{"192.168.10.0/24"}},
		{Listen: "192.168.10.1:55358", AllowLAN: true, AllowedClients: []string{"8.8.8.8"}},
		{Listen: "127.0.0.1:55353", AllowLAN: true, AllowedClients: []string{"127.0.0.1"}},
		{Listen: "127.0.0.1:55353", AllowedClients: []string{"127.0.0.1"}},
	}
	for _, opts := range cases {
		if _, err := NewShadowForwarder(cfg, opts); err == nil {
			t.Errorf("unsafe options accepted %+v", opts)
		}
	}
	if _, err := NewShadowForwarder(cfg, ShadowOptions{Listen: "127.0.0.1:55353"}); err != nil {
		t.Fatalf("legacy loopback rejected: %v", err)
	}
}
