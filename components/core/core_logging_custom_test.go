package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestP57CustomLogPathSafety(t *testing.T) {
	valid := []string{"/opt/var/log/routerforge-custom.log", "/tmp/rf/custom.log", "/mnt/usb/rf.log", "/storage/logs/rf.log"}
	for _, path := range valid {
		if !coreCustomPathValid(path) {
			t.Fatalf("valid path rejected: %q", path)
		}
	}
	invalid := []string{"", "../log.log", "/etc/passwd", "/opt/etc/routerforge/logging.json", "/proc/1/mem", "/mnt/foo/../../etc/x.log", "/tmp/bad.txt", "/tmp/a\n.log", "/"}
	for _, path := range invalid {
		if coreCustomPathValid(path) {
			t.Fatalf("unsafe custom path accepted: %q", path)
		}
	}
}

func TestP57CustomLogLocationAndRoundtrip(t *testing.T) {
	// t.TempDir lives under /tmp in Linux CI, so it is a permitted destination.
	dir := t.TempDir()
	if !strings.HasPrefix(dir, "/tmp/") {
		t.Skip("temporary directory not under /tmp")
	}
	path := filepath.Join(dir, "custom.log")
	s := defaultCoreLoggingSettings()
	s.Mode = "custom"
	s.CustomPath = path
	if err := s.validate(); err != nil {
		t.Fatal(err)
	}
	got, err := resolveCoreLogPath(s, "/opt/var/log/routerforge.log", false)
	if err != nil || got != path {
		t.Fatalf("custom path = %q, err = %v", got, err)
	}
	if err := checkCoreCustomLogLocation(path, true); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("probe leaked entries: %v, %v", entries, err)
	}
	// Never follow a preexisting log symlink.
	target := filepath.Join(dir, "sensitive")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if err := checkCoreCustomLogLocation(path, false); err == nil {
		t.Fatal("custom log symlink accepted")
	}
	redirect := filepath.Join(dir, "redirect")
	if err := os.Symlink("/etc", redirect); err != nil {
		t.Fatal(err)
	}
	if err := checkCoreCustomLogLocation(filepath.Join(redirect, "should-not-write.log"), false); err == nil {
		t.Fatal("parent symlink outside log storage roots accepted")
	}
	s.Mode = "entware"
	if err := s.validate(); err != nil {
		t.Fatalf("stored custom path rejected in entware mode: %v", err)
	}
}

func TestP57LegacySettingsRemainValid(t *testing.T) {
	s := defaultCoreLoggingSettings()
	if err := s.validate(); err != nil {
		t.Fatal(err)
	}
	s.Mode = "ram"
	if err := s.validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveCoreLogPath(s, "original", false); err == nil {
		t.Fatal("RAM without tmpfs accepted")
	}
	s.Mode = "custom"
	if err := s.validate(); err == nil {
		t.Fatal("custom without path accepted")
	}
}
