package safety

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwapPathReplacesExistingAndKeepsRollback(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current")
	staged := filepath.Join(dir, "staged")
	rollback := filepath.Join(dir, "rollback")

	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "value"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "value"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := SwapPath(staged, current, rollback)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HadCurrent {
		t.Fatal("existing destination was not recorded")
	}

	got, err := os.ReadFile(filepath.Join(current, "value"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("current=%q want=new", string(got))
	}

	old, err := os.ReadFile(filepath.Join(rollback, "value"))
	if err != nil {
		t.Fatal(err)
	}
	if string(old) != "old" {
		t.Fatalf("rollback=%q want=old", string(old))
	}

	if err := CleanupRollback(rollback, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(rollback); !os.IsNotExist(err) {
		t.Fatalf("rollback path still exists: %v", err)
	}
}

func TestSwapPathPublishesWhenDestinationMissing(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current")
	staged := filepath.Join(dir, "staged")
	rollback := filepath.Join(dir, "rollback")

	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := SwapPath(staged, current, rollback)
	if err != nil {
		t.Fatal(err)
	}
	if result.HadCurrent {
		t.Fatal("missing destination must not report rollback state")
	}
	if _, err := os.Stat(current); err != nil {
		t.Fatal(err)
	}
}

func TestSwapPathRejectsDuplicatePaths(t *testing.T) {
	dir := t.TempDir()
	if _, err := SwapPath(dir, dir, filepath.Join(dir, "rollback")); err == nil {
		t.Fatal("duplicate swap paths must be rejected")
	}
}
