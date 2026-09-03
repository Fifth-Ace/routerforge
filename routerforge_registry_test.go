package main

import "testing"

func TestBundledRouterForgeRegistry(t *testing.T) {
	doc, err := parseRouterForgeRegistry(bundledRouterForgeRegistry)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Brand != "RouterForge" || doc.RegistryID != "routerforge-community" {
		t.Fatalf("unexpected registry identity: %#v", doc)
	}
	var admin, dns, web *catalogItem
	for i := range doc.Entries {
		switch doc.Entries[i].ID {
		case "admin":
			admin = &doc.Entries[i]
		case "dns":
			dns = &doc.Entries[i]
		case "nfqws-web":
			web = &doc.Entries[i]
		}
	}
	if admin == nil || admin.Trust.Status != "official" || admin.Install.Method != "routerforge-release" {
		t.Fatalf("bad official admin entry: %#v", admin)
	}
	if dns == nil || dns.Trust.Status != "official" || len(dns.Install.Packages) != 1 || dns.Install.Packages[0] != "routerforge-dns" {
		t.Fatalf("bad RouterForge DNS entry: %#v", dns)
	}
	if web == nil || web.Trust.Status != "verified" || web.Install.Method != "structured" {
		t.Fatalf("bad verified integration entry: %#v", web)
	}
}

func TestRegistryRejectsRawShellLifecycle(t *testing.T) {
	plan := catalogInstallPlan{
		Method: "structured",
		Steps:  []catalogLifecycleStep{{Type: "shell"}},
	}
	if err := validateCatalogPlan(plan); err == nil {
		t.Fatal("raw shell lifecycle must be rejected")
	}
}
