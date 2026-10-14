package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNF5G5NewSavedReferenceDuringRestartAbortsRemoval(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, false)
	ops, _, calls := nf5g2TestOps(t)
	ops.Restart = func() error {
		*calls++
		if *calls == 1 {
			return os.WriteFile(filepath.Join(lib, "external.conf"), []byte("--ipset="+filepath.Join(lists, req.OldName)+"\n"), 0600)
		}
		return nil
	}
	result, err := nf5g2Execute(lists, active, lib, req, ops)
	if err == nil || result.OK || !strings.Contains(err.Error(), "still references original list") {
		t.Fatalf("unsafe rename success: result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(lists, req.OldName)); err != nil {
		t.Fatal("old file removed", err)
	}
	if _, err := os.Lstat(filepath.Join(lists, req.NewName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clean rollback kept new file: %v", err)
	}
	if *calls != 2 || !result.RolledBack {
		t.Fatalf("unexpected rollback: %+v restart=%d", result, *calls)
	}
	original, _ := os.ReadFile(active)
	if strings.Contains(string(original), req.NewName) {
		t.Fatalf("active config was not rolled back: %q", original)
	}
}

func TestNF5G5ExternalActiveReversionAfterRestartFailsClosed(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, false)
	ops, _, calls := nf5g2TestOps(t)
	ops.Restart = func() error {
		*calls++
		if *calls == 1 {
			return os.WriteFile(active, []byte("--hostlist="+filepath.Join(lists, req.OldName)+"\n# external change\n"), 0600)
		}
		return nil
	}
	result, err := nf5g2Execute(lists, active, lib, req, ops)
	if err == nil || result.OK || result.RolledBack {
		t.Fatalf("external active change accepted: %+v err=%v", result, err)
	}
	if _, e := os.Lstat(filepath.Join(lists, req.OldName)); e != nil {
		t.Fatal("old file removed", e)
	}
	if _, e := os.Lstat(filepath.Join(lists, req.NewName)); e != nil {
		t.Fatal("new file should be retained on partial rollback", e)
	}
}

func TestNF5G5CommentMentionBlocksDeletionConservatively(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, false)
	ops, _, _ := nf5g2TestOps(t)
	ops.Restart = func() error {
		return os.WriteFile(filepath.Join(lib, "added.conf"), []byte("# old reference to "+req.OldName+"\n"), 0600)
	}
	result, err := nf5g2Execute(lists, active, lib, req, ops)
	if err == nil || result.OK || !strings.Contains(err.Error(), "still references original list") {
		t.Fatalf("ambiguous reference missed %+v %v", result, err)
	}
	if _, e := os.Stat(filepath.Join(lists, req.OldName)); e != nil {
		t.Fatal("old list disappeared", e)
	}
}

func TestNF5G5UnreadableLateSavedConfigFailsClosed(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, false)
	ops, _, _ := nf5g2TestOps(t)
	ops.Restart = func() error { return os.Mkdir(filepath.Join(lib, "unknown.conf"), 0700) }
	result, err := nf5g2Execute(lists, active, lib, req, ops)
	if err == nil || result.OK || result.RolledBack {
		t.Fatalf("unexpected success for unreadable saved config %+v %v", result, err)
	}
	if _, e := os.Stat(filepath.Join(lists, req.OldName)); e != nil {
		t.Fatal("old removed", e)
	}
	if _, e := os.Stat(filepath.Join(lists, req.NewName)); e != nil {
		t.Fatal("new must remain on incomplete rollback", e)
	}
}

func TestNF5G5SuccessNoReferences(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, true)
	ops, _, _ := nf5g2TestOps(t)
	result, err := nf5g2Execute(lists, active, lib, req, ops)
	if err != nil || !result.OK {
		t.Fatalf("safe rename failed: %+v %v", result, err)
	}
	if _, e := os.Lstat(filepath.Join(lists, req.OldName)); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("old list retained", e)
	}
	if _, e := os.Stat(filepath.Join(lists, req.NewName)); e != nil {
		t.Fatal("new list missing", e)
	}
}

func TestNF5G5RejectUnsafeName(t *testing.T) {
	_, active, lib, _ := nf5g2Fixture(t, false, false)
	if err := nf5g5AllowOldSourceRemoval(active, lib, "../unsafe.list", nil); err == nil {
		t.Fatal("unsafe list basename allowed")
	}
}

func TestNF5G5FinalGateCalledBeforeUnlink(t *testing.T) {
	data, err := os.ReadFile("list_rename_apply_v1.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	gate := strings.Index(s, "nf5g5AllowOldSourceRemoval(activePath, libRoot, req.OldName, snaps)")
	unlink := strings.Index(s, "os.Remove(oldPath)")
	if gate < 0 || unlink <= gate {
		t.Fatalf("final guard is not before removal gate=%d unlink=%d", gate, unlink)
	}
}
