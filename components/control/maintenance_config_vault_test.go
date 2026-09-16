package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdminConfigVaultArtifactIDStable(t *testing.T) {
	a := adminConfigVaultArtifactID("dns/resolvers.json")
	b := adminConfigVaultArtifactID(filepath.Join("dns", "resolvers.json"))
	if a != b {
		t.Fatalf("artifact id must be separator-stable: %q != %q", a, b)
	}
	if a == adminConfigVaultArtifactID("dns/other.json") {
		t.Fatal("different paths must not share artifact id")
	}
}

func TestSHA256RegularFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config")
	if err := os.WriteFile(target, []byte("ok\n"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := sha256RegularFile(link); err == nil {
		t.Fatal("symlink must be rejected")
	}
}

func TestSHA256RegularFileStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("routerforge\n"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := sha256RegularFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sha256RegularFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("unexpected checksum: %q %q", first, second)
	}
}
