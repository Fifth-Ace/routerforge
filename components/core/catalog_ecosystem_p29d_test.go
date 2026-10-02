package main

import "testing"

func TestP29EcosystemBatchD(t *testing.T) {
	requireAuditedEcosystemIDs(t,
		"magitrickle-badigit",
		"magitrickle-larin",
		"xkeen-ui-fan92",
		"dropweb-xkeen",
		"wdtt-server-entware",
		"netcraze-giga-awg3",
	)
}

func TestP29BatchDExecutableScriptsUseApprovedHTTPSHosts(t *testing.T) {
	for _, item := range []catalogItem{
		magiTrickleBadigitCatalogItem(),
		magiTrickleLarinCatalogItem(),
		xkeenUIFan92CatalogItem(),
	} {
		if !validOfficialScriptURL(item.Install.InstallerURL) {
			t.Fatalf("%s installer URL rejected: %s", item.ID, item.Install.InstallerURL)
		}
	}
}

func TestNetcrazeAWG3RemainsHardwareSpecificPreview(t *testing.T) {
	item := netcrazeAWG3CatalogItem()
	if !item.Install.PreviewOnly || item.Install.Method != "manual" {
		t.Fatalf("unexpected install plan=%#v", item.Install)
	}
	if len(item.Compatibility.Targets) != 1 || item.Compatibility.Targets[0] != "aarch64-3.10" {
		t.Fatalf("unexpected targets=%v", item.Compatibility.Targets)
	}
}
