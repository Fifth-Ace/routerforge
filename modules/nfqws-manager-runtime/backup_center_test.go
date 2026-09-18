package main

import (
	"path/filepath"
	"testing"
)

func TestSafeBackupID(t *testing.T) {
	good := []string{"123-config-acde1234", "backup_1", "a.b-c"}
	for _, id := range good {
		if !safeBackupID(id) {
			t.Fatalf("expected safe backup id: %s", id)
		}
	}
	bad := []string{"", ".hidden", "../escape", "nested/id", "bad id"}
	for _, id := range bad {
		if safeBackupID(id) {
			t.Fatalf("expected unsafe backup id: %s", id)
		}
	}
}

func TestBackupKindTarget(t *testing.T) {
	cases := []struct {
		label string
		kind  string
		base  string
	}{
		{"config", "config", "nfqws2.conf"},
		{"smart-apply-config", "config", "nfqws2.conf"},
		{"config-switch-alt.conf", "config", "nfqws2.conf"},
		{"list-user.list", "list", "user.list"},
		{"list-source-youtube.list", "list", "youtube.list"},
		{"smart-apply-list-google.list", "list", "google.list"},
		{"deleted-extra.list", "list", "extra.list"},
		{"blob-tls.bin", "blob", "tls.bin"},
		{"blob-delete-quic.bin", "blob", "quic.bin"},
		{"smart-apply-blob-stun.bin", "blob", "stun.bin"},
		{"library-alt.conf", "library", "alt.conf"},
		{"library-delete-old.conf", "library", "old.conf"},
	}
	for _, tc := range cases {
		kind, target := backupKindTarget(tc.label)
		if kind != tc.kind {
			t.Fatalf("%s: expected kind %s, got %s", tc.label, tc.kind, kind)
		}
		if filepath.Base(target) != tc.base {
			t.Fatalf("%s: expected target base %s, got %s", tc.label, tc.base, filepath.Base(target))
		}
	}
}

func TestValidRestoreTarget(t *testing.T) {
	if !validRestoreTarget("config", configPath) {
		t.Fatal("canonical config should be restorable")
	}
	if !validRestoreTarget("list", filepath.Join(listsRoot, "user.list")) {
		t.Fatal("safe list target should be restorable")
	}
	if !validRestoreTarget("blob", filepath.Join(blobRoot, "tls.bin")) {
		t.Fatal("safe blob target should be restorable")
	}
	if !validRestoreTarget("library", filepath.Join(configLibraryRoot, "alt.conf")) {
		t.Fatal("safe library target should be restorable")
	}
	if validRestoreTarget("list", filepath.Join(listsRoot, "../escape.list")) {
		t.Fatal("escaped list target must not be restorable")
	}
	if validRestoreTarget("blob", filepath.Join(blobRoot, "bad.txt")) {
		t.Fatal("non-bin blob target must not be restorable")
	}
	if validRestoreTarget("unknown", configPath) {
		t.Fatal("unknown kind must not be restorable")
	}
}
