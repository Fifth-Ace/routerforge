package main

import "testing"

func TestP29NPreviewManualMegaBatch(t *testing.T) {
	ids := map[string]bool{
		"usque-keenetic":           true,
		"keenetic-singbox":         true,
		"adguardhome-keenetic":     true,
		"keenetic-cloudflared":     true,
		"netbird-keenetic":         true,
		"keen2ygg":                 true,
		"keenetic-zapret2-manager": true,
		"keenetic-aria2-manager":   true,
		"keenetic-firewall":        true,
		"keenetic-traffic-via-vpn": true,
	}
	seen := map[string]bool{}
	for _, item := range auditedEcosystemIntegrations() {
		if !ids[item.ID] {
			continue
		}
		seen[item.ID] = true
		if item.Trust.Status != "verified" {
			t.Fatalf("%s trust=%q", item.ID, item.Trust.Status)
		}
		if item.Install.Method != "manual" || !item.Install.PreviewOnly {
			t.Fatalf("%s unexpectedly gained install authority: %#v", item.ID, item.Install)
		}
		if deriveCatalogActions(item).Install {
			t.Fatalf("%s install action must stay disabled", item.ID)
		}
	}
	for id := range ids {
		if !seen[id] {
			t.Fatalf("%s missing from ecosystem batch", id)
		}
	}
}
