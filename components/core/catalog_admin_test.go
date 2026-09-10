package main

import "testing"

func testBundledRegistryModule(t *testing.T, id string) catalogItem {
	t.Helper()
	doc, err := parseRouterForgeRegistry(bundledRouterForgeRegistry)
	if err != nil {
		t.Fatalf("parse bundled registry: %v", err)
	}
	for i := range doc.Entries {
		if doc.Entries[i].Kind == "module" && doc.Entries[i].ID == id {
			return doc.Entries[i]
		}
	}
	t.Fatalf("bundled registry module %q missing", id)
	return catalogItem{}
}

func TestCatalogDetectsManagedAdminModule(t *testing.T) {
	installed := map[string]string{
		"routerforge-admin": "0.3.0-beta",
	}
	processes := map[string]bool{
		"routerforge-admin": true,
	}

	admin := testBundledRegistryModule(t, "admin")
	finalizeCatalogItem(&admin, installed, processes, func(string) bool { return false })

	if !admin.Managed {
		t.Fatal("admin module must be managed by RouterForge")
	}
	if !admin.Installed || admin.State != "installed" {
		t.Fatalf("bad admin install state: %#v", admin)
	}
	if !admin.ServiceRunning || !admin.Enabled {
		t.Fatalf("admin service should be running: %#v", admin)
	}
	if admin.Version != "0.3.0-beta" {
		t.Fatalf("admin version=%q", admin.Version)
	}
}

func TestBundledRegistryKeepsAdminVisibleAsStandaloneModule(t *testing.T) {
	snapshot := catalogSnapshot{Modules: builtinModuleCatalog()}
	installed := map[string]string{"routerforge-admin": "0.7.1~dev.test"}
	ensureBundledRouterForgeModules(&snapshot, installed, map[string]bool{}, func(string) bool { return false })

	admin := findCatalogItem(&snapshot, "admin", "module")
	if admin == nil {
		t.Fatal("bundled fallback did not keep admin module in catalog")
	}
	if admin.Kind != "module" || admin.Name != "RouterForge Control" {
		t.Fatalf("unexpected admin catalog item: kind=%q name=%q", admin.Kind, admin.Name)
	}
	if !admin.Installed {
		t.Fatal("routerforge-admin package was not detected as installed")
	}
}
