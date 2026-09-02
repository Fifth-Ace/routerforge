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
	if nfqws2 == nil || !nfqws2.Installed || nfqws2.WebPort != 90 {
		t.Fatalf("bad nfqws2 state: %#v", nfqws2)
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
