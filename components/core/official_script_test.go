package main

import "testing"

func TestOfficialScriptPlanValidation(t *testing.T) {
	good := catalogInstallPlan{
		Method:       "official-script",
		InstallerURL: "https://raw.githubusercontent.com/example/project/main/install.sh",
		Args:         []string{"--install", "stable"},
	}
	if err := validateCatalogPlan(good); err != nil {
		t.Fatalf("valid official-script rejected: %v", err)
	}
	if !executableCatalogPlan(good) {
		t.Fatal("validated official-script must be executable")
	}

	for _, bad := range []catalogInstallPlan{
		{Method: "official-script", InstallerURL: "http://raw.githubusercontent.com/example/project/main/install.sh"},
		{Method: "official-script", InstallerURL: "https://127.0.0.1/install.sh"},
		{Method: "official-script", InstallerURL: "https://example.com/install.sh"},
		{Method: "official-script", InstallerURL: "https://raw.githubusercontent.com/example/project/main/install.sh", Args: []string{"bad\narg"}},
	} {
		if err := validateCatalogPlan(bad); err == nil {
			t.Fatalf("unsafe official-script accepted: %#v", bad)
		}
	}
}

func TestOfficialScriptPreviewOnlyStaysNonExecutable(t *testing.T) {
	plan := catalogInstallPlan{
		Method:       "official-script",
		InstallerURL: "https://raw.githubusercontent.com/example/project/main/install.sh",
		PreviewOnly:  true,
	}
	if executableCatalogPlan(plan) {
		t.Fatal("preview-only official-script became executable")
	}
}
