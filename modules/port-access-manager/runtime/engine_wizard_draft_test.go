package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWizardDraftStoreAndOptimisticLock(t *testing.T) {
	dir := t.TempDir()
	req := exampleWizard()
	sha, err := storeWizardDraft(dir, req, "")
	if err != nil {
		t.Fatal(err)
	}
	if sha == "" {
		t.Fatal("empty SHA")
	}
	stored, got, err := readWizardDraft(dir, req.Engine)
	if err != nil {
		t.Fatal(err)
	}
	if got != sha || stored.Engine != req.Engine {
		t.Fatal("readback mismatch")
	}
	if _, err := storeWizardDraft(dir, req, ""); err == nil {
		t.Fatal("overwrite without SHA")
	}
	req.AccessSeconds++
	sha2, err := storeWizardDraft(dir, req, sha)
	if err != nil {
		t.Fatal(err)
	}
	if sha == sha2 {
		t.Fatal("digest unchanged")
	}
	if _, err := storeWizardDraft(dir, req, sha); err == nil {
		t.Fatal("stale update accepted")
	}
	if _, err := storeWizardDraft(dir, req, sha2); err != nil {
		t.Fatal(err)
	}
}
func TestWizardDraftRejectsBadEngineAndSymlink(t *testing.T) {
	dir := t.TempDir()
	if _, err := wizardDraftPath(dir, "../../odin"); err == nil {
		t.Fatal("engine traversal")
	}
	req := exampleWizard()
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, req.Engine+".json")); err != nil {
		t.Fatal(err)
	}
	if _, err := storeWizardDraft(dir, req, ""); err == nil {
		t.Fatal("symlink allowed")
	}
}
