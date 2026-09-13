package main

import "testing"

func TestNetworkToolsSeedAppCenterLifecycle(t *testing.T) {
	items := networkToolsSeedModules()
	if len(items) != 1 {
		t.Fatalf("expected Network Tools as the only development seed module, got %d", len(items))
	}

	item := items[0]
	if item.ID != "network-tools" {
		t.Fatalf("unexpected seed module %q", item.ID)
	}
	if item.Builtin {
		t.Fatal("Network Tools must remain optional")
	}
	if !item.Managed || !item.PackageAuthoritative {
		t.Fatal("Network Tools must be managed/package-authoritative")
	}
	if item.Trust.Status != "official" || item.Publisher.ID != "routerforge" {
		t.Fatal("Network Tools must be official RouterForge")
	}
	if len(item.Detection.Packages) != 1 || item.Detection.Packages[0] != "routerforge-network-tools" {
		t.Fatal("Network Tools package mismatch")
	}
	if item.Install.Method != "routerforge-release" || item.Update.Method != "routerforge-release" {
		t.Fatal("Network Tools release lifecycle mismatch")
	}
	if item.Remove.Method != "opkg" {
		t.Fatal("Network Tools remove lifecycle mismatch")
	}
	nav, ok := item.Presentation["navigation"].(map[string]any)
	if !ok || nav["href"] != "/network-tools" {
		t.Fatal("Network Tools navigation mismatch")
	}

	item.Release = catalogRelease{
		Channel: "dev",
		Version: "0.8.0~dev.r999.deadbeefcafe",
		Package: "routerforge-network-tools",
		Asset:   "routerforge-network-tools_0.8.0-dev.r999.deadbeefcafe_aarch64-3.10.ipk",
		SHA256:  "0000000000000000000000000000000000000000000000000000000000000000",
	}
	if !deriveCatalogActions(item).Install {
		t.Fatal("Network Tools must become installable with a trusted Dev release")
	}
}
