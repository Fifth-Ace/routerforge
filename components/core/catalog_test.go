package main

import (
	"strings"
	"testing"
)

func TestParseOpkgStatus(t *testing.T) {
	got := parseOpkgStatus(strings.NewReader(`
Package: awg-manager
Version: 2.15.1
Status: install user installed

Package: nfqws2-keenetic
Version: 1.1.5
Status: install user installed
`))
	if got["awg-manager"] != "2.15.1" {
		t.Fatalf("awg-manager version=%q", got["awg-manager"])
	}
	if got["nfqws2-keenetic"] != "1.1.5" {
		t.Fatalf("nfqws2 version=%q", got["nfqws2-keenetic"])
	}
}

func TestParseOpkgStatusIgnoresNotInstalledTombstone(t *testing.T) {
	got := parseOpkgStatus(strings.NewReader(`
Package: routerforge-dns
Version: 0.4.18-beta
Status: install user installed

Package: routerforge-dns
Version: 0.4.17-beta
Status: install prefer not-installed
`))
	if got["routerforge-dns"] != "0.4.18-beta" {
		t.Fatalf("routerforge-dns version=%q, want 0.4.18-beta", got["routerforge-dns"])
	}
}

func TestParseOpkgStatusOmitsNotInstalledPackage(t *testing.T) {
	got := parseOpkgStatus(strings.NewReader(`
Package: routerforge-dns
Version: 0.4.17-beta
Status: install prefer not-installed
`))
	if version, ok := got["routerforge-dns"]; ok {
		t.Fatalf("not-installed package leaked into installed map with version %q", version)
	}
}

func TestCatalogDetectsExternalIntegrations(t *testing.T) {
	installed := map[string]string{
		"awg-manager":        "2.15.1",
		"nfqws2-keenetic":    "1.1.5",
		"nfqws-keenetic-web": "3.0.23",
	}
	processes := map[string]bool{
		"awg-manager": true,
		"nfqws2":      true,
	}
	snap := buildCatalog(installed, processes, func(string) bool { return false })

	var awg, nfqws2 *catalogItem
	for i := range snap.Integrations {
		switch snap.Integrations[i].ID {
		case "awg-manager":
			awg = &snap.Integrations[i]
		case "nfqws2":
			nfqws2 = &snap.Integrations[i]
		}
	}
	if awg == nil || !awg.Installed || awg.State != "installed_external" || !awg.ServiceRunning {
		t.Fatalf("bad awg state: %#v", awg)
	}
	if awg.Web == nil || awg.Web.Mode != "probe-required" || !awg.Web.Embed ||
		awg.Web.Scheme != "http" || awg.Web.Port != 2222 || awg.Web.Path != "/" {
		t.Fatalf("bad awg web contract: %#v", awg.Web)
	}
	if nfqws2 == nil || !nfqws2.Installed || nfqws2.WebPort != 90 {
		t.Fatalf("bad nfqws2 state: %#v", nfqws2)
	}
	if nfqws2.Web == nil || nfqws2.Web.Mode != "probe-required" || !nfqws2.Web.Embed || nfqws2.Web.Port != 90 {
		t.Fatalf("bad nfqws2 web contract: %#v", nfqws2.Web)
	}
}

func TestCatalogSuppressesWebWhenCompanionPackageIsMissing(t *testing.T) {
	installed := map[string]string{
		"nfqws2-keenetic": "1.1.5",
	}
	snap := buildCatalog(installed, map[string]bool{}, func(string) bool { return false })

	var nfqws2 *catalogItem
	for i := range snap.Integrations {
		if snap.Integrations[i].ID == "nfqws2" {
			nfqws2 = &snap.Integrations[i]
			break
		}
	}
	if nfqws2 == nil || !nfqws2.Installed {
		t.Fatalf("nfqws2 missing from catalog: %#v", nfqws2)
	}
	if nfqws2.WebPort != 0 || nfqws2.Web != nil {
		t.Fatalf("nfqws2 web UI leaked without companion package: port=%d web=%#v", nfqws2.WebPort, nfqws2.Web)
	}
}

func TestCatalogInstallPlansArePreviewOnly(t *testing.T) {
	snap := buildCatalog(map[string]string{}, map[string]bool{}, func(string) bool { return false })
	if !snap.ReadOnly {
		t.Fatal("catalog foundation must be read-only")
	}
	for _, item := range snap.Integrations {
		if !item.Install.PreviewOnly {
			t.Fatalf("%s install plan is not preview-only", item.ID)
		}
	}
}
