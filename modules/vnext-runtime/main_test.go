package main

import (
	"net"
	"testing"
)

func TestModuleConfigs(t *testing.T) {
	want := []string{"maintenance", "network-tools", "integrations", "developer-tools"}
	for _, id := range want {
		cfg, ok := moduleConfigs[id]
		if !ok || cfg.Socket == "" || cfg.UIPath == "" {
			t.Fatalf("missing module config for %s: %#v", id, cfg)
		}
	}
}

func TestRouteHexIP(t *testing.T) {
	got := routeHexIP("0001A8C0")
	if got == nil || !got.Equal(net.ParseIP("192.168.1.0")) {
		t.Fatalf("unexpected route IP: %v", got)
	}
}

func TestParseFlow(t *testing.T) {
	line := "ipv4 2 tcp 6 431999 ESTABLISHED src=192.168.1.2 dst=1.1.1.1 sport=55555 dport=443 packets=1 bytes=60"
	flow, ok := parseFlow(line)
	if !ok || flow.Protocol != "tcp" || flow.Source != "192.168.1.2" || flow.Destination != "1.1.1.1" || flow.DestPort != "443" {
		t.Fatalf("unexpected flow: %#v ok=%v", flow, ok)
	}
}
