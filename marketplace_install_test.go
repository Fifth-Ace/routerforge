package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCatalogItemTestInstallable(t *testing.T) {
	good := catalogItem{
		Kind: "module", Managed: true,
		Install: catalogInstallPlan{
			Method: "opkg-feed", Repository: "dns-monitor",
			Packages: []string{"dns-monitor-thermal"},
		},
	}
	if !catalogItemTestInstallable(good) {
		t.Fatal("managed DNS Monitor module should be test-installable")
	}

	builtin := good
	builtin.Builtin = true
	if catalogItemTestInstallable(builtin) {
		t.Fatal("built-in module must not be installable")
	}

	thirdParty := good
	thirdParty.Kind = "integration"
	if catalogItemTestInstallable(thirdParty) {
		t.Fatal("third-party integration must not be test-installable")
	}

	script := good
	script.Install.Method = "official-script"
	if catalogItemTestInstallable(script) {
		t.Fatal("script installer must not be test-installable")
	}

	unsafe := good
	unsafe.Install.Packages = []string{"../../evil"}
	if catalogItemTestInstallable(unsafe) {
		t.Fatal("unsafe package name must be rejected")
	}
}

func TestFindNewestLocalIPK(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "dns-monitor-thermal_0.1.ipk")
	newPath := filepath.Join(dir, "dns-monitor-thermal_0.2.ipk")
	if err := os.WriteFile(oldPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(oldPath, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, now, now); err != nil {
		t.Fatal(err)
	}

	if got := findNewestLocalIPK(dir, "dns-monitor-thermal"); got != newPath {
		t.Fatalf("got %q want %q", got, newPath)
	}
	if got := findNewestLocalIPK(dir, "../../evil"); got != "" {
		t.Fatalf("unsafe package returned %q", got)
	}
}
