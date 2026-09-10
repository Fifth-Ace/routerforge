package main

import "testing"

func TestValidAdminSnapshotName(t *testing.T) {
	if !validAdminSnapshotName("routerforge-snapshot-123.json") {
		t.Fatal("valid snapshot name rejected")
	}
	for _, name := range []string{
		"snapshot-123.json",
		"routerforge-snapshot-123.tar.gz",
		"../routerforge-snapshot-123.json",
	} {
		if validAdminSnapshotName(name) {
			t.Fatalf("invalid snapshot name accepted: %q", name)
		}
	}
}
