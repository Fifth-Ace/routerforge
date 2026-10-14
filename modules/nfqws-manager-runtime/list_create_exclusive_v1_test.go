package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNF5CCreateFreshListIsEmpty(t *testing.T) {
	root := t.TempDir()
	if err := nf5cCreateListExclusiveInRoot(root, "mine.list"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "mine.list"))
	if err != nil || len(content) != 0 {
		t.Fatalf("new list must be empty: %q %v", content, err)
	}
}

func TestNF5CExistingListNeverOverwritten(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mine.list")
	old := []byte("do-not-erase.example\n")
	if err := os.WriteFile(path, old, 0600); err != nil {
		t.Fatal(err)
	}
	if err := nf5cCreateListExclusiveInRoot(root, "mine.list"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected EEXIST: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(old) {
		t.Fatalf("existing data overwritten: %q %v", got, err)
	}
}

func TestNF5CSymlinkDestinationNeverFollowed(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(t.TempDir(), "private")
	if err := os.WriteFile(private, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(private, filepath.Join(root, "mine.list")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := nf5cCreateListExclusiveInRoot(root, "mine.list"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("final symlink must be blocked: %v", err)
	}
	got, err := os.ReadFile(private)
	if err != nil || string(got) != "secret" {
		t.Fatalf("symlink target changed: %q %v", got, err)
	}
}

func TestNF5CSymlinkRootRejected(t *testing.T) {
	actual := t.TempDir()
	link := filepath.Join(t.TempDir(), "lists")
	if err := os.Symlink(actual, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := nf5cCreateListExclusiveInRoot(link, "mine.list"); err == nil {
		t.Fatal("symlink root accepted")
	}
	if _, err := os.Lstat(filepath.Join(actual, "mine.list")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("wrote inside symlink root: %v", err)
	}
}

func TestNF5CNonDirectoryRootRejected(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(root, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := nf5cCreateListExclusiveInRoot(root, "mine.list"); err == nil {
		t.Fatal("regular file root accepted")
	}
}

func TestNF5CMissingRootDoesNotCreateFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	if err := nf5cCreateListExclusiveInRoot(root, "mine.list"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing root not rejected: %v", err)
	}
}

func TestNF5CUnsafeNamesRejected(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"", "../outside.list", "/tmp/evil.list", ".hidden.list", "mine.list-opkg", "mine.list-old", "mine.txt", "evil name.list"} {
		if err := nf5cCreateListExclusiveInRoot(root, name); err == nil {
			t.Errorf("unsafe list name accepted: %q", name)
		}
	}
}

func TestNF5CConcurrentCreateHasSingleWinner(t *testing.T) {
	root := t.TempDir()
	const parallel = 24
	start := make(chan struct{})
	results := make(chan error, parallel)
	var wg sync.WaitGroup
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- nf5cCreateListExclusiveInRoot(root, "race.list")
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, os.ErrExist) {
			t.Fatalf("unexpected race error: %v", err)
		}
	}
	if winners != 1 {
		t.Fatalf("exclusive create admitted %d winners", winners)
	}
}
