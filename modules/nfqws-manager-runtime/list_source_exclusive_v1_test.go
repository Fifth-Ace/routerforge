package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestNF5EExclusiveNewSourceWritesExactPayload(t *testing.T) {
	root := t.TempDir()
	payload := []byte("# source\nexample.com\nsub.example.com\n")
	if err := nf5eInstallNewSource(root, "new.list", payload); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "new.list"))
	if err != nil || string(got) != string(payload) {
		t.Fatalf("got %q, err %v", got, err)
	}
}

func TestNF5EExistingSourceNeverOverwritten(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "existing.list")
	old := []byte("manual.example\n")
	if err := os.WriteFile(path, old, 0600); err != nil {
		t.Fatal(err)
	}
	if err := nf5eInstallNewSource(root, "existing.list", []byte("incoming.example\n")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected ErrExist, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(old) {
		t.Fatalf("old file modified: %q %v", got, err)
	}
}

func TestNF5EConcurrentOnlyOneWriter(t *testing.T) {
	root := t.TempDir()
	const workers = 32
	var wg sync.WaitGroup
	var successes int32
	var rejected int32
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := nf5eInstallNewSource(root, "race.list", []byte("example.com\n"))
			if err == nil {
				atomic.AddInt32(&successes, 1)
			} else if errors.Is(err, os.ErrExist) {
				atomic.AddInt32(&rejected, 1)
			} else {
				t.Errorf("unexpected concurrent failure: %v", err)
			}
		}()
	}
	wg.Wait()
	if successes != 1 || rejected != workers-1 {
		t.Fatalf("success=%d conflict=%d", successes, rejected)
	}
	data, err := os.ReadFile(filepath.Join(root, "race.list"))
	if err != nil || string(data) != "example.com\n" {
		t.Fatalf("race output incomplete: %q %v", data, err)
	}
}

func TestNF5EExistingSymlinkIsNotFollowed(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "link.list")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := nf5eInstallNewSource(root, "link.list", []byte("new.example\n")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing symlink should block create: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "untouched" {
		t.Fatal("symlink target overwritten")
	}
}

func TestNF5ERejectsRootSymlink(t *testing.T) {
	parent := t.TempDir()
	actual := filepath.Join(parent, "actual")
	if err := os.Mkdir(actual, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(actual, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := nf5eInstallNewSource(link, "new.list", []byte("new.example\n")); err == nil {
		t.Fatal("root symlink accepted")
	}
}

func TestNF5ERejectsUnsafeNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../out.list", "x/list", ".hidden.list", "x.list-old", "x.list-opkg", "no-extension"} {
		if err := nf5eInstallNewSource(root, name, []byte("example.com\n")); err == nil {
			t.Errorf("accepted unsafe name %q", name)
		}
	}
}

func TestNF5ERejectsEmptyAndOversizedContent(t *testing.T) {
	root := t.TempDir()
	for _, data := range [][]byte{nil, {}, []byte(strings.Repeat("x", listFileMaxBytes+1))} {
		if err := nf5eInstallNewSource(root, "bad.list", data); err == nil {
			t.Fatalf("accepted size %d", len(data))
		}
	}
	if _, err := os.Lstat(filepath.Join(root, "bad.list")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid payload created output: %v", err)
	}
}

func TestNF5ERejectsNULPayload(t *testing.T) {
	root := t.TempDir()
	if err := nf5eInstallNewSource(root, "bad.list", []byte("x\x00y")); err == nil {
		t.Fatal("NUL accepted")
	}
}

func TestNF5ENotDirectoryRejected(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "not-dir")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := nf5eInstallNewSource(path, "new.list", []byte("example.com\n")); err == nil {
		t.Fatal("non-directory accepted")
	}
}

func TestNF5EHandlerUsesExclusiveBranch(t *testing.T) {
	data, err := os.ReadFile("list_sources.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "if !replaced {\n\t\tif err := nf5eInstallNewSource(listsRoot, request.Name, data); err != nil {") {
		t.Fatal("new upstream list is not installed exclusively")
	}
	if !strings.Contains(s, "} else if err := safety.WriteFileAtomic(path, data, mode); err != nil {") {
		t.Fatal("existing NF5D replacement path missing")
	}
}
