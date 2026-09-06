package main

import (
	"reflect"
	"testing"
)

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

func TestParseEntwareInstalled(t *testing.T) {
	got := parseEntwareInstalled("curl - 8.17.0-1\nnano - 8.6-1\nbad line\n")
	want := map[string]string{"curl": "8.17.0-1", "nano": "8.6-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("installed=%#v, want %#v", got, want)
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

func TestParseOpkgControlStanzas(t *testing.T) {
	raw := "Package: foo\nVersion: 1.2.3\nArchitecture: aarch64-3.10\nDepends: libc, libssl | openssl\nDescription: first line\n second line\n\nPackage: bar\nVersion: 2\nStatus: install user installed\n"
	got := parseOpkgControlStanzas(raw)
	if len(got) != 2 {
		t.Fatalf("stanzas=%d, want 2", len(got))
	}
	if got[0]["Package"] != "foo" || got[0]["Architecture"] != "aarch64-3.10" {
		t.Fatalf("unexpected first stanza: %#v", got[0])
	}
	if got[0]["Description"] != "first line\nsecond line" {
		t.Fatalf("continuation not preserved: %q", got[0]["Description"])
	}
}

func TestParseDependencyGroups(t *testing.T) {
	got := parseDependencyGroups("libc (>= 1.0), libssl | openssl, zlib")
	want := [][]string{{"libc"}, {"libssl", "openssl"}, {"zlib"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groups=%#v, want %#v", got, want)
	}
}

func TestMissingDependencyGroupsHonorsAlternatives(t *testing.T) {
	groups := parseDependencyGroups("libc, libssl | openssl, zlib")
	installed := map[string]string{"libc": "1", "openssl": "3"}
	got := missingDependencyGroups(groups, installed)
	want := []string{"zlib"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%#v, want %#v", got, want)
	}
}

func TestReverseDependencies(t *testing.T) {
	stanzas := parseOpkgControlStanzas(
		"Package: app-a\nStatus: install user installed\nDepends: libc, target\n\n" +
			"Package: app-b\nStatus: install user installed\nDepends: other | target\n\n" +
			"Package: app-c\nStatus: deinstall user not-installed\nDepends: target\n\n",
	)
	got := reverseDependencies(stanzas, "target")
	want := []string{"app-a", "app-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reverse=%#v, want %#v", got, want)
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
