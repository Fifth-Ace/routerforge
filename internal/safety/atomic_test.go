package safety

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicFilePublish(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")

	atomic, err := NewAtomicFile(dir, ".routerforge-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer atomic.Cleanup()

	if _, err := atomic.File().WriteString("hello"); err != nil {
		t.Fatal(err)
	}
	if err := atomic.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := atomic.Close(); err != nil {
		t.Fatal(err)
	}
	if err := atomic.Publish(target); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("content=%q want=%q", string(got), "hello")
	}
}

func TestAtomicFileCleanupRemovesAbandonedTemp(t *testing.T) {
	dir := t.TempDir()

	atomic, err := NewAtomicFile(dir, ".routerforge-test-*")
	if err != nil {
		t.Fatal(err)
	}
	path := atomic.Path()

	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := atomic.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary file still exists: %v", err)
	}
}

func TestAtomicFilePublishRequiresClose(t *testing.T) {
	dir := t.TempDir()

	atomic, err := NewAtomicFile(dir, ".routerforge-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer atomic.Cleanup()

	if err := atomic.Publish(filepath.Join(dir, "target")); err == nil {
		t.Fatal("publish must require a closed temporary file")
	}
}

func TestWriteFileAtomicPublishesContentAndMode(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")

	if err := WriteFileAtomic(target, []byte("payload\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload\n" {
		t.Fatalf("content=%q", string(got))
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode=%#o want=%#o", info.Mode().Perm(), os.FileMode(0o600))
		}
	}
}

func TestWriteFileAtomicRejectsEmptyDestination(t *testing.T) {
	if err := WriteFileAtomic("", []byte("x"), 0o600); err == nil {
		t.Fatal("empty destination must be rejected")
	}
}
