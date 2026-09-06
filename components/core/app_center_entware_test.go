package main

import "testing"

func TestParseEntwareList(t *testing.T) {
	items := parseEntwareList("curl - 8.17.0-1 - Command line transfer tool\nnano - 8.6-1 - Small editor\n")
	if len(items) != 2 {
		t.Fatalf("items=%d, want 2", len(items))
	}
	if got := items["curl"].AvailableVersion; got != "8.17.0-1" {
		t.Fatalf("curl version=%q", got)
	}
	if got := items["nano"].Description; got != "Small editor" {
		t.Fatalf("nano description=%q", got)
	}
}

func TestEntwarePackageProtection(t *testing.T) {
	for _, name := range []string{"routerforge-core", "routerforge-dns", "routerforge-foo", "opkg", "awg-manager", "nfqws2-keenetic", "nfqws-keenetic-web", "adguardhome-go"} {
		if !entwarePackageProtected(name) {
			t.Fatalf("%s should be protected", name)
		}
	}
	for _, name := range []string{"curl", "nano", "jq", "htop"} {
		if entwarePackageProtected(name) {
			t.Fatalf("%s should not be protected", name)
		}
	}
}

func TestParseEntwareIntClamps(t *testing.T) {
	if got := parseEntwareInt("999", 100, 1, 250); got != 250 {
		t.Fatalf("got=%d", got)
	}
	if got := parseEntwareInt("-5", 100, 0, 250); got != 0 {
		t.Fatalf("got=%d", got)
	}
}

func TestParseEntwareUpgradableUsesTargetVersion(t *testing.T) {
	got := parseEntwareUpgradable("awg-manager - 2.17.2 - 2.17.3\nnfqws2-keenetic - 1.2.4 - 1.2.5\n")
	if got["awg-manager"] != "2.17.3" {
		t.Fatalf("awg-manager target=%q", got["awg-manager"])
	}
	if got["nfqws2-keenetic"] != "1.2.5" {
		t.Fatalf("nfqws2 target=%q", got["nfqws2-keenetic"])
	}
}

func TestFilterEntwarePackagesHidesAppCenterOwnedPackages(t *testing.T) {
	items := []entwarePackage{
		{Name: "awg-manager"},
		{Name: "nfqws2-keenetic"},
		{Name: "routerforge-core"},
		{Name: "opkg"},
		{Name: "curl"},
	}
	got := filterEntwarePackages(items)
	if len(got) != 1 || got[0].Name != "curl" {
		t.Fatalf("unexpected visible Entware packages: %#v", got)
	}
}

func TestApplyIntegrationPackageVersions(t *testing.T) {
	item := awgManagerIntegration()
	finalizeCatalogItem(
		&item,
		map[string]string{"awg-manager": "2.17.2"},
		map[string]bool{},
		func(string) bool { return false },
	)
	snapshot := catalogSnapshot{Integrations: []catalogItem{item}}

	applyIntegrationPackageVersions(&snapshot, []entwarePackage{{
		Name:             "awg-manager",
		Installed:        true,
		InstalledVersion: "2.17.2",
		AvailableVersion: "2.17.3",
		Upgradable:       true,
	}})

	got := snapshot.Integrations[0]
	if got.Version != "2.17.2" || got.AvailableVersion != "2.17.3" {
		t.Fatalf("unexpected versions: installed=%q available=%q", got.Version, got.AvailableVersion)
	}
	if !got.PackageInstalled || !got.UpdateAvailable {
		t.Fatalf("package state was not propagated: %#v", got)
	}
}
