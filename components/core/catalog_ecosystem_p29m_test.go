package main

import "testing"

func TestP29MOfficialScriptBatch(t *testing.T) {
	items := []catalogItem{
		keeneticXrayVPNCatalogItem(),
		keenManagerCatalogItem(),
		keeneticDOQCatalogItem(),
		keeneticVPNXORCatalogItem(),
	}
	for _, item := range items {
		if item.Trust.Status != "verified" {
			t.Fatalf("%s trust=%q", item.ID, item.Trust.Status)
		}
		if item.Install.Method != "official-script" {
			t.Fatalf("%s install method=%q", item.ID, item.Install.Method)
		}
		if item.Install.PreviewOnly || !executableCatalogPlan(item.Install) {
			t.Fatalf("%s install not executable: %#v", item.ID, item.Install)
		}
		if err := validateCatalogPlan(item.Install); err != nil {
			t.Fatalf("%s install invalid: %v", item.ID, err)
		}
		if item.Update.PreviewOnly || !executableCatalogPlan(item.Update) {
			t.Fatalf("%s update not executable: %#v", item.ID, item.Update)
		}
		if err := validateCatalogPlan(item.Update); err != nil {
			t.Fatalf("%s update invalid: %v", item.ID, err)
		}
		if item.Remove.PreviewOnly || !executableCatalogPlan(item.Remove) {
			t.Fatalf("%s remove not executable: %#v", item.ID, item.Remove)
		}
		if err := validateCatalogPlan(item.Remove); err != nil {
			t.Fatalf("%s remove invalid: %v", item.ID, err)
		}
	}
}

func TestP29MInstallerURLsArePinnedOrChecksummed(t *testing.T) {
	xray := keeneticXrayVPNCatalogItem()
	if xray.Install.ExpectedSHA256 == "" {
		t.Fatal("keenetic-xray-vpn release installer must be SHA256 pinned")
	}

	for _, item := range []catalogItem{
		keenManagerCatalogItem(),
		keeneticDOQCatalogItem(),
		keeneticVPNXORCatalogItem(),
	} {
		if !validOfficialScriptURL(item.Install.InstallerURL) {
			t.Fatalf("%s installer URL rejected: %s", item.ID, item.Install.InstallerURL)
		}
		if item.Install.InstallerURL == "" {
			t.Fatalf("%s installer URL missing", item.ID)
		}
	}
}

func TestP29MNoInteractivePreviewAuthority(t *testing.T) {
	for _, id := range []string{
		"keenetic-xray-vpn",
		"keen-manager",
		"keenetic-doq",
		"keenetic-vpn-xor",
	} {
		found := false
		for _, item := range auditedEcosystemIntegrations() {
			if item.ID == id {
				found = true
				if !deriveCatalogActions(item).Install {
					t.Fatalf("%s install action missing", id)
				}
				break
			}
		}
		if !found {
			t.Fatalf("%s missing from audited ecosystem", id)
		}
	}
}
