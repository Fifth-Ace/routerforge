package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestP29QReadStableLocalScriptRejectsUnsafeInputs(t *testing.T) {
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty.sh")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readStableLocalScript(empty, 1024); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty script accepted: %v", err)
	}

	nul := filepath.Join(dir, "nul.sh")
	if err := os.WriteFile(nul, []byte("#!/bin/sh\n\x00echo bad\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readStableLocalScript(nul, 1024); err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("NUL script accepted: %v", err)
	}

	large := filepath.Join(dir, "large.sh")
	if err := os.WriteFile(large, []byte("123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readStableLocalScript(large, 8); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversized script accepted: %v", err)
	}
}

func TestP29QReadStableLocalScriptRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.sh")
	link := filepath.Join(dir, "link.sh")
	if err := os.WriteFile(target, []byte("#!/bin/sh\necho ok\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := readStableLocalScript(link, 1024); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink accepted: %v", err)
	}
}

func TestP29QReadStableLocalScriptReturnsSnapshotBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "safe.sh")
	want := []byte("#!/bin/sh\necho stable\n")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readStableLocalScript(path, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("snapshot mismatch: %q != %q", string(got), string(want))
	}
}
