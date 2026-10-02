package main

import "testing"

func TestP29EcosystemBatchB(t *testing.T) {
	requireAuditedEcosystemIDs(t,
		"susanin-keenetic",
		"trusttunnel-keenetic",
		"tg-ws-proxy-go",
		"tg-ws-proxy-rs",
		"aiway-manager",
		"bird4static",
		"ipset4static",
	)
}

func TestSusaninUsesNonInteractiveInstallMode(t *testing.T) {
	item := susaninCatalogItem()
	if len(item.Install.Args) != 1 || item.Install.Args[0] != "--yes" {
		t.Fatalf("Susanin install args=%v", item.Install.Args)
	}
}

func TestP29BatchBExecutableScriptsUseApprovedHTTPSHosts(t *testing.T) {
	for _, item := range []catalogItem{susaninCatalogItem(), tgWSProxyRSCatalogItem(), aiwayManagerCatalogItem()} {
		if !validOfficialScriptURL(item.Install.InstallerURL) {
			t.Fatalf("%s installer URL rejected: %s", item.ID, item.Install.InstallerURL)
		}
	}
}
