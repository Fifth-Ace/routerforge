package main

import (
	"testing"
	"time"
)

func TestParseIPPolicies(t *testing.T) {
	got := parseIPPolicies(`
policy, name = Policy0, description = test:
      mark: ffffaaa
    table4: 4096
policy, name = Policy2, description = vpn:
      mark: ffffaac
    table4: 4100
`)
	if got["Policy0"].Mark != 0x0ffffaaa || got["Policy0"].Table != 4096 {
		t.Fatalf("Policy0 parse: %#v", got["Policy0"])
	}
	if got["Policy2"].Mark != 0x0ffffaac || got["Policy2"].Table != 4100 {
		t.Fatalf("Policy2 parse: %#v", got["Policy2"])
	}
}

func TestParseHotspotClient(t *testing.T) {
	got := parseHotspot(`
             host:
                  mac: 02:00:00:00:00:83
                   ip: 192.168.10.83
             hostname: DESKTOP-LDTB0SG
                 name: Gaming-PC
            interface:
                       id: Bridge0
                     name: Home
               policy: Policy2
               active: yes
                  mws:
                       ap: WifiMaster1/AccessPoint0
                 ssid: ExampleWiFi_5G
`)
	if len(got) != 1 {
		t.Fatalf("clients=%d", len(got))
	}
	c := got[0]
	if c.Name != "Gaming-PC" || c.Policy != "Policy2" || c.Network != "Home" || c.SSID != "ExampleWiFi_5G" || c.AP != "WifiMaster1/AccessPoint0" {
		t.Fatalf("unexpected client: %#v", c)
	}
	if c.Access != "Wi-Fi · Mesh · ExampleWiFi_5G" {
		t.Fatalf("access=%q", c.Access)
	}
}

func TestClientAttributionToResolverFlow(t *testing.T) {
	s := NewStore(16, 16)
	s.UpdateDiscovery([]UpstreamMeta{{Port: 40551, Profile: "Policy2", Name: "Google DoT", Protocol: "DoT"}})
	s.UpdateClientRegistry([]ClientInfo{{IP: "192.168.10.83", MAC: "02:00:00:00:00:83", Name: "Gaming-PC", Policy: "Policy2", Access: "Ethernet · port 1", Active: true}})
	now := time.Now()
	d := DNSMessage{ID: 77, QName: "catalog.gamepass.com", QType: 1}
	s.RecordClientQuery(now, "UDP", "192.168.10.83", "02:00:00:00:00:83", d.ID, d)
	s.RecordQuery(now.Add(10*time.Millisecond), "UDP", 40551, 50123, d.ID, d)
	snap := s.Snapshot(10, 10, 10)
	flow := snap["flow"].([]FlowEvent)
	if len(flow) != 1 || flow[0].ClientName != "Gaming-PC" || flow[0].ClientIP != "192.168.10.83" {
		t.Fatalf("flow attribution failed: %#v", flow)
	}
	clients := s.Clients(10)
	if len(clients) != 1 || clients[0].Matched != 1 || clients[0].Requests != 1 {
		t.Fatalf("client stats failed: %#v", clients)
	}
}

func TestHistoryCoverage(t *testing.T) {
	s := NewStore(8, 8)
	s.started = time.Now().Add(-30 * time.Minute)
	c := s.HistoryCoverage(60)
	if c < 0.49 || c > 0.51 {
		t.Fatalf("coverage=%f", c)
	}
}
