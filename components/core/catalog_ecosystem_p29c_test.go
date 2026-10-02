package main

import "testing"

func TestP29EcosystemBatchC(t *testing.T) {
	requireAuditedEcosystemIDs(t,
		"kee-geo-split",
		"kee-smartdns-geo-conf",
		"kee-smartdns-redirect",
		"kee-net-check",
		"kee-webui",
		"trusttunnel-keenetic-native",
		"tg-ws-keenetic",
		"wireguard-dpi-bypass",
	)
}

func TestKeeneticExtrasChildrenUseExplicitPackageArgs(t *testing.T) {
	for _, item := range []catalogItem{
		keeGeoSplitCatalogItem(),
		keeSmartDNSGeoConfCatalogItem(),
		keeSmartDNSRedirectCatalogItem(),
		keeNetCheckCatalogItem(),
		keeWebUICatalogItem(),
	} {
		if len(item.Install.Args) != 1 || len(item.Install.Packages) != 1 || item.Install.Args[0] != item.Install.Packages[0] {
			t.Fatalf("%s package args mismatch: args=%v packages=%v", item.ID, item.Install.Args, item.Install.Packages)
		}
	}
}

func TestTGWSKeeneticPinsEntwareExecutionMode(t *testing.T) {
	item := tgWSKeeneticCatalogItem()
	if len(item.Install.Args) != 2 || item.Install.Args[0] != "--system" || item.Install.Args[1] != "entware" {
		t.Fatalf("install args=%v", item.Install.Args)
	}
	if item.Remove.Method != "official-script" || len(item.Remove.Args) != 2 || item.Remove.Args[1] != "entware" {
		t.Fatalf("remove plan=%#v", item.Remove)
	}
}

func TestP29BatchCExecutableScriptsUseApprovedHTTPSHosts(t *testing.T) {
	for _, item := range []catalogItem{
		keeGeoSplitCatalogItem(),
		keeSmartDNSGeoConfCatalogItem(),
		keeSmartDNSRedirectCatalogItem(),
		keeNetCheckCatalogItem(),
		keeWebUICatalogItem(),
		tgWSKeeneticCatalogItem(),
		wireguardDPIBypassCatalogItem(),
	} {
		if !validOfficialScriptURL(item.Install.InstallerURL) {
			t.Fatalf("%s installer URL rejected: %s", item.ID, item.Install.InstallerURL)
		}
	}
}
