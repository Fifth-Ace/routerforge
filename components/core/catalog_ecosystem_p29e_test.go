package main

import "testing"

func TestP29EcosystemBatchE(t *testing.T) {
	requireAuditedEcosystemIDs(t,
		"b4",
		"hydra-bridge",
		"ssclash-go",
		"broray",
		"keenetic-auto-setup",
	)
}

func TestHydraBridgeApprovedHost(t *testing.T) {
	item := hydraBridgeCatalogItem()
	if !validOfficialScriptURL(item.Install.InstallerURL) {
		t.Fatalf("HydraBridge installer rejected: %s", item.Install.InstallerURL)
	}
}

func TestBROrayUsesDigestPinnedExecutableInstaller(t *testing.T) {
	item := broRayCatalogItem()
	if item.Install.PreviewOnly || item.Install.Method != "official-script" {
		t.Fatalf("unexpected BROray install plan=%#v", item.Install)
	}
	if !executableCatalogPlan(item.Install) {
		t.Fatal("BROray pinned installer is not executable")
	}
	if item.Install.ExpectedSHA256 != "ac334c4f3ce16e9119dcc3b84e21bd076ba5fce5252cf0f6aef2df5510edba1a" {
		t.Fatalf("unexpected BROray installer digest=%q", item.Install.ExpectedSHA256)
	}
	if err := validateCatalogPlan(item.Install); err != nil {
		t.Fatalf("BROray install plan invalid: %v", err)
	}
}
