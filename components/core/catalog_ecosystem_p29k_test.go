package main

import "testing"

func TestVerifiedIPKTargetResolution(t *testing.T) {
	plan := catalogInstallPlan{
		Method:   "verified-ipk-target",
		Packages: []string{"demo"},
		VerifiedIPKTargets: map[string]catalogVerifiedIPKAsset{
			"aarch64-3.10": {
				InstallerURL:   "https://github.com/example/demo/releases/download/v1/demo.ipk",
				ExpectedSHA256: "0000000000000000000000000000000000000000000000000000000000000000",
			},
		},
	}
	resolved, err := resolveVerifiedIPKPlan(plan, "aarch64-3.10")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Method != "verified-ipk" || resolved.InstallerURL == "" {
		t.Fatalf("resolved=%#v", resolved)
	}
	if _, err := resolveVerifiedIPKPlan(plan, "mips-3.4"); err == nil {
		t.Fatal("unsupported target accepted")
	}
}

func TestP29VerifiedIPKBatch(t *testing.T) {
	items := []catalogItem{
		entwareManagerCatalogItem(),
		wayHopCatalogItem(),
		qWDTTKeeneticCatalogItem(),
		detourKeeneticCatalogItem(),
		xkeenSmartRouteCatalogItem(),
	}
	for _, item := range items {
		if item.Trust.Status != "verified" {
			t.Fatalf("%s trust=%q", item.ID, item.Trust.Status)
		}
		if item.Install.PreviewOnly || !executableCatalogPlan(item.Install) {
			t.Fatalf("%s install not executable: %#v", item.ID, item.Install)
		}
		if err := validateCatalogPlan(item.Install); err != nil {
			t.Fatalf("%s install invalid: %v", item.ID, err)
		}
	}
}

func TestP29LegacyXrayAutoIsPreviewOnly(t *testing.T) {
	item := keeneticXrayAutoCatalogItem()
	if item.Trust.Status != "verified" {
		t.Fatalf("%s trust=%q", item.ID, item.Trust.Status)
	}
	if item.Install.Method != "manual" || !item.Install.PreviewOnly {
		t.Fatalf("%s legacy install authority mismatch: %#v", item.ID, item.Install)
	}
	if executableCatalogPlan(item.Install) {
		t.Fatalf("%s legacy install unexpectedly executable: %#v", item.ID, item.Install)
	}
}

func TestTargetAwareVerifiedIPKBatchHasAssets(t *testing.T) {
	for _, item := range []catalogItem{
		entwareManagerCatalogItem(),
		wayHopCatalogItem(),
		qWDTTKeeneticCatalogItem(),
	} {
		if item.Install.Method != "verified-ipk-target" {
			t.Fatalf("%s method=%q", item.ID, item.Install.Method)
		}
		if len(item.Install.VerifiedIPKTargets) < 3 {
			t.Fatalf("%s targets=%v", item.ID, item.Install.VerifiedIPKTargets)
		}
	}
}
