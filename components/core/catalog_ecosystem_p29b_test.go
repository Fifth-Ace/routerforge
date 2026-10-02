package main

import "testing"

func TestP29EcosystemBatchB(t *testing.T) {
	items := auditedEcosystemIntegrations()
	tests := []struct {
		id         string
		executable bool
		preview    bool
	}{
		{"susanin-keenetic", true, false},
		{"trusttunnel-keenetic", false, true},
		{"tg-ws-proxy-go", false, true},
		{"tg-ws-proxy-rs", true, false},
		{"aiway-manager", true, false},
		{"bird4static", false, true},
		{"ipset4static", false, true},
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

func TestSusaninUsesNonInteractiveInstallMode(t *testing.T) {
	item := susaninCatalogItem()
	if len(item.Install.Args) != 1 || item.Install.Args[0] != "--yes" {
		t.Fatalf("Susanin install args=%v", item.Install.Args)
	}
}

func TestP29BatchBExecutableScriptsUseApprovedHTTPSHosts(t *testing.T) {
	for _, item := range []catalogItem{susaninCatalogItem(), tgWSProxyRSCatalogItem(), aiwayManagerCatalogItem()} {
		if !validOfficialScriptURL(item.Install.InstallerURL) {
			t.Fatalf("%s installer URL rejected: %s", item.ID, item.Install.InstallerURL)
		}
	}
}
