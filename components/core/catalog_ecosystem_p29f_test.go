package main

import "testing"

func TestP29CatalogReviewBatchF(t *testing.T) {
	requireAuditedEcosystemIDs(t,
		"qeli-keenetic",
		"xkeen-ui-umarcheh",
	)
}

func TestQeliKeeneticDetectionContract(t *testing.T) {
	item := qeliKeeneticCatalogItem()
	if len(item.Detection.Services) != 1 || item.Detection.Services[0] != "/opt/etc/init.d/S99qeli" {
		t.Fatalf("services=%v", item.Detection.Services)
	}
	if item.Install.Method != "manual" || !item.Install.PreviewOnly {
		t.Fatalf("install=%#v", item.Install)
	}
}

func TestUmarchehXKeenUIRequiresReleaseBundle(t *testing.T) {
	item := xkeenUIUmarchehCatalogItem()
	if item.Install.Method != "manual" || !item.Install.PreviewOnly {
		t.Fatalf("install=%#v", item.Install)
	}
	if len(item.Detection.Services) != 2 {
		t.Fatalf("services=%v", item.Detection.Services)
	}
}
