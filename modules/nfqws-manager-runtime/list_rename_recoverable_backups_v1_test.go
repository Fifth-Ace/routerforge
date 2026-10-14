package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Every preimage persisted by rename must have an unambiguous canonical target.
func TestNF5G3BackupLabelsAndRestoreTargets(t *testing.T) {
	lists, active, library, req := nf5g2Fixture(t, true, true)
	ops, _, _ := nf5g2TestOps(t)
	labels := []string{}
	ops.Backup = func(label string, _ []byte, _ os.FileMode) (string, error) {
		labels = append(labels, label)
		kind, target := backupKindTarget(label)
		if !validRestoreTarget(kind, target) {
			t.Fatalf("backup label %s cannot be restored: kind=%s target=%s", label, kind, target)
		}
		return label, nil
	}
	result, err := nf5g2Execute(lists, active, library, req, ops)
	if err != nil || !result.OK {
		t.Fatalf("rename result %+v %v", result, err)
	}
	expected := []string{"list-rename-library-saved.conf", "list-rename-active", "list-rename-source-one.list"}
	if !reflect.DeepEqual(labels, expected) {
		t.Fatalf("backup labels: got=%v expected=%v", labels, expected)
	}
	for _, test := range []struct{ label, kind, target string }{
		{"list-rename-library-saved.conf", "library", filepath.Join(configLibraryRoot, "saved.conf")},
		{"list-rename-active", "config", configPath},
		{"list-rename-source-one.list", "list", filepath.Join(listsRoot, "one.list")},
	} {
		kind, target := backupKindTarget(test.label)
		if kind != test.kind || target != test.target {
			t.Fatalf("misbound %s => %s %s", test.label, kind, target)
		}
	}
}

func TestNF5G3LegacyAmbiguousBackupRemainsUnbound(t *testing.T) {
	kind, path := backupKindTarget("list-rename-config-nfqws2.conf")
	if kind != "unknown" || path != "" {
		t.Fatalf("ambiguous legacy backup was guessed: %s %s", kind, path)
	}
}

func TestNF5G3UnsafeBackupNamesStayUnbound(t *testing.T) {
	for _, bad := range []string{
		"list-rename-source-../escape.list",
		"list-rename-source-.hidden.list",
		"list-rename-library-../escape.conf",
		"list-rename-library-no-extension",
	} {
		kind, path := backupKindTarget(bad)
		if kind != "unknown" || path != "" {
			t.Fatalf("unsafe label bound: %s => %s %s", bad, kind, path)
		}
	}
}
