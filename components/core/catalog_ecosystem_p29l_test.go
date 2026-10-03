package main

import "testing"

func TestP29LVerifiedIPKBatch(t *testing.T) {
	items := []catalogItem{
		nfqwsKeeneticCatalogItem(),
		nfqws2KeeneticCatalogItem(),
		nfqwsKeeneticWebCatalogItem(),
		keeneticPolicyUICatalogItem(),
		web4staticCatalogItem(),
		keenPBRHeadlessCatalogItem(),
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
		if item.Update.PreviewOnly || !executableCatalogPlan(item.Update) {
			t.Fatalf("%s update not executable: %#v", item.ID, item.Update)
		}
	}
}

func TestP29LTargetAwareAssets(t *testing.T) {
	for _, item := range []catalogItem{
		nfqwsKeeneticCatalogItem(),
		nfqws2KeeneticCatalogItem(),
		keenPBRHeadlessCatalogItem(),
	} {
		if item.Install.Method != "verified-ipk-target" {
			t.Fatalf("%s method=%q", item.ID, item.Install.Method)
		}
		if len(item.Install.VerifiedIPKTargets) < 3 {
			t.Fatalf("%s targets=%v", item.ID, item.Install.VerifiedIPKTargets)
		}
	}
	if got := len(keenPBRHeadlessCatalogItem().Install.VerifiedIPKTargets); got != 5 {
		t.Fatalf("keen-pbr-headless targets=%d", got)
	}
}

func TestP29LSingleAssetPins(t *testing.T) {
	for _, item := range []catalogItem{
		nfqwsKeeneticWebCatalogItem(),
		keeneticPolicyUICatalogItem(),
		web4staticCatalogItem(),
	} {
		if item.Install.Method != "verified-ipk" {
			t.Fatalf("%s method=%q", item.ID, item.Install.Method)
		}
		if item.Install.ExpectedSHA256 == "" || item.Install.InstallerURL == "" {
			t.Fatalf("%s missing immutable asset pin", item.ID)
		}
	}
}
