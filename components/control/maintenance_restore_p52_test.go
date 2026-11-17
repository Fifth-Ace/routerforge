package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestP52RestoreVaultCapacityFallsBackToPortableBackup(t *testing.T) {
	root := t.TempDir()
	needed, err := adminMaintenanceNeedsBackupOnly(root)
	if err != nil || needed {
		t.Fatalf("empty root must fit vault: %v %v", needed, err)
	}
	if err := os.WriteFile(filepath.Join(root, "large-config"), make([]byte, 2<<20+1), 0600); err != nil {
		t.Fatal(err)
	}
	needed, err = adminMaintenanceNeedsBackupOnly(root)
	if err != nil || !needed {
		t.Fatalf("oversized file must request portable backup fallback: %v %v", needed, err)
	}
}
