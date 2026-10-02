package main

import "testing"

func TestKeenPBRFullStructuredLifecycle(t *testing.T) {
	item := catalogItem{
		ID:      "keen-pbr",
		Install: keenPBRStructuredPlan("keen-pbr"),
	}
	if item.Install.PreviewOnly || !executableCatalogPlan(item.Install) {
		t.Fatalf("install=%#v", item.Install)
	}
	if err := validateCatalogPlan(item.Install); err != nil {
		t.Fatalf("install plan invalid: %v", err)
	}
	if len(item.Install.Steps) != 3 || item.Install.Steps[0].Type != "write-opkg-feed-arch" {
		t.Fatalf("steps=%#v", item.Install.Steps)
	}
}

func TestKeenPBRHeadlessCatalogLifecycle(t *testing.T) {
	item := keenPBRHeadlessCatalogItem()
	if item.Install.PreviewOnly || !executableCatalogPlan(item.Install) {
		t.Fatalf("install=%#v", item.Install)
	}
	if !executableCatalogPlan(item.Update) || !executableCatalogPlan(item.Remove) {
		t.Fatalf("update/remove not executable: update=%#v remove=%#v", item.Update, item.Remove)
	}
	if item.Web != nil {
		t.Fatalf("headless unexpectedly exposes Web metadata: %#v", item.Web)
	}
	if len(item.Detection.Packages) != 1 || item.Detection.Packages[0] != "keen-pbr-headless" {
		t.Fatalf("packages=%v", item.Detection.Packages)
	}
}

func TestKeenPBRFeedSupportsPublishedEntwareArchitectures(t *testing.T) {
	plan := keenPBRStructuredPlan("keen-pbr")
	step := plan.Steps[0]
	want := map[string]bool{
		"aarch64-3.10": true,
		"mips-3.4":     true,
		"mipsel-3.4":   true,
		"armv7-3.2":    true,
		"x64-3.2":      true,
	}
	if len(step.Args) != len(want) {
		t.Fatalf("architectures=%v", step.Args)
	}
	for _, arch := range step.Args {
		if !want[arch] {
			t.Fatalf("unexpected architecture=%q", arch)
		}
	}
}
