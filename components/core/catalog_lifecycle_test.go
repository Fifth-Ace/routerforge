package main

import "testing"

func TestBuiltinCatalogContainsCoreOnly(t *testing.T) {
	snap := buildCatalog(map[string]string{}, map[string]bool{}, func(string) bool { return false })
	if len(snap.Modules) != 1 || snap.Modules[0].ID != "routerforge-core" {
		t.Fatalf("builtin catalog must contain Core only: %#v", snap.Modules)
	}
}

func TestCatalogManagedMonitoringModuleFromRegistry(t *testing.T) {
	installed := map[string]string{
		"routerforge-monitoring": "0.7.0-dev.test",
	}
	processes := map[string]bool{
		"routerforge-monitoring": true,
	}

	item := testBundledRegistryModule(t, "monitoring")
	finalizeCatalogItem(&item, installed, processes, func(string) bool { return false })

	if !item.Managed || !item.Installed || !item.Enabled || !item.ServiceRunning || item.State != "installed" {
		t.Fatalf("bad consolidated monitoring state: %#v", item)
	}
	if item.Version != "0.7.0-dev.test" {
		t.Fatalf("monitoring version=%q", item.Version)
	}
}

func TestCatalogProfilingUsesMarkerAsRunningState(t *testing.T) {
	item := testBundledRegistryModule(t, "profiling")
	exists := func(path string) bool {
		return path == profilingMarker
	}
	finalizeCatalogItem(
		&item,
		map[string]string{"routerforge-profiling": "0.7.0-dev.test"},
		map[string]bool{},
		exists,
	)
	if !item.Installed || !item.ServiceRunning || !item.Enabled || item.State != "installed" {
		t.Fatalf("bad profiling state: %#v", item)
	}
}

func TestCatalogDNSUsesEnabledMarkerAsRunningState(t *testing.T) {
	item := testBundledRegistryModule(t, "dns")
	const marker = "/opt/etc/routerforge/dns.enabled"
	exists := func(path string) bool {
		return path == marker
	}
	finalizeCatalogItem(
		&item,
		map[string]string{"routerforge-dns": "0.7.0-dev.test"},
		map[string]bool{},
		exists,
	)
	if !item.Installed || !item.ServiceRunning || !item.Enabled || item.State != "installed" {
		t.Fatalf("bad DNS marker state: %#v", item)
	}
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
