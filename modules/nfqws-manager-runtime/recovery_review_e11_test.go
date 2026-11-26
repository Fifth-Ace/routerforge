package main

import (
	"os"
	"path/filepath"
	"testing"
)

// E11 exposes the existing native recovery endpoint from Backup Center.
// Exercise its file boundary rather than changing the native restore logic.
func TestE11RecoveryReadExactLimitAndParent(t *testing.T) {
	root := t.TempDir()
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(realRoot, "custom.list")
	data := []byte("example.org\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, info, err := nfRecoveryRead(path, int64(len(data)))
	if err != nil || string(got) != string(data) || info == nil || info.Mode().Perm() != 0600 {
		t.Fatalf("exact bound read: %v", err)
	}
	if _, _, err := nfRecoveryRead(path, int64(len(data)-1)); err == nil {
		t.Fatal("oversize accepted")
	}
	link := filepath.Join(realRoot, "alias")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, _, err := nfRecoveryRead(filepath.Join(link, "custom.list"), int64(len(data))); err == nil {
		t.Fatal("symlink parent accepted")
	}
}
