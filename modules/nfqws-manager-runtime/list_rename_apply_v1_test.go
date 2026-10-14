package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nf5g2Fixture(t *testing.T, activeRef, savedRef bool) (string, string, string, nf5g2Request) {
	t.Helper()
	lists, active, lib, sha := nf5gTestRoots(t, "# active\n")
	if activeRef {
		if err := os.WriteFile(active, []byte("--hostlist="+filepath.Join(lists, "one.list")+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if savedRef {
		if err := os.WriteFile(filepath.Join(lib, "saved.conf"), []byte("--ipset="+filepath.Join(lists, "one.list")+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := nf5gPreviewRenameInRoots(lists, active, lib, "one.list", "two.list", sha)
	if err != nil {
		t.Fatal(err)
	}
	return lists, active, lib, nf5g2Request{OldName: "one.list", NewName: "two.list", ExpectedSHA256: sha, ExpectedChanges: p.ConfigChanges, Confirm: "ROUTERFORGE_LIST_RENAME_APPLY"}
}

func nf5g2TestOps(t *testing.T) (nf5g2Ops, *int, *int) {
	t.Helper()
	b := new(int)
	r := new(int)
	return nf5g2Ops{Backup: func(_ string, _ []byte, _ os.FileMode) (string, error) { *b++; return "backup", nil },
		WriteConfig: func(path string, data []byte, mode os.FileMode) error { return os.WriteFile(path, data, mode) },
		Restart:     func() error { *r++; return nil }, Running: true}, b, r
}

func TestNF5G2ApplyUpdatesActiveSavedAndUnlinksOld(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, true)
	ops, b, r := nf5g2TestOps(t)
	result, err := nf5g2Execute(lists, active, lib, req, ops)
	if err != nil || !result.OK || result.ChangedConfigs != 2 || result.UpdatedSelectors != 2 || !result.ServiceRestarted || *b != 3 || *r != 1 {
		t.Fatalf("result=%+v backups=%d restart=%d err=%v", result, *b, *r, err)
	}
	if _, err := os.Lstat(filepath.Join(lists, "one.list")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("old list still exists")
	}
	newData, _ := os.ReadFile(filepath.Join(lists, "two.list"))
	if string(newData) != "hello.example\n" {
		t.Fatalf("destination=%q", newData)
	}
	for _, p := range []string{active, filepath.Join(lib, "saved.conf")} {
		d, _ := os.ReadFile(p)
		if !strings.Contains(string(d), "two.list") || strings.Contains(string(d), "one.list") {
			t.Fatalf("not renamed: %q", d)
		}
	}
}

func TestNF5G2NoActiveReferencesNeverRestarts(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, false, true)
	ops, _, r := nf5g2TestOps(t)
	res, err := nf5g2Execute(lists, active, lib, req, ops)
	if err != nil || !res.OK || res.ServiceRestarted || *r != 0 {
		t.Fatalf("result=%+v restarts=%d err=%v", res, *r, err)
	}
}

func TestNF5G2UnreferencedRenameCreatesOnlyList(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, false, false)
	ops, b, r := nf5g2TestOps(t)
	res, err := nf5g2Execute(lists, active, lib, req, ops)
	if err != nil || !res.OK || res.ChangedConfigs != 0 || *b != 1 || *r != 0 {
		t.Fatalf("%+v backups=%d restart=%d err=%v", res, *b, *r, err)
	}
}

func TestNF5G2RejectsStalePlanBeforeWrites(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, true)
	if err := os.WriteFile(filepath.Join(lib, "saved.conf"), []byte("# changed\n--ipset="+filepath.Join(lists, "one.list")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ops, b, _ := nf5g2TestOps(t)
	if _, err := nf5g2Execute(lists, active, lib, req, ops); err == nil || *b != 0 {
		t.Fatalf("stale plan accepted, backups=%d err=%v", *b, err)
	}
	if _, err := os.Lstat(filepath.Join(lists, "two.list")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("destination created on stale plan")
	}
}

func TestNF5G2RejectsTamperedPreview(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, false)
	req.ExpectedChanges[0].Selectors = 999
	ops, b, _ := nf5g2TestOps(t)
	if _, err := nf5g2Execute(lists, active, lib, req, ops); err == nil || *b != 0 {
		t.Fatalf("tampered preview accepted: %v", err)
	}
}

func TestNF5G2RollbackSecondConfigWrite(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, true)
	originalActive, _ := os.ReadFile(active)
	originalSaved, _ := os.ReadFile(filepath.Join(lib, "saved.conf"))
	ops, _, r := nf5g2TestOps(t)
	writes := 0
	ops.WriteConfig = func(path string, data []byte, mode os.FileMode) error {
		writes++
		if writes == 2 {
			return errors.New("injected write failure")
		}
		return os.WriteFile(path, data, mode)
	}
	res, err := nf5g2Execute(lists, active, lib, req, ops)
	if err == nil || !res.RolledBack || res.OK || *r != 1 {
		t.Fatalf("%+v restarts=%d err=%v", res, *r, err)
	}
	nowActive, _ := os.ReadFile(active)
	nowSaved, _ := os.ReadFile(filepath.Join(lib, "saved.conf"))
	if string(nowActive) != string(originalActive) || string(nowSaved) != string(originalSaved) {
		t.Fatalf("rollback changed configs active=%q saved=%q", nowActive, nowSaved)
	}
	if _, err := os.Stat(filepath.Join(lists, "two.list")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("rollback retained destination")
	}
	if _, err := os.Stat(filepath.Join(lists, "one.list")); err != nil {
		t.Fatal("rollback removed source")
	}
}

func TestNF5G2RollbackRestartFailure(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, false)
	old, _ := os.ReadFile(active)
	ops, _, r := nf5g2TestOps(t)
	ops.Restart = func() error {
		*r++
		if *r == 1 {
			return errors.New("service restart failed")
		}
		return nil
	}
	res, err := nf5g2Execute(lists, active, lib, req, ops)
	if err == nil || !res.RolledBack || *r != 2 {
		t.Fatalf("%+v err=%v restarts=%d", res, err, *r)
	}
	now, _ := os.ReadFile(active)
	if string(now) != string(old) {
		t.Fatalf("wrong active config %q", now)
	}
	if _, err := os.Stat(filepath.Join(lists, "two.list")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("restart failure retained destination")
	}
}

func TestNF5G2BackupFailureLeavesFilesUntouched(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, true, false)
	ops, _, _ := nf5g2TestOps(t)
	ops.Backup = func(string, []byte, os.FileMode) (string, error) { return "", errors.New("backup unavailable") }
	if _, err := nf5g2Execute(lists, active, lib, req, ops); err == nil {
		t.Fatal("no backup accepted")
	}
	if _, err := os.Stat(filepath.Join(lists, "one.list")); err != nil {
		t.Fatal("old list missing")
	}
	if _, err := os.Stat(filepath.Join(lists, "two.list")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("destination created")
	}
}

func TestNF5G2DestinationRaceNeverOverwrites(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, false, false)
	ops, _, _ := nf5g2TestOps(t)
	ops.Backup = func(string, []byte, os.FileMode) (string, error) {
		// A competing actor wins just after preview but before O_EXCL.
		_ = os.WriteFile(filepath.Join(lists, "two.list"), []byte("manual.example\n"), 0600)
		return "backup", nil
	}
	if _, err := nf5g2Execute(lists, active, lib, req, ops); err == nil {
		t.Fatal("destination race accepted")
	}
	data, _ := os.ReadFile(filepath.Join(lists, "two.list"))
	if string(data) != "manual.example\n" {
		t.Fatal("manual writer overwritten")
	}
}

func TestNF5G2RejectsUnsafeNamesAndMissingConfirm(t *testing.T) {
	lists, active, lib, req := nf5g2Fixture(t, false, false)
	ops, _, _ := nf5g2TestOps(t)
	req.Confirm = ""
	if _, err := nf5g2Execute(lists, active, lib, req, ops); err == nil {
		t.Fatal("missing confirmation accepted")
	}
	req.Confirm = "ROUTERFORGE_LIST_RENAME_APPLY"
	req.NewName = "../escape.list"
	if _, err := nf5g2Execute(lists, active, lib, req, ops); err == nil {
		t.Fatal("unsafe new name accepted")
	}
}

func TestNF5G2ApplyRouteIsMutationGuarded(t *testing.T) {
	data, err := os.ReadFile("list_rename_apply_v1.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `mutationOnly(handleNF5G2RenameApply)`) {
		t.Fatal("apply route must be mutationOnly")
	}
}
