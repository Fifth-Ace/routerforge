package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/configvault"
)

const recOld = "{\"mode\":\"entware\",\"max_mb\":1,\"backups\":2}\n"
const recNew = "{\"mode\":\"entware\",\"max_mb\":4,\"backups\":3}\n"

func TestRecoveryValidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "logging.json")
	if !recoveryAllowed(root, path, []byte(recOld), 0600) {
		t.Fatal("valid standard settings blocked")
	}
	for _, data := range []string{`{"mode":"custom","custom_path":"/tmp/x.log","max_mb":1,"backups":2}`, `{"mode":"ram","max_mb":1,"backups":2}`, `{"mode":"entware","max_mb":0,"backups":2}`, `{"mode":"entware","Mode":"entware","max_mb":1,"backups":2}`, `{"mode":"entware","mode":"entware","max_mb":1,"backups":2}`, recOld + `{}`, `{"mode":"entware","max_mb":1,"backups":2,"other":true}`} {
		if recoveryAllowed(root, path, []byte(data), 0600) {
			t.Fatalf("accepted %s", data)
		}
	}
	if recoveryAllowed(root, filepath.Join(root, "policy-catalog-v1.json"), []byte(recOld), 0600) || recoveryAllowed(root, path, []byte(recOld), 0644) {
		t.Fatal("unsafe path or mode accepted")
	}
}
func TestRecoverySelectiveRestoreAndRollback(t *testing.T) {
	root := filepath.Join(t.TempDir(), "managed")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	v, e := configvault.New(configvault.Options{Root: filepath.Join(t.TempDir(), "vault"), AllowedRoots: []string{root}})
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "logging.json")
	other := filepath.Join(root, "observer-v1.id")
	write := func(p, s string) {
		t.Helper()
		if e := os.WriteFile(p, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	read := func(p string) string {
		t.Helper()
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	write(path, recOld)
	write(other, "original observer")
	m, e := v.Capture(configvault.CaptureRequest{Component: "admin", Artifacts: []configvault.ArtifactSpec{{ID: "log", Path: path}, {ID: "observer", Path: other}}, NoPrune: true})
	if e != nil {
		t.Fatal(e)
	}
	write(path, recNew)
	write(other, "new observer")
	p, _, e := recoveryInspect(v, root, m.ID, "log")
	if e != nil || !p.Item.RestoreAllowed || p.Token == "" {
		t.Fatalf("preview %+v %v", p, e)
	}
	blocked, _, e := recoveryInspect(v, root, m.ID, "observer")
	if e != nil || blocked.Item.RestoreAllowed {
		t.Fatalf("protected %+v %v", blocked, e)
	}
	q := recoveryRequest{SnapshotID: m.ID, ArtifactID: "log", Token: p.Token}
	write(path, recOld)
	if _, e = recoveryApply(v, root, q); e == nil {
		t.Fatal("stale preview accepted")
	}
	write(path, recNew)
	if read(path) != recNew || read(other) != "new observer" {
		t.Fatal("preview mutated")
	}
	out, e := recoveryApply(v, root, q)
	if e != nil || !out.OK || !out.ReadbackVerified || out.SafetySnapshot == "" {
		t.Fatalf("restore %+v %v", out, e)
	}
	if read(path) != recOld || read(other) != "new observer" {
		t.Fatal("restore scope")
	}
	rollback, _, e := recoveryInspect(v, root, out.SafetySnapshot, "log")
	if e != nil || !rollback.Item.RestoreAllowed {
		t.Fatalf("rollback preview %+v %v", rollback, e)
	}
	done, e := recoveryApply(v, root, recoveryRequest{SnapshotID: out.SafetySnapshot, ArtifactID: "log", Token: rollback.Token})
	if e != nil || !done.OK || read(path) != recNew || read(other) != "new observer" {
		t.Fatalf("rollback %+v %v", done, e)
	}
	if _, e = v.Get(m.ID); e != nil {
		t.Fatal("original snapshot lost")
	}
}
func TestRecoveryNoPrune(t *testing.T) {
	root := filepath.Join(t.TempDir(), "managed")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "logging.json")
	if e := os.WriteFile(path, []byte(recOld), 0600); e != nil {
		t.Fatal(e)
	}
	v, e := configvault.New(configvault.Options{Root: filepath.Join(t.TempDir(), "vault"), AllowedRoots: []string{root}, MaxSnapshots: 1})
	if e != nil {
		t.Fatal(e)
	}
	req := configvault.CaptureRequest{Component: "admin", Artifacts: []configvault.ArtifactSpec{{ID: "log", Path: path}}, NoPrune: true, Version: "test"}
	first, e := v.Capture(req)
	if e != nil || first.Version != "test" {
		t.Fatalf("capture %v", e)
	}
	if e = os.WriteFile(path, []byte(recNew), 0600); e != nil {
		t.Fatal(e)
	}
	p, _, e := recoveryInspect(v, root, first.ID, "log")
	if e != nil || !p.Item.RestoreAllowed {
		t.Fatalf("plan %v", e)
	}
	if _, e = recoveryApply(v, root, recoveryRequest{SnapshotID: first.ID, ArtifactID: "log", Token: p.Token}); e == nil {
		t.Fatal("restore without safety capacity")
	}
	b, e := os.ReadFile(path)
	if e != nil || string(b) != recNew {
		t.Fatal("current changed")
	}
	list, e := v.List()
	if e != nil || len(list) != 1 || list[0].ID != first.ID {
		t.Fatal("retention removed snapshot")
	}
}
func TestRecoverySymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside")
	if e := os.WriteFile(target, []byte(recOld), 0600); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "logging.json")
	if e := os.Symlink(target, path); e != nil {
		t.Skipf("symlink unavailable: %v", e)
	}
	if _, _, e := recoveryRead(root, path); e == nil {
		t.Fatal("followed symlink")
	}
}

func TestRecoveryCorruptObjectAndMissingTarget(t *testing.T) {
	root := filepath.Join(t.TempDir(), "managed")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "logging.json")
	if e := os.WriteFile(path, []byte(recOld), 0600); e != nil {
		t.Fatal(e)
	}
	vaultRoot := filepath.Join(t.TempDir(), "vault")
	v, e := configvault.New(configvault.Options{Root: vaultRoot, AllowedRoots: []string{root}})
	if e != nil {
		t.Fatal(e)
	}
	m, e := v.Capture(configvault.CaptureRequest{Component: "admin", Artifacts: []configvault.ArtifactSpec{{ID: "log", Path: path}}, NoPrune: true})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(path); e != nil {
		t.Fatal(e)
	}
	p, _, e := recoveryInspect(v, root, m.ID, "log")
	if e != nil || p.Item.Status != "missing" || p.Item.RestoreAllowed || p.Token != "" {
		t.Fatalf("missing target: %+v %v", p, e)
	}
	object := filepath.Join(vaultRoot, "objects", m.Artifacts[0].SHA256)
	if e = os.WriteFile(object, []byte(recNew), 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e = recoveryInspect(v, root, m.ID, "log"); e == nil {
		t.Fatal("corrupt object accepted")
	}
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("target created")
	}
}
