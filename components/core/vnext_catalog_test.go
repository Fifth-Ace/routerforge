package main

import "testing"

func TestVnextSeedModulesAppCenterLifecycle(t *testing.T) {
	expected := map[string]struct {
		pkg  string
		href string
	}{
		"maintenance":     {"routerforge-maintenance", "/api/modules/maintenance/ui/index.html"},
		"network-tools":   {"routerforge-network-tools", "/api/modules/network-tools/ui/index.html"},
		"integrations":    {"routerforge-integrations", "/api/modules/integrations/ui/index.html"},
		"developer-tools": {"routerforge-developer-tools", "/api/modules/developer-tools/ui/index.html"},
	}

	found := map[string]bool{}
	for _, item := range vnextSeedModules() {
		want, ok := expected[item.ID]
		if !ok {
			continue
		}
		found[item.ID] = true
		if item.Builtin {
			t.Fatalf("%s must remain optional", item.ID)
		}
		if !item.Managed || !item.PackageAuthoritative {
			t.Fatalf("%s must be managed/package-authoritative", item.ID)
		}
		if item.Trust.Status != "official" || item.Publisher.ID != "routerforge" {
			t.Fatalf("%s must be official RouterForge", item.ID)
		}
		if len(item.Detection.Packages) != 1 || item.Detection.Packages[0] != want.pkg {
			t.Fatalf("%s package mismatch", item.ID)
		}
		if item.Install.Method != "routerforge-release" || item.Update.Method != "routerforge-release" {
			t.Fatalf("%s release lifecycle mismatch", item.ID)
		}
		if item.Remove.Method != "opkg" {
			t.Fatalf("%s remove lifecycle mismatch", item.ID)
		}
		nav, ok := item.Presentation["navigation"].(map[string]any)
		if !ok || nav["href"] != want.href {
			t.Fatalf("%s navigation mismatch", item.ID)
		}
		item.Release = catalogRelease{
			Channel: "dev",
			Version: "0.8.0~dev.r999.deadbeefcafe",
			Package: want.pkg,
			Asset:   want.pkg + "_0.8.0-dev.r999.deadbeefcafe_aarch64-3.10.ipk",
			SHA256:  "0000000000000000000000000000000000000000000000000000000000000000",
		}
		if !deriveCatalogActions(item).Install {
			t.Fatalf("%s must become installable with trusted Dev release", item.ID)
		}
	}

	for id := range expected {
		if !found[id] {
			t.Fatalf("missing vNext seed module %s", id)
		}
	}
}
