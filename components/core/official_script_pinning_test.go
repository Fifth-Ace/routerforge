package main

import "testing"

func TestOfficialScriptExpectedSHA256Validation(t *testing.T) {
	good := catalogInstallPlan{
		Method:         "official-script",
		InstallerURL:   "https://api.brovibe.cloud/releases/stable/broray/3.1.1-r12/INSTALL-ON-ROUTER.sh",
		ExpectedSHA256: "ac334c4f3ce16e9119dcc3b84e21bd076ba5fce5252cf0f6aef2df5510edba1a",
	}
	if err := validateCatalogPlan(good); err != nil {
		t.Fatalf("valid pinned official-script rejected: %v", err)
	}

	for _, value := range []string{
		"abc",
		"AC334C4F3CE16E9119DCC3B84E21BD076BA5FCE5252CF0F6AEF2DF5510EDBA1A",
		"gc334c4f3ce16e9119dcc3b84e21bd076ba5fce5252cf0f6aef2df5510edba1a",
	} {
		plan := good
		plan.ExpectedSHA256 = value
		if err := validateCatalogPlan(plan); err == nil {
			t.Fatalf("invalid expected sha accepted: %q", value)
		}
	}
}

func TestBROrayPinnedInstallerExecutable(t *testing.T) {
	item := broRayCatalogItem()
	if item.Install.PreviewOnly {
		t.Fatal("BROray remained preview-only")
	}
	if item.Install.ExpectedSHA256 != "ac334c4f3ce16e9119dcc3b84e21bd076ba5fce5252cf0f6aef2df5510edba1a" {
		t.Fatalf("unexpected pinned digest=%q", item.Install.ExpectedSHA256)
	}
	if !executableCatalogPlan(item.Install) {
		t.Fatal("BROray pinned installer is not executable")
	}
	if err := validateCatalogPlan(item.Install); err != nil {
		t.Fatalf("BROray install plan invalid: %v", err)
	}
}

func TestOfficialScriptPinnedDigestMatch(t *testing.T) {
	plan := catalogInstallPlan{
		Method:         "official-script",
		InstallerURL:   "https://raw.githubusercontent.com/example/project/main/install.sh",
		ExpectedSHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
	if err := verifyOfficialScriptDigest(plan, []byte{}); err != nil {
		t.Fatalf("matching digest rejected: %v", err)
	}
	if err := verifyOfficialScriptDigest(plan, []byte("x")); err == nil {
		t.Fatal("digest mismatch accepted")
	}
}
