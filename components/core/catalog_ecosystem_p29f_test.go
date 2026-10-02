package main

import "testing"

func TestP29CatalogReviewBatchF(t *testing.T) {
	items := auditedEcosystemIntegrations()
	for _, id := range []string{"qeli-keenetic", "xkeen-ui-umarcheh"} {
		found := false
		for i := range items {
			if items[i].ID == id {
				found = true
				if items[i].Trust.Status != "verified" {
					t.Fatalf("%s trust=%q", id, items[i].Trust.Status)
				}
				if !items[i].Install.PreviewOnly {
					t.Fatalf("%s unexpectedly executable", id)
				}
				if executableCatalogPlan(items[i].Install) {
					t.Fatalf("%s preview plan became executable", id)
				}
				break
			}
		}
		if !found {
			t.Fatalf("%s missing", id)
		}
	}
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
