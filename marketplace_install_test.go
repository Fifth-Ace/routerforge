package main

import (
	"strings"
	"testing"
)

func TestSafeCatalogPackageName(t *testing.T) {
	for _, value := range []string{"routerforge-thermal", "nfqws2-keenetic", "adguardhome-go"} {
		if !safeCatalogPackageName(value) {
			t.Fatalf("expected safe package: %q", value)
		}
	}
	for _, value := range []string{"../../evil", "bad package", "/tmp/x"} {
		if safeCatalogPackageName(value) {
			t.Fatalf("expected unsafe package: %q", value)
		}
	}
}

func TestExpandAssetTemplate(t *testing.T) {
	got := expandAssetTemplate("{package}_{version}_aarch64-3.10.ipk", "routerforge-thermal", "0.3.0-beta")
	want := "routerforge-thermal_0.3.0-beta_aarch64-3.10.ipk"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := expandAssetTemplate("../../{package}.ipk", "routerforge-thermal", "0.3.0-beta"); got != "" {
		t.Fatalf("unsafe template returned %q", got)
	}
}

func TestParseChecksumList(t *testing.T) {
	got, err := parseChecksumList("03c9787e77338360005886d13aac316d8ac8ad8611aeafc46ebf0304c3df6f30  routerforge-core_0.3.0-beta_aarch64-3.10.ipk\n")
	if err != nil {
		t.Fatal(err)
	}
	if got["routerforge-core_0.3.0-beta_aarch64-3.10.ipk"] == "" {
		t.Fatal("checksum was not parsed")
	}
}

func TestValidateCatalogActionCompletionRejectsVersionMismatch(t *testing.T) {
	item := catalogItem{Release: catalogRelease{Version: "0.4.18-beta"}}
	updated := catalogItem{Installed: true, Version: "0.4.17-beta"}

	failure := validateCatalogActionCompletion(item, updated, true, "update")
	if failure == nil {
		t.Fatal("version mismatch was accepted")
	}
	if failure.Status != 500 {
		t.Fatalf("status=%d, want 500", failure.Status)
	}
	if !strings.Contains(failure.Detail, "expected 0.4.18-beta, got 0.4.17-beta") {
		t.Fatalf("unexpected detail: %q", failure.Detail)
	}
}

func TestValidateCatalogActionCompletionAcceptsTargetVersion(t *testing.T) {
	item := catalogItem{Release: catalogRelease{Version: "0.4.18-beta"}}
	updated := catalogItem{Installed: true, Version: "0.4.18-beta"}

	if failure := validateCatalogActionCompletion(item, updated, true, "update"); failure != nil {
		t.Fatalf("matching release version was rejected: %v", failure)
	}
}
