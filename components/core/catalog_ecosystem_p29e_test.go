package main

import "testing"

func TestP29EcosystemBatchE(t *testing.T) {
	items := auditedEcosystemIntegrations()
	tests := []struct {
		id         string
		executable bool
		preview    bool
	}{
		{"b4", true, false},
		{"hydra-bridge", true, false},
		{"ssclash-go", true, false},
		{"broray", false, true},
		{"keenetic-auto-setup", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			var item *catalogItem
			for i := range items {
				if items[i].ID == tt.id {
					item = &items[i]
					break
				}
			}
			if item == nil {
				t.Fatal("catalog item missing")
			}
			if item.Trust.Status != "verified" {
				t.Fatalf("trust=%q", item.Trust.Status)
			}
			if item.Install.PreviewOnly != tt.preview {
				t.Fatalf("preview=%v want=%v", item.Install.PreviewOnly, tt.preview)
			}
			if got := executableCatalogPlan(item.Install); got != tt.executable {
				t.Fatalf("executable=%v want=%v plan=%#v", got, tt.executable, item.Install)
			}
			if tt.executable {
				if err := validateCatalogPlan(item.Install); err != nil {
					t.Fatalf("install plan invalid: %v", err)
				}
			}
		})
	}
}

func TestHydraBridgeApprovedHost(t *testing.T) {
	item := hydraBridgeCatalogItem()
	if !validOfficialScriptURL(item.Install.InstallerURL) {
		t.Fatalf("HydraBridge installer rejected: %s", item.Install.InstallerURL)
	}
}

func TestBROrayRemainsDigestPinnedPreview(t *testing.T) {
	item := broRayCatalogItem()
	if !item.Install.PreviewOnly || item.Install.Method != "official-script" {
		t.Fatalf("unexpected BROray install plan=%#v", item.Install)
	}
	if executableCatalogPlan(item.Install) {
		t.Fatal("BROray preview unexpectedly executable")
	}
}
