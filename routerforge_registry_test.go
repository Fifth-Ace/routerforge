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
	var admin, web *catalogItem
	for i := range doc.Entries {
		switch doc.Entries[i].ID {
		case "admin":
			admin = &doc.Entries[i]
		case "nfqws-web":
			web = &doc.Entries[i]
		}
	}
	if admin == nil || admin.Trust.Status != "official" || admin.Install.Method != "routerforge-release" {
		t.Fatalf("bad official module entry: %#v", admin)
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
