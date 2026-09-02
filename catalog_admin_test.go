package main

import "testing"

func TestCatalogDetectsManagedAdminModule(t *testing.T) {
	installed := map[string]string{
		"dns-monitor-admin": "0.2.0-dev",
	}
	processes := map[string]bool{
		"dnsmon-admin": true,
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
		t.Fatal("admin module must be managed by DNS Monitor")
	}
	if !admin.Installed || admin.State != "installed" {
		t.Fatalf("bad admin install state: %#v", admin)
	}
	if !admin.ServiceRunning || !admin.Enabled {
		t.Fatalf("admin service should be running: %#v", admin)
	}
	if admin.Version != "0.2.0-dev" {
		t.Fatalf("admin version=%q", admin.Version)
	}
}
