package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestE062NFRecoveryTarget(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"user.list", "auto.list", "rf-binding-a.list", "RF-BINDING-X.list", "foo.list-old", "../else.list"} {
		if nfRecoveryListTarget(root, "list", filepath.Join(root, n)) {
			t.Fatal("accepted", n)
		}
	}
	if !nfRecoveryListTarget(root, "list", filepath.Join(root, "custom.list")) {
		t.Fatal("custom list blocked")
	}
	if nfRecoveryListTarget(root, "config", filepath.Join(root, "custom.list")) {
		t.Fatal("config accepted")
	}
}
func TestE062NFRecoveryRead(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fixture.list")
	if e := os.WriteFile(path, []byte("example.org\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e := nfRecoveryRead(path, 3); e == nil {
		t.Fatal("oversize accepted")
	}
	link := filepath.Join(root, "link.list")
	if e := os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if _, _, e := nfRecoveryRead(link, 100); e == nil {
		t.Fatal("symlink accepted")
	}
}
