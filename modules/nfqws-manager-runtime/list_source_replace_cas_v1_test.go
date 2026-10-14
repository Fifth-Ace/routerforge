package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNF5DReplaceAllowsExactPreimage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one.list")
	data := []byte("one.example\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := nf5dReplacementPreimage(path, listSourceSHA256(data))
	if err != nil || string(got) != string(data) {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestNF5DReplaceRejectsModifiedList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one.list")
	original := []byte("one.example\n")
	if err := os.WriteFile(path, []byte("manual-edit.example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := nf5dReplacementPreimage(path, listSourceSHA256(original)); err == nil {
		t.Fatal("stale inventory was authorized")
	}
}

func TestNF5DReplaceRejectsNoOldSHA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one.list")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"", "bad", "000000"} {
		if _, err := nf5dReplacementPreimage(path, want); err == nil {
			t.Fatalf("bad SHA %q accepted", want)
		}
	}
}

func TestNF5DReplaceRejectsMissingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.list")
	if _, err := nf5dReplacementPreimage(path, listSourceSHA256(nil)); err == nil {
		t.Fatal("replace unexpectedly created an absent list")
	}
}

func TestNF5DReplaceRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	path := filepath.Join(root, "alias.list")
	data := []byte("x")
	if err := os.WriteFile(target, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if _, err := nf5dReplacementPreimage(path, listSourceSHA256(data)); err == nil {
		t.Fatal("replacement followed symlink")
	}
}

func TestNF5DReplaceRejectsDirectory(t *testing.T) {
	if _, err := nf5dReplacementPreimage(t.TempDir(), listSourceSHA256(nil)); err == nil {
		t.Fatal("directory accepted as a list")
	}
}

func TestNF5DReplaceRejectsOversizedList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge.list")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(listFileMaxBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := nf5dReplacementPreimage(path, listSourceSHA256(nil)); err == nil {
		t.Fatal("oversized list accepted")
	}
}
