package main

import "testing"

func TestP29OPreviewUnlocks(t *testing.T) {
	sing := keeneticSingboxCatalogItem()
	if sing.Install.Method != "manual" || !sing.Install.PreviewOnly {
		t.Fatalf("sing-box first install authority changed: %#v", sing.Install)
	}
	if !executableCatalogPlan(sing.Update) || sing.Update.PreviewOnly {
		t.Fatalf("sing-box update must be executable: %#v", sing.Update)
	}
	if sing.Update.Method != "official-script" || len(sing.Update.Args) != 1 || sing.Update.Args[0] != "--update" {
		t.Fatalf("sing-box update contract mismatch: %#v", sing.Update)
	}
	if err := validateCatalogPlan(sing.Update); err != nil {
		t.Fatalf("sing-box update invalid: %v", err)
	}

	traffic := keeneticTrafficViaVPNCatalogItem()
	if traffic.Install.Method != "official-script" || traffic.Install.PreviewOnly {
		t.Fatalf("traffic-via-vpn install not unlocked: %#v", traffic.Install)
	}
	if !executableCatalogPlan(traffic.Install) {
		t.Fatalf("traffic-via-vpn install not executable: %#v", traffic.Install)
	}
	if err := validateCatalogPlan(traffic.Install); err != nil {
		t.Fatalf("traffic-via-vpn install invalid: %v", err)
	}
}

func TestP29OUnlocksUseCommitPinnedURLs(t *testing.T) {
	sing := keeneticSingboxCatalogItem()
	traffic := keeneticTrafficViaVPNCatalogItem()

	if sing.Update.InstallerURL != "https://raw.githubusercontent.com/inlarin/keenetic-singbox-installer/24e181fc76202a85edf3f100aa9bcae2579bdeef/install.sh" {
		t.Fatalf("sing-box update URL not commit pinned: %s", sing.Update.InstallerURL)
	}
	if traffic.Install.InstallerURL != "https://raw.githubusercontent.com/rustrict/keenetic-traffic-via-vpn/5c0521ab3b1e632a44d39691fe98d818062f9894/install.sh" {
		t.Fatalf("traffic-via-vpn installer URL not commit pinned: %s", traffic.Install.InstallerURL)
	}
}
