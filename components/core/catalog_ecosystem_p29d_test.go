package main

import "testing"

func TestP29EcosystemBatchD(t *testing.T) {
	items := auditedEcosystemIntegrations()
	tests := []struct {
		id         string
		executable bool
		preview    bool
	}{
		{"magitrickle-badigit", true, false},
		{"magitrickle-larin", true, false},
		{"xkeen-ui-fan92", true, false},
		{"dropweb-xkeen", false, true},
		{"wdtt-server-entware", false, true},
		{"netcraze-giga-awg3", false, true},
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

func TestP29BatchDExecutableScriptsUseApprovedHTTPSHosts(t *testing.T) {
	for _, item := range []catalogItem{
		magiTrickleBadigitCatalogItem(),
		magiTrickleLarinCatalogItem(),
		xkeenUIFan92CatalogItem(),
	} {
		if !validOfficialScriptURL(item.Install.InstallerURL) {
			t.Fatalf("%s installer URL rejected: %s", item.ID, item.Install.InstallerURL)
		}
	}
}

func TestNetcrazeAWG3RemainsHardwareSpecificPreview(t *testing.T) {
	item := netcrazeAWG3CatalogItem()
	if !item.Install.PreviewOnly || item.Install.Method != "manual" {
		t.Fatalf("unexpected install plan=%#v", item.Install)
	}
	if len(item.Compatibility.Targets) != 1 || item.Compatibility.Targets[0] != "aarch64-3.10" {
		t.Fatalf("unexpected targets=%v", item.Compatibility.Targets)
	}
}
