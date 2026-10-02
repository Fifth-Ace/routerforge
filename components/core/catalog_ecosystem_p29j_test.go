package main

import "testing"

func TestVerifiedIPKPlanValidation(t *testing.T) {
	good := catalogInstallPlan{
		Method:         "verified-ipk",
		Packages:       []string{"zapret-gui"},
		InstallerURL:   "https://github.com/avatarDD/zapret-gui/releases/download/v0.25.5/zapret-gui-keenetic.ipk",
		ExpectedSHA256: "76d9b57e911ccc9b4ce03712be983c1559101bed80c3b887b26acb90ec536274",
	}
	if err := validateCatalogPlan(good); err != nil {
		t.Fatalf("valid verified-ipk plan rejected: %v", err)
	}
	if !executableCatalogPlan(good) {
		t.Fatal("verified-ipk plan is not executable")
	}

	bad := good
	bad.InstallerURL = "https://example.com/zapret-gui.ipk"
	if err := validateCatalogPlan(bad); err == nil {
		t.Fatal("non-GitHub verified-ipk URL accepted")
	}

	bad = good
	bad.ExpectedSHA256 = "abc"
	if err := validateCatalogPlan(bad); err == nil {
		t.Fatal("invalid digest accepted")
	}
}

func TestVerifiedIPKDigestVerification(t *testing.T) {
	data := []byte("routerforge")
	sum := "dcafacc0d5ead6c1fe3a923ac877fc4a40929bccd6eaf6448f3c8a80c28d0e07"
	if _, err := verifyExpectedSHA256(sum, data); err != nil {
		t.Fatalf("matching digest rejected: %v", err)
	}
	if _, err := verifyExpectedSHA256("0000000000000000000000000000000000000000000000000000000000000000", data); err == nil {
		t.Fatal("digest mismatch accepted")
	}
}

func TestZapretGUIUsesPinnedVerifiedIPK(t *testing.T) {
	item := zapretGUICatalogItem()
	if item.Install.Method != "verified-ipk" || item.Install.PreviewOnly {
		t.Fatalf("install=%#v", item.Install)
	}
	if item.Install.ExpectedSHA256 != "76d9b57e911ccc9b4ce03712be983c1559101bed80c3b887b26acb90ec536274" {
		t.Fatalf("digest=%q", item.Install.ExpectedSHA256)
	}
	if item.Update.Method != "verified-ipk" {
		t.Fatalf("update=%#v", item.Update)
	}
	if item.Remove.Method != "opkg" {
		t.Fatalf("remove=%#v", item.Remove)
	}
}
