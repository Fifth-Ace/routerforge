package main

import "testing"

func TestXKeenPanelReviewedLifecycleIsExecutable(t *testing.T) {
	item := xkeenPanelCatalogItem()
	if item.Install.PreviewOnly {
		t.Fatal("install remained preview-only")
	}
	if item.Update.PreviewOnly {
		t.Fatal("update remained preview-only")
	}
	if !executableCatalogPlan(item.Install) {
		t.Fatal("install plan is not executable")
	}
	if !executableCatalogPlan(item.Update) {
		t.Fatal("update plan is not executable")
	}
	if err := validateCatalogPlan(item.Install); err != nil {
		t.Fatalf("install plan invalid: %v", err)
	}
	if err := validateCatalogPlan(item.Update); err != nil {
		t.Fatalf("update plan invalid: %v", err)
	}
	if item.Install.InstallerURL != "https://raw.githubusercontent.com/Dearonski/xkeen-panel/main/install.sh" {
		t.Fatalf("unexpected installer URL: %s", item.Install.InstallerURL)
	}
}

func TestXKeenPanelNoArchitectureArgumentNeeded(t *testing.T) {
	item := xkeenPanelCatalogItem()
	if len(item.Install.Args) != 0 {
		t.Fatalf("unexpected static architecture args: %v", item.Install.Args)
	}
	if len(item.Compatibility.Targets) != 3 {
		t.Fatalf("unexpected targets: %v", item.Compatibility.Targets)
	}
}
