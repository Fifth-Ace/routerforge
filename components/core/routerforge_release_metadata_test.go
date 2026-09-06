package main

import "testing"

func TestValidateCatalogReleaseMetadata(t *testing.T) {
	valid := catalogRelease{
		Package:            "routerforge-system",
		Architecture:       "aarch64-3.10",
		SizeBytes:          1234,
		InstalledSizeBytes: 4321,
		Depends:            []string{"routerforge-core"},
		Conflicts:          []string{"dns-monitor-system"},
	}
	if err := validateCatalogReleaseMetadata(valid, "aarch64-3.10"); err != nil {
		t.Fatalf("valid metadata rejected: %v", err)
	}

	badArch := valid
	badArch.Architecture = "mips-3.4"
	if err := validateCatalogReleaseMetadata(badArch, "aarch64-3.10"); err == nil {
		t.Fatal("architecture mismatch must be rejected")
	}

	badDependency := valid
	badDependency.Depends = []string{"routerforge-core;rm"}
	if err := validateCatalogReleaseMetadata(badDependency, "aarch64-3.10"); err == nil {
		t.Fatal("unsafe dependency must be rejected")
	}
}
