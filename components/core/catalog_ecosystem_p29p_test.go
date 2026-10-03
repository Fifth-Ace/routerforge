package main

import "testing"

func TestP29PLocalScriptValidation(t *testing.T) {
	ok := catalogInstallPlan{Method: "local-script", ScriptPath: "/opt/etc/xkeen-ui/uninstall.sh"}
	if err := validateCatalogPlan(ok); err != nil {
		t.Fatalf("safe local-script rejected: %v", err)
	}
	for _, path := range []string{
		"",
		"/tmp/uninstall.sh",
		"/opt/etc/../tmp/uninstall.sh",
		"/opt/bin/uninstall.sh",
		"relative/uninstall.sh",
	} {
		plan := catalogInstallPlan{Method: "local-script", ScriptPath: path}
		if err := validateCatalogPlan(plan); err == nil {
			t.Fatalf("unsafe local-script path accepted: %q", path)
		}
	}
	if !executableCatalogPlan(ok) {
		t.Fatal("local-script must be an executable lifecycle method")
	}
}

func TestP29PSafeRemoveUnlocks(t *testing.T) {
	xkeen := xkeenUIUmarchehCatalogItem()
	if xkeen.Install.Method != "manual" || !xkeen.Install.PreviewOnly {
		t.Fatalf("Xkeen install authority changed: %#v", xkeen.Install)
	}
	if xkeen.Remove.Method != "local-script" || xkeen.Remove.ScriptPath != "/opt/etc/xkeen-ui/uninstall.sh" {
		t.Fatalf("Xkeen remove mismatch: %#v", xkeen.Remove)
	}
	if err := validateCatalogPlan(xkeen.Remove); err != nil {
		t.Fatalf("Xkeen remove invalid: %v", err)
	}

	wdtt := wdttServerEntwareCatalogItem()
	if wdtt.Install.Method != "manual" || !wdtt.Install.PreviewOnly {
		t.Fatalf("WDTT install authority changed: %#v", wdtt.Install)
	}
	if wdtt.Remove.Method != "official-script" || len(wdtt.Remove.Args) != 1 || wdtt.Remove.Args[0] != "--uninstall" {
		t.Fatalf("WDTT remove mismatch: %#v", wdtt.Remove)
	}
	if wdtt.Remove.ExpectedSHA256 != "17a2df63face66e241dd475bec4ed6f383339bd2a78ec4e6ce688d2b5d9d36f4" {
		t.Fatalf("WDTT remove digest mismatch: %q", wdtt.Remove.ExpectedSHA256)
	}
	if err := validateCatalogPlan(wdtt.Remove); err != nil {
		t.Fatalf("WDTT remove invalid: %v", err)
	}
}
