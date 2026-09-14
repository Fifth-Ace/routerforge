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

func TestSelectDefaultRoutePrefersMetric(t *testing.T) {
	routes := []routeEntry{
		{
			Destination: "0.0.0.0",
			Mask:        "0.0.0.0",
			Prefix:      0,
			Metric:      50,
			Interface:   "backup",
			Gateway:     "192.168.2.1",
		},
		{
			Destination: "0.0.0.0",
			Mask:        "0.0.0.0",
			Prefix:      0,
			Metric:      10,
			Interface:   "wan",
			Gateway:     "192.168.1.1",
		},
	}

	selected := selectDefaultRoute(routes)
	if selected == nil {
		t.Fatal("expected default route")
	}
	if selected.Interface != "wan" || selected.Gateway != "192.168.1.1" {
		t.Fatalf("unexpected default route: %#v", selected)
	}
}

func TestDoctorVerdictPriority(t *testing.T) {
	healthy := doctorVerdictFor([]doctorStage{
		{ID: "default_route", Status: "ok"},
		{ID: "interface", Status: "ok"},
		{ID: "internet", Status: "ok"},
		{ID: "dns", Status: "ok"},
	})
	if healthy.Code != "healthy" || healthy.Severity != "ok" {
		t.Fatalf("unexpected healthy verdict: %#v", healthy)
	}

	dnsFailure := doctorVerdictFor([]doctorStage{
		{ID: "default_route", Status: "ok"},
		{ID: "interface", Status: "ok"},
		{ID: "internet", Status: "ok"},
		{ID: "dns", Status: "fail"},
	})
	if dnsFailure.Code != "dns_failure" || dnsFailure.FaultDomain != "dns" {
		t.Fatalf("unexpected DNS verdict: %#v", dnsFailure)
	}

	routeFailure := doctorVerdictFor([]doctorStage{
		{ID: "default_route", Status: "fail"},
		{ID: "dns", Status: "fail"},
	})
	if routeFailure.Code != "no_default_route" {
		t.Fatalf("route failure must win priority: %#v", routeFailure)
	}

	degraded := doctorVerdictFor([]doctorStage{
		{ID: "default_route", Status: "ok"},
		{ID: "gateway", Status: "warn"},
		{ID: "internet", Status: "ok"},
	})
	if degraded.Code != "degraded" || degraded.Severity != "warn" {
		t.Fatalf("unexpected degraded verdict: %#v", degraded)
	}
}

func TestPrimaryInterfaceAddress(t *testing.T) {
	got := primaryInterfaceAddress([]string{
		"fe80::1/64",
		"192.168.10.1/24",
	})
	if got != "192.168.10.1/24" {
		t.Fatalf("unexpected primary address: %q", got)
	}
}