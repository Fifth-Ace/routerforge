package main

import (
	"net"
	"strings"
	"testing"
)

func TestValidTarget(t *testing.T) {
	for _, value := range []string{"8.8.8.8", "router.local", "example.com"} {
		if !validTarget(value) {
			t.Fatalf("expected valid target %q", value)
		}
	}
	for _, value := range []string{"", "../etc/passwd", "bad host", "-bad.example", "bad..example"} {
		if validTarget(value) {
			t.Fatalf("expected invalid target %q", value)
		}
	}
}

func TestSelectRouteLongestPrefix(t *testing.T) {
	routes := []routeEntry{
		{Destination: "0.0.0.0", Mask: "0.0.0.0", Prefix: 0, Interface: "wan"},
		{Destination: "10.0.0.0", Mask: "255.0.0.0", Prefix: 8, Interface: "vpn"},
	}
	selected := selectRoute(routes, net.ParseIP("10.2.3.4"))
	if selected == nil || selected.Interface != "vpn" {
		t.Fatalf("unexpected selected route: %#v", selected)
	}
}

func TestParseFlow(t *testing.T) {
	line := "ipv4 2 tcp 6 431999 ESTABLISHED src=192.168.1.100 dst=8.8.8.8 sport=52344 dport=443 packets=12 bytes=4096"
	flow, ok := parseFlow(line)
	if !ok {
		t.Fatal("flow did not parse")
	}
	if flow.Protocol != "TCP" || flow.Source != "192.168.1.100" || flow.Destination != "8.8.8.8" {
		t.Fatalf("unexpected flow: %#v", flow)
	}
	if flow.Bytes != 4096 || flow.Packets != 12 {
		t.Fatalf("unexpected counters: %#v", flow)
	}
}

func TestParseTraceroute(t *testing.T) {
	raw := "traceroute to 8.8.8.8\n 1  192.168.1.1  0.4 ms  0.3 ms  0.4 ms\n 2  * * *\n"
	hops := parseTraceroute(raw)
	if len(hops) != 2 {
		t.Fatalf("expected 2 hops, got %d", len(hops))
	}
	if hops[0].Address != "192.168.1.1" || len(hops[0].RTTMS) != 3 {
		t.Fatalf("unexpected first hop: %#v", hops[0])
	}
	if hops[1].LossPct != 100 {
		t.Fatalf("expected 100%% loss, got %v", hops[1].LossPct)
	}
	if !strings.Contains(hops[0].Raw, "192.168.1.1") {
		t.Fatal("raw hop lost")
	}
}
