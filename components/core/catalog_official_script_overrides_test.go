package main

import "testing"

func TestReviewedOfficialScriptLifecycleBatch(t *testing.T) {
	snapshot := catalogSnapshot{
		Integrations: []catalogItem{
			testBundledRegistryIntegration(t, "antiscan"),
			testBundledRegistryIntegration(t, "awg-manager"),
			testBundledRegistryIntegration(t, "hydraroute-neo"),
			razvilkaCatalogItem(),
		},
	}

	applyReviewedOfficialScriptLifecycle(&snapshot)

	tests := []struct {
		id     string
		url    string
		remove bool
		rmArg  string
	}{
		{"antiscan", "https://raw.githubusercontent.com/dimon27254/antiscan/refs/heads/main/install.sh", false, ""},
		{"awg-manager", "https://raw.githubusercontent.com/hoaxisr/awg-manager/develop/scripts/install.sh", false, ""},
		{"hydraroute-neo", "https://git.zerrolabs.org/Ground-Zerro/release/pages/keenetic/install-neo.sh", false, ""},
		{"razvilka", "https://raw.githubusercontent.com/ArtixSx/RAZVILKA/main/scripts/bootstrap.sh", true, "--uninstall"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			item := findCatalogItem(&snapshot, tt.id, "integration")
			if item == nil {
				t.Fatal("integration missing")
			}
			if item.Trust.Status != "verified" {
				t.Fatalf("trust=%q", item.Trust.Status)
			}
			if item.Install.Method != "official-script" || item.Install.PreviewOnly {
				t.Fatalf("install plan=%#v", item.Install)
			}
			if item.Install.InstallerURL != tt.url {
				t.Fatalf("installer=%q", item.Install.InstallerURL)
			}
			if err := validateCatalogPlan(item.Install); err != nil {
				t.Fatalf("install validation: %v", err)
			}
			if !executableCatalogPlan(item.Install) {
				t.Fatal("install plan is not executable")
			}
			if item.Update.Method != "official-script" || item.Update.PreviewOnly {
				t.Fatalf("update plan=%#v", item.Update)
			}

			if tt.remove {
				if item.Remove.Method != "official-script" || len(item.Remove.Args) != 1 || item.Remove.Args[0] != tt.rmArg {
					t.Fatalf("remove plan=%#v", item.Remove)
				}
				if err := validateCatalogPlan(item.Remove); err != nil {
					t.Fatalf("remove validation: %v", err)
				}
			} else if item.Remove.Method != "" {
				t.Fatalf("unexpected automatic remove plan=%#v", item.Remove)
			}
		})
	}
}

func TestReviewedScriptBatchEnablesInstallActions(t *testing.T) {
	snapshot := catalogSnapshot{
		Integrations: []catalogItem{
			testBundledRegistryIntegration(t, "antiscan"),
			testBundledRegistryIntegration(t, "awg-manager"),
			testBundledRegistryIntegration(t, "hydraroute-neo"),
			razvilkaCatalogItem(),
		},
	}
	applyReviewedOfficialScriptLifecycle(&snapshot)

	for _, id := range []string{"antiscan", "awg-manager", "hydraroute-neo", "razvilka"} {
		item := findCatalogItem(&snapshot, id, "integration")
		if item == nil || !item.Actions.Install {
			t.Fatalf("%s install action not enabled: %#v", id, item)
		}
	}
}
