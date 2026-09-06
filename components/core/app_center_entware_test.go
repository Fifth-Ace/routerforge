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
	for _, name := range []string{"routerforge-core", "routerforge-dns", "routerforge-foo", "opkg"} {
		if !entwarePackageProtected(name) {
			t.Fatalf("%s should be protected", name)
		}
	}
	for _, name := range []string{"curl", "nano", "adguardhome-go"} {
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
