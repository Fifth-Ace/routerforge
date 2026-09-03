package main

import "testing"

func TestCatalogManagedMonitoringModules(t *testing.T) {
	installed := map[string]string{
		"dns-monitor-system":  "0.2.0-dev",
		"dns-monitor-thermal": "0.2.0-dev",
		"dns-monitor-storage": "0.2.0-dev",
		"dns-monitor-network": "0.2.0-dev",
	}
	processes := map[string]bool{
		"dnsmon-system": true, "dnsmon-thermal": true,
		"dnsmon-storage": true, "dnsmon-network": true,
	}
	snap := buildCatalog(installed, processes, func(string) bool { return false })

	want := map[string]bool{"system": true, "thermal": true, "storage": true, "network": true}
	for i := range snap.Modules {
		item := snap.Modules[i]
		if !want[item.ID] {
			continue
		}
		delete(want, item.ID)
		if !item.Managed || !item.Installed || !item.Enabled || !item.ServiceRunning || item.State != "installed" {
			t.Fatalf("bad managed module state for %s: %#v", item.ID, item)
		}
	}
	if len(want) != 0 {
		t.Fatalf("managed modules missing: %#v", want)
	}
}

func TestCatalogProfilingUsesMarkerAsRunningState(t *testing.T) {
	exists := func(path string) bool {
		return path == profilingMarker
	}
	snap := buildCatalog(
		map[string]string{"dns-monitor-profiling": "0.2.0-dev"},
		map[string]bool{},
		exists,
	)
	for i := range snap.Modules {
		if snap.Modules[i].ID != "profiling" {
			continue
		}
		item := snap.Modules[i]
		if !item.Installed || !item.ServiceRunning || !item.Enabled || item.State != "installed" {
			t.Fatalf("bad profiling state: %#v", item)
		}
		return
	}
	t.Fatal("profiling module missing")
}

func TestCombatMarketplaceHasCuratedIntegrations(t *testing.T) {
	snap := buildCatalog(map[string]string{}, map[string]bool{}, func(string) bool { return false })
	want := map[string]bool{
		"awg-manager": true, "nfqws": true, "nfqws2": true, "nfqws-web": true, "hydraroute-neo": true,
		"xkeen": true, "xkeen-ui": true, "keen-pbr": true, "kvas": true, "bypass-keenetic": true,
		"traffic-via-vpn": true, "adguardhome-keenetic": true, "skeen": true, "chur-keenetic": true,
		"keenetic-sing-box-ui":    true,
		"keenetic-entware-extras": true,
	}
	for _, item := range snap.Integrations {
		delete(want, item.ID)
		if !item.Install.PreviewOnly {
			t.Fatalf("%s is not preview-only", item.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("marketplace integrations missing: %#v", want)
	}
}
