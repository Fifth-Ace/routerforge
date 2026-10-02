package main

import "testing"

func TestP29EcosystemBatchC(t *testing.T) {
	items := auditedEcosystemIntegrations()
	tests := []struct {
		id         string
		executable bool
		preview    bool
	}{
		{"kee-geo-split", true, false},
		{"kee-smartdns-geo-conf", true, false},
		{"kee-smartdns-redirect", true, false},
		{"kee-net-check", true, false},
		{"kee-webui", true, false},
		{"trusttunnel-keenetic-native", false, true},
		{"tg-ws-keenetic", true, false},
		{"wireguard-dpi-bypass", true, false},
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

func TestKeeneticExtrasChildrenUseExplicitPackageArgs(t *testing.T) {
	for _, item := range []catalogItem{
		keeGeoSplitCatalogItem(),
		keeSmartDNSGeoConfCatalogItem(),
		keeSmartDNSRedirectCatalogItem(),
		keeNetCheckCatalogItem(),
		keeWebUICatalogItem(),
	} {
		if len(item.Install.Args) != 1 || len(item.Install.Packages) != 1 || item.Install.Args[0] != item.Install.Packages[0] {
			t.Fatalf("%s package args mismatch: args=%v packages=%v", item.ID, item.Install.Args, item.Install.Packages)
		}
	}
}

func TestTGWSKeeneticPinsEntwareExecutionMode(t *testing.T) {
	item := tgWSKeeneticCatalogItem()
	if len(item.Install.Args) != 2 || item.Install.Args[0] != "--system" || item.Install.Args[1] != "entware" {
		t.Fatalf("install args=%v", item.Install.Args)
	}
	if item.Remove.Method != "official-script" || len(item.Remove.Args) != 2 || item.Remove.Args[1] != "entware" {
		t.Fatalf("remove plan=%#v", item.Remove)
	}
}

func TestP29BatchCExecutableScriptsUseApprovedHTTPSHosts(t *testing.T) {
	for _, item := range []catalogItem{
		keeGeoSplitCatalogItem(),
		keeSmartDNSGeoConfCatalogItem(),
		keeSmartDNSRedirectCatalogItem(),
		keeNetCheckCatalogItem(),
		keeWebUICatalogItem(),
		tgWSKeeneticCatalogItem(),
		wireguardDPIBypassCatalogItem(),
	} {
		if !validOfficialScriptURL(item.Install.InstallerURL) {
			t.Fatalf("%s installer URL rejected: %s", item.ID, item.Install.InstallerURL)
		}
	}
}
