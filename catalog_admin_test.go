package main

import "testing"

func TestCatalogDetectsManagedAdminModule(t *testing.T) {
	installed := map[string]string{
		"routerforge-admin": "0.3.0-beta",
	}
	processes := map[string]bool{
		"routerforge-admin": true,
	}

	snap := buildCatalog(installed, processes, func(string) bool { return false })

	var admin *catalogItem
	for i := range snap.Modules {
		if snap.Modules[i].ID == "admin" {
			admin = &snap.Modules[i]
			break
		}
	}
	if admin == nil {
		t.Fatal("admin module missing")
	}
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
