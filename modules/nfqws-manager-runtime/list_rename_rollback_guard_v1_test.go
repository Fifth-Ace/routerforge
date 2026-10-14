package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Simulate an outside editor changing the active file after the rename
// attempted to write it. A rollback conflict must retain the new list.
func TestNF5G4ConflictedActiveRollbackRetainsDestination(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, true)
	ops, _, restarts := nf5g2TestOps(t)
	ops.WriteConfig = func(path string, data []byte, mode os.FileMode) error {
		if path == active && strings.Contains(string(data), "two.list") {
			changed := []byte("--hostlist=" + filepath.Join(lists, "two.list") + "\n# external edit\n")
			if err := os.WriteFile(path, changed, mode); err != nil {
				return err
			}
			return errors.New("injected active-write failure after external edit")
		}
		return os.WriteFile(path, data, mode)
	}
	out, err := nf5g2Execute(lists, active, lib, req, ops)
	if err == nil || out.RolledBack || out.OK || *restarts != 0 {
		t.Fatalf("result=%+v restarts=%d err=%v", out, *restarts, err)
	}
	if _, err := os.Stat(filepath.Join(lists, "two.list")); err != nil {
		t.Fatal("created list removed despite incomplete rollback", err)
	}
	if _, err := os.Stat(filepath.Join(lists, "one.list")); err != nil {
		t.Fatal("original list missing", err)
	}
	now, _ := os.ReadFile(active)
	if !strings.Contains(string(now), "two.list") || !strings.Contains(out.RollbackError, "external config modification") {
		t.Fatalf("unexpected conflict state %q %+v", now, out)
	}
}

// If an unrelated process creates a new config referencing the destination
// after the preview, cleanup must not unlink that destination.
func TestNF5G4NewSavedReferenceRetainsDestination(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, false)
	ops, _, restarts := nf5g2TestOps(t)
	ops.WriteConfig = func(path string, data []byte, mode os.FileMode) error {
		if path == active && strings.Contains(string(data), "two.list") {
			newConfig := filepath.Join(lib, "external.conf")
			if err := os.WriteFile(newConfig, []byte("--ipset="+filepath.Join(lists, "two.list")+"\n"), 0600); err != nil {
				return err
			}
			return errors.New("injected write failure")
		}
		return os.WriteFile(path, data, mode)
	}
	out, err := nf5g2Execute(lists, active, lib, req, ops)
	if err == nil || out.RolledBack || *restarts != 1 || !strings.Contains(out.RollbackError, "still references new list") {
		t.Fatalf("out=%+v restarts=%d err=%v", out, *restarts, err)
	}
	if _, err := os.Stat(filepath.Join(lists, "two.list")); err != nil {
		t.Fatal("new reference orphaned", err)
	}
}

func TestNF5G4CleanRollbackStillRemovesDestination(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, true)
	ops, _, restarts := nf5g2TestOps(t)
	ops.WriteConfig = func(path string, data []byte, mode os.FileMode) error {
		if path == active && strings.Contains(string(data), "two.list") {
			return errors.New("injected clean write failure")
		}
		return os.WriteFile(path, data, mode)
	}
	out, err := nf5g2Execute(lists, active, lib, req, ops)
	if err == nil || !out.RolledBack || *restarts != 1 {
		t.Fatalf("out=%+v restarts=%d err=%v", out, *restarts, err)
	}
	if _, err := os.Stat(filepath.Join(lists, "two.list")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clean rollback retained list: %v", err)
	}
}

func TestNF5G4RestartRollbackFailureRetainsDestination(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, false)
	ops, _, restarts := nf5g2TestOps(t)
	ops.Restart = func() error { *restarts++; return errors.New("injected restart failure") }
	out, err := nf5g2Execute(lists, active, lib, req, ops)
	if err == nil || out.RolledBack || *restarts != 2 {
		t.Fatalf("out=%+v restarts=%d err=%v", out, *restarts, err)
	}
	if _, err := os.Stat(filepath.Join(lists, "two.list")); err != nil {
		t.Fatalf("restart uncertain, list removed: %v", err)
	}
}

func TestNF5G4RejectsUnreadableSavedConfigsDuringCleanup(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, false)
	ops, _, _ := nf5g2TestOps(t)
	ops.WriteConfig = func(path string, data []byte, mode os.FileMode) error {
		if path == active && strings.Contains(string(data), "two.list") {
			if err := os.Mkdir(filepath.Join(lib, "unreadable.conf"), 0700); err != nil {
				return err
			}
			return errors.New("injected failure with unsafe config")
		}
		return os.WriteFile(path, data, mode)
	}
	out, err := nf5g2Execute(lists, active, lib, req, ops)
	if err == nil || out.RolledBack || !strings.Contains(out.RollbackError, "inspect saved config") {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if _, err := os.Stat(filepath.Join(lists, "two.list")); err != nil {
		t.Fatalf("failed scan removed list: %v", err)
	}
}
