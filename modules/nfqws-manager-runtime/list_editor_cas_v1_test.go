package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNF5AExpectedSHAFromExactRead(t *testing.T) {
	original := []byte("# a\nexample.com\n")
	if err := nf5aRequireListPreimage(original, listSourceSHA256(original)); err != nil {
		t.Fatal(err)
	}
}

func TestNF5ARejectsStaleListHash(t *testing.T) {
	original := []byte("example.com\n")
	next := []byte("example.com\nnew.example\n")
	if err := nf5aRequireListPreimage(next, listSourceSHA256(original)); err == nil {
		t.Fatal("changed list accepted a stale SHA")
	}
}

func TestNF5ARejectsMissingAndInvalidHash(t *testing.T) {
	for _, expected := range []string{"", "hash", strings.Repeat("x", 64)} {
		if err := nf5aRequireListPreimage([]byte("data"), expected); err == nil {
			t.Fatalf("bad expected sha was accepted: %q", expected)
		}
	}
}

func TestNF5AReadRegularFileAndPreserveMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "valid.list")
	if err := os.WriteFile(path, []byte("example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	data, info, err := nf5aReadRegularList(path)
	if err != nil || string(data) != "example.com\n" || !info.Mode().IsRegular() {
		t.Fatalf("read=%s info=%v err=%v", data, info, err)
	}
}

func TestNF5ARejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "unsafe.list")
	if err := os.WriteFile(target, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, _, err := nf5aReadRegularList(link); err == nil {
		t.Fatal("symlink followed")
	}
}

func TestNF5ARejectsDirectory(t *testing.T) {
	if _, _, err := nf5aReadRegularList(t.TempDir()); err == nil {
		t.Fatal("directory was accepted")
	}
}

func TestNF5ARejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.list")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(listFileMaxBytes + 1); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()
	if _, _, err := nf5aReadRegularList(path); err == nil {
		t.Fatal("oversized list accepted")
	}
}

func TestNF5AEmptyListCanBeEditedWithExactHash(t *testing.T) {
	data := []byte{}
	if err := nf5aRequireListPreimage(data, listSourceSHA256(data)); err != nil {
		t.Fatal(err)
	}
}

func TestNF5AChangedFileDetectedOnSecondRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "updated.list")
	before := []byte("old.example\n")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	sha := listSourceSHA256(before)
	if err := os.WriteFile(path, []byte("new.example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	current, _, err := nf5aReadRegularList(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := nf5aRequireListPreimage(current, sha); err == nil {
		t.Fatal("modified file was accepted")
	}
}
