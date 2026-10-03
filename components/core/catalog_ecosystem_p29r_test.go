package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestP29REcosystemIdentityAndTrustInvariants(t *testing.T) {
	seen := map[string]bool{}
	items := auditedEcosystemIntegrations()
	if len(items) < 60 {
		t.Fatalf("audited ecosystem unexpectedly shrank: %d", len(items))
	}

	for _, item := range items {
		if item.ID == "" || item.Name == "" || item.Category == "" {
			t.Fatalf("incomplete ecosystem identity: %#v", item)
		}
		if seen[item.ID] {
			t.Fatalf("duplicate audited ecosystem ID: %s", item.ID)
		}
		seen[item.ID] = true

		if item.Kind != "integration" {
			t.Fatalf("%s kind=%q", item.ID, item.Kind)
		}
		if item.Trust.Status != "verified" {
			t.Fatalf("%s trust=%q", item.ID, item.Trust.Status)
		}
		if item.Publisher.Name == "" || item.Publisher.URL == "" {
			t.Fatalf("%s missing publisher metadata: %#v", item.ID, item.Publisher)
		}
		if item.ProjectURL == "" || !strings.HasPrefix(item.ProjectURL, "https://") {
			t.Fatalf("%s invalid project URL: %q", item.ID, item.ProjectURL)
		}
		if len(item.Compatibility.Targets) == 0 {
			t.Fatalf("%s has no compatibility targets", item.ID)
		}
	}
}

func TestP29RLifecyclePlanInvariants(t *testing.T) {
	for _, item := range auditedEcosystemIntegrations() {
		plans := []struct {
			action string
			plan   catalogInstallPlan
		}{
			{"install", item.Install},
			{"update", item.Update},
			{"remove", item.Remove},
		}
		for _, current := range plans {
			if err := validateCatalogPlan(current.plan); err != nil {
				t.Fatalf("%s %s plan invalid: %v", item.ID, current.action, err)
			}
			if current.plan.PreviewOnly && executableCatalogPlan(current.plan) {
				t.Fatalf("%s %s preview plan became executable: %#v", item.ID, current.action, current.plan)
			}
			if current.action == "install" && current.plan.Method == "local-script" {
				t.Fatalf("%s gained forbidden local-script install authority", item.ID)
			}
			if current.plan.Method == "local-script" && current.action != "update" && current.action != "remove" {
				t.Fatalf("%s local-script used for unsupported action %s", item.ID, current.action)
			}
		}
	}
}

func TestP29RExecutablePlanValidationMatrix(t *testing.T) {
	for _, item := range auditedEcosystemIntegrations() {
		for action, plan := range map[string]catalogInstallPlan{
			"install": item.Install,
			"update":  item.Update,
			"remove":  item.Remove,
		} {
			if !executableCatalogPlan(plan) {
				continue
			}
			if plan.Method == "manual" || plan.PreviewOnly {
				t.Fatalf("%s %s executable policy mismatch: %#v", item.ID, action, plan)
			}
			if err := validateCatalogPlan(plan); err != nil {
				t.Fatalf("%s %s executable plan rejected by validator: %v", item.ID, action, err)
			}
		}
	}
}

func TestP29RCoreSourcesContainLifecycleRuntime(t *testing.T) {
	path := filepath.Join("..", "..", "scripts", "build-opkg.sh")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read build-opkg.sh: %v", err)
	}
	text := string(data)
	for _, source := range []string{
		"catalog_ecosystem.go",
		"catalog_official_script_overrides.go",
		"official_script.go",
		"local_script.go",
		"marketplace_install.go",
		"routerforge_registry.go",
	} {
		if !strings.Contains(text, "\n"+source+"\n") {
			t.Fatalf("critical Core source missing from CORE_SOURCES: %s", source)
		}
	}
}
