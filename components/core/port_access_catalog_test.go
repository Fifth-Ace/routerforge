package main

import "testing"

func TestPortAccessK2AReadOnlyCatalog(t *testing.T) {
	items := portAccessSeedModules()
	if len(items) != 1 {
		t.Fatalf("expected one module, got %d", len(items))
	}
	item := items[0]
	if item.ID != "port-access-manager" || item.Kind != "module" {
		t.Fatalf("wrong module: %+v", item)
	}
	if item.Install.Method != "" || item.Update.Method != "" || item.Remove.Method != "" {
		t.Fatal("K2A must not advertise package mutation")
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
