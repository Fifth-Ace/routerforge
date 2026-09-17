package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyAdminDirectoryTreeCopiesNestedFiles(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	staging := filepath.Join(t.TempDir(), "staging")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0750); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.Mkdir(staging, 0700); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "root.txt"), []byte("root"), 0640); err != nil {
		t.Fatalf("write root file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "child.txt"), []byte("child"), 0600); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	snapshots, totalBytes, err := copyAdminDirectoryTree(source, staging)
	if err != nil {
		t.Fatalf("copy tree: %v", err)
	}
	if len(snapshots) != 4 {
		t.Fatalf("snapshot count=%d want 4", len(snapshots))
	}
	if totalBytes != int64(len("root")+len("child")) {
		t.Fatalf("bytes=%d", totalBytes)
	}
	if err := verifyAdminDirectorySource(source, snapshots); err != nil {
		t.Fatalf("verify source: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(staging, "nested", "child.txt"))
	if err != nil {
		t.Fatalf("read copied child: %v", err)
	}
	if string(got) != "child" {
		t.Fatalf("child content=%q", string(got))
	}

	entries, measuredBytes, err := measureAdminDirectoryTree(staging)
	if err != nil {
		t.Fatalf("measure copied tree: %v", err)
	}
	if entries != len(snapshots) || measuredBytes != totalBytes {
		t.Fatalf("measure entries=%d bytes=%d want entries=%d bytes=%d", entries, measuredBytes, len(snapshots), totalBytes)
	}
}

func TestCopyAdminDirectoryTreeRejectsSymlink(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	staging := filepath.Join(t.TempDir(), "staging")
	if err := os.Mkdir(source, 0755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.Mkdir(staging, 0700); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "target.txt"), []byte("target"), 0644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(filepath.Join(source, "target.txt"), filepath.Join(source, "link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if _, _, err := copyAdminDirectoryTree(source, staging); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestVerifyAdminDirectorySourceDetectsMutation(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	staging := filepath.Join(t.TempDir(), "staging")
	if err := os.Mkdir(source, 0755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.Mkdir(staging, 0700); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}
	path := filepath.Join(source, "file.txt")
	if err := os.WriteFile(path, []byte("one"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	snapshots, _, err := copyAdminDirectoryTree(source, staging)
	if err != nil {
		t.Fatalf("copy tree: %v", err)
	}
	if err := os.WriteFile(path, []byte("changed-content"), 0644); err != nil {
		t.Fatalf("mutate source: %v", err)
	}
	if err := verifyAdminDirectorySource(source, snapshots); err == nil {
		t.Fatal("expected source mutation detection")
	}
}
