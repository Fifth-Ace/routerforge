package main

import "testing"

func TestAuditedEcosystemCatalogIDsUnique(t *testing.T) {
	items := auditedEcosystemIntegrations()
	seen := map[string]bool{}
	for _, item := range items {
		if item.ID == "" {
			t.Fatal("empty catalog id")
		}
		if seen[item.ID] {
			t.Fatalf("duplicate catalog id %q", item.ID)
		}
		seen[item.ID] = true
		if item.Kind != "integration" {
			t.Fatalf("%s kind=%q", item.ID, item.Kind)
		}
		if item.Trust.Status != "verified" {
			t.Fatalf("%s trust=%q", item.ID, item.Trust.Status)
		}
	}
}

func TestKeeneticPolicyUILifecycleIsExecutableAndBounded(t *testing.T) {
	item := keeneticPolicyUICatalogItem()
	if item.Install.Method != "structured" || item.Install.PreviewOnly {
		t.Fatalf("unexpected install plan: %#v", item.Install)
	}
	if !executableCatalogPlan(item.Install) {
		t.Fatal("Keenetic Policy UI install plan must be executable")
	}
	if err := validateCatalogPlan(item.Install); err != nil {
		t.Fatalf("install plan rejected: %v", err)
	}
	if item.Web == nil || item.Web.Port != 3000 || item.Web.Mode != "probe-required" {
		t.Fatalf("unexpected web metadata: %#v", item.Web)
	}
}

func TestAuditedPreviewOnlyProjectsDoNotGainInstallAuthority(t *testing.T) {
	for _, item := range []catalogItem{
		entwareManagerCatalogItem(),
		razvilkaCatalogItem(),
		keeneticMCPCatalogItem(),
	} {
		actions := deriveCatalogActions(item)
		if actions.Install {
			t.Fatalf("%s unexpectedly gained automatic install authority", item.ID)
		}
	}
}

func TestAuditedOverlayDoesNotReplaceCanonicalRegistryItem(t *testing.T) {
	snapshot := catalogSnapshot{
		Integrations: []catalogItem{{
			ID: "keenetic-policy-ui", Kind: "integration", Name: "Canonical",
		}},
	}
	applyAuditedEcosystemCatalog(&snapshot, map[string]string{})
	count := 0
	for _, item := range snapshot.Integrations {
		if item.ID == "keenetic-policy-ui" {
			count++
			if item.Name != "Canonical" {
				t.Fatalf("canonical item was replaced: %q", item.Name)
			}
		}
	}
	if count != 1 {
		t.Fatalf("keenetic-policy-ui count=%d, want 1", count)
	}
}
