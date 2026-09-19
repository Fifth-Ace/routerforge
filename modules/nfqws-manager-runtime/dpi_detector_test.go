package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDPIDetector(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dpi-detector")
	if err := os.WriteFile(path, []byte("stub"), 0700); err != nil {
		t.Fatal(err)
	}
	got, info := findDPIDetector([]string{path})
	if got != path || info == nil {
		t.Fatalf("findDPIDetector = %q / %#v", got, info)
	}
}

func TestFindDPIDetectorRejectsWrongBasename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not-dpi-detector")
	if err := os.WriteFile(path, []byte("stub"), 0700); err != nil {
		t.Fatal(err)
	}
	got, info := findDPIDetector([]string{path})
	if got != "" || info != nil {
		t.Fatalf("unexpected match = %q / %#v", got, info)
	}
}

func TestFindDPIDetectorRequiresExecutable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dpi-detector")
	if err := os.WriteFile(path, []byte("stub"), 0600); err != nil {
		t.Fatal(err)
	}
	got, info := findDPIDetector([]string{path})
	if got != "" || info != nil {
		t.Fatalf("non-executable matched = %q / %#v", got, info)
	}
}

func TestNormalizeDPIDetectorVersion(t *testing.T) {
	got := normalizeDPIDetectorVersion("\n dpi-detector 5.0.0-alpha.19 \r\nother\n")
	if got != "dpi-detector 5.0.0-alpha.19" {
		t.Fatalf("version = %q", got)
	}
}
