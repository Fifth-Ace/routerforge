package main

import "testing"

func TestPortAccessK2BInstallableReadOnlyCatalog(t *testing.T) {
	items := portAccessSeedModules()
	if len(items) != 1 {
		t.Fatalf("expected one module, got %d", len(items))
	}
	item := items[0]
	if item.ID != "port-access-manager" || item.Kind != "module" {
		t.Fatalf("wrong module: %+v", item)
	}
	if item.Install.Method != "routerforge-release" || item.Install.Repository != "routerforge-dev" ||
		item.Update.Method != "routerforge-release" || item.Remove.Method != "opkg" {
		t.Fatal("K2B must expose manager package lifecycle")
	}
	if len(item.Install.Packages) != 1 || item.Install.Packages[0] != "routerforge-port-access-manager" {
		t.Fatal("K2B must install only the RouterForge manager")
	}
	for _, name := range append(append([]string{}, item.Install.Packages...), item.Update.Packages...) {
		if name == "knockd" || name == "fwknopd" {
			t.Fatal("K2B must not install authorization engines")
		}
	}
	found := false
	for _, candidate := range buildCatalog(map[string]string{}, map[string]bool{}, func(string) bool { return false }).Modules {
		if candidate.ID == item.ID {
			found = true
			if candidate.Installed {
				t.Fatal("preview should not claim installed")
			}
			break
		}
	}
	if !found {
		t.Fatal("Port Access manager missing from catalog")
	}
}
