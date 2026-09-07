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
	if admin.ProjectURL != "https://github.com/Fifth-Ace/routerforge" ||
		admin.Publisher.URL != "https://github.com/Fifth-Ace/routerforge" ||
		admin.Install.RepositoryURL != "https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta" {
		t.Fatalf("official RouterForge URLs were not canonicalized: %#v", admin)
	}
	if dns == nil || dns.Trust.Status != "official" || len(dns.Install.Packages) != 1 || dns.Install.Packages[0] != "routerforge-dns" {
		t.Fatalf("bad RouterForge DNS entry: %#v", dns)
	}
	if web == nil || web.Trust.Status != "verified" || web.Install.Method != "structured" {
		t.Fatalf("bad verified integration entry: %#v", web)
	}
	if web.Web == nil || web.Web.Mode != "external-only" || web.Web.Embed {
		t.Fatalf("bad nfqws-web launch contract: %#v", web.Web)
	}
}

func TestCatalogWebMetadataContract(t *testing.T) {
	tests := []struct {
		name    string
		meta    catalogWebMetadata
		wantErr bool
	}{
		{
			name:    "missing mode",
			meta:    catalogWebMetadata{Port: 90},
			wantErr: true,
		},
		{
			name: "external only",
			meta: catalogWebMetadata{Port: 90, Mode: "external-only"},
		},
		{
			name: "embedded supported but disabled",
			meta: catalogWebMetadata{Port: 90, Mode: "embedded-supported"},
		},
		{
			name: "embedded enabled",
			meta: catalogWebMetadata{Port: 90, Mode: "embedded-supported", Embed: true},
		},
		{
			name:    "unknown mode",
			meta:    catalogWebMetadata{Port: 90, Mode: "iframe-maybe"},
			wantErr: true,
		},
		{
			name:    "external only cannot embed",
			meta:    catalogWebMetadata{Port: 90, Mode: "external-only", Embed: true},
			wantErr: true,
		},
		{
			name:    "unsupported version cannot embed",
			meta:    catalogWebMetadata{Port: 90, Mode: "unsupported-version", Embed: true},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCatalogWebMetadata(&tt.meta)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
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

func TestUnverifiedIntegrationUsesDirectConfiguredOpkgFallback(t *testing.T) {
	item := awgManagerIntegration()
	item.Trust = catalogTrust{Status: "unverified"}
	item.Installed = true
	item.PackageInstalled = true
	item.Version = "2.17.2"
	item.AvailableVersion = "2.17.3"
	item.UpdateAvailable = true

	actions := deriveCatalogActions(item)
	if actions.Install || !actions.Update || !actions.Remove {
		t.Fatalf("unexpected unverified actions: %#v", actions)
	}
	if actions.Reason == "" {
		t.Fatal("unverified integration should carry a warning reason")
	}

	plan := catalogPlanForAction(item, "update")
	if plan.Method != "opkg" || len(plan.Packages) != 1 || plan.Packages[0] != "awg-manager" {
		t.Fatalf("unexpected fallback update plan: %#v", plan)
	}
}

func TestBlockedIntegrationCannotUseDirectOpkgFallback(t *testing.T) {
	item := awgManagerIntegration()
	item.Trust = catalogTrust{Status: "blocked"}
	item.Installed = true
	item.PackageInstalled = true
	item.AvailableVersion = "2.17.3"
	item.UpdateAvailable = true

	actions := deriveCatalogActions(item)
	if actions.Install || actions.Update || actions.Remove {
		t.Fatalf("blocked integration received actions: %#v", actions)
	}
}

func TestVerifiedPackageIntegrationUpdateRequiresRealUpgrade(t *testing.T) {
	item := nfqws2Integration()
	item.Trust = catalogTrust{Status: "verified"}
	item.Installed = true
	item.Update = catalogInstallPlan{
		Method:   "opkg",
		Packages: []string{"nfqws2-keenetic"},
	}

	if actions := deriveCatalogActions(item); actions.Update {
		t.Fatalf("update was enabled without opkg upgrade state: %#v", actions)
	}

	item.UpdateAvailable = true
	if actions := deriveCatalogActions(item); !actions.Update {
		t.Fatalf("update stayed disabled with opkg upgrade state: %#v", actions)
	}
}
