package main

import "testing"

func TestContainsAnyToken(t *testing.T) {
	if !containsAnyToken("/opt/bin/awg-manager --config /opt/etc/awg-manager", []string{"awg-manager"}) {
		t.Fatal("expected AWG Manager token match")
	}
	if !containsAnyToken("AdGuardHome --no-check-update", []string{"adguardhome"}) {
		t.Fatal("expected AdGuard Home token match")
	}
	if containsAnyToken("routerforge-admin", []string{"nfqws2", "adguardhome"}) {
		t.Fatal("unexpected token match")
	}
}

func TestDetectAdminIntegrationFromRuntimeSignals(t *testing.T) {
	definition := adminIntegrationDefinition{
		ID:            "nfqws2",
		Name:          "nfqws2",
		ProcessTokens: []string{"nfqws2"},
		ServiceTokens: []string{"nfqws2"},
	}
	processes := []processInfo{{PID: 101, Name: "nfqws2", Command: "/opt/usr/bin/nfqws2"}}
	services := []serviceInfo{{ID: "S51nfqws2", Name: "S51nfqws2", Running: true}}
	ports := []portInfo{{LocalPort: 8888, Process: "nfqws2"}}

	info := detectAdminIntegration(definition, processes, services, ports)
	if !info.Detected || !info.Running {
		t.Fatalf("unexpected detection: %+v", info)
	}
	if info.ServiceID != "S51nfqws2" {
		t.Fatalf("service_id=%q", info.ServiceID)
	}
	if len(info.ProcessPIDs) != 1 || info.ProcessPIDs[0] != 101 {
		t.Fatalf("pids=%v", info.ProcessPIDs)
	}
	if len(info.Ports) != 1 || info.Ports[0] != 8888 {
		t.Fatalf("ports=%v", info.Ports)
	}
}
