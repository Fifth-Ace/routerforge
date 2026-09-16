package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/configvault"
)

func TestValidateAdminConfigVaultRestorePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "managed")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "dns", "config.json")
	if got, err := validateAdminConfigVaultRestorePath(root, inside); err != nil || got != inside {
		t.Fatalf("inside path rejected: got=%q err=%v", got, err)
	}
	if _, err := validateAdminConfigVaultRestorePath(root, root); err == nil {
		t.Fatal("managed root itself must not be a restore file")
	}
	if _, err := validateAdminConfigVaultRestorePath(root, filepath.Join(filepath.Dir(root), "outside")); err == nil {
		t.Fatal("outside path must be rejected")
	}
}

func TestValidateAdminConfigVaultRestoreManifestRejectsDuplicates(t *testing.T) {
	root := filepath.Join(t.TempDir(), "managed")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config")
	manifest := configvault.Manifest{
		SchemaVersion: configvault.SchemaVersion,
		Component:     "admin",
		Artifacts: []configvault.ArtifactRecord{
			{ID: "one", SourcePath: path},
			{ID: "two", SourcePath: path},
		},
	}
	if err := validateAdminConfigVaultRestoreManifest(root, manifest); err == nil {
		t.Fatal("duplicate restore path must be rejected")
	}
}

func TestEnsureAdminConfigVaultParentRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("control package is validated in Linux CI")
	}
	root := filepath.Join(t.TempDir(), "managed")
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := ensureAdminConfigVaultParent(root, filepath.Join(link, "config")); err == nil {
		t.Fatal("symlink parent must be rejected")
	}
}
