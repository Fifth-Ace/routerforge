package main

import (
	"path/filepath"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/configvault"
)

func TestAdminConfigVaultPathWithinRoot(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "opt", "etc", "routerforge")
	if !adminConfigVaultPathWithinRoot(root, root) {
		t.Fatal("managed root must match itself")
	}
	if !adminConfigVaultPathWithinRoot(root, filepath.Join(root, "dns", "config.json")) {
		t.Fatal("managed child must be detected")
	}
	if adminConfigVaultPathWithinRoot(root, filepath.Join(filepath.Dir(root), "routerforge-other", "config")) {
		t.Fatal("prefix sibling must not be treated as managed")
	}
	if adminConfigVaultPathWithinRoot(root, "relative/path") {
		t.Fatal("relative path must not be treated as managed")
	}
}

func TestValidateAdminConfigVaultRestoreManifestAllowsEmptyBaseline(t *testing.T) {
	root := filepath.Join(t.TempDir(), "managed")
	manifest := configvault.Manifest{
		SchemaVersion: configvault.SchemaVersion,
		Component:     "admin",
		Artifacts:     []configvault.ArtifactRecord{},
	}
	if err := validateAdminConfigVaultRestoreManifest(root, manifest); err != nil {
		t.Fatalf("empty baseline must be restorable: %v", err)
	}
}
