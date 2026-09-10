package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTailFileBoundsReadToLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routerforge.log")
	content := strings.Repeat("A", 1024) + strings.Repeat("B", 256)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := readTailFile(path, 256)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 256 {
		t.Fatalf("tail length=%d want=256", len(got))
	}
	if got != strings.Repeat("B", 256) {
		t.Fatalf("unexpected tail content")
	}
}

func TestReadTailFileReturnsWholeSmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routerforge.log")
	if err := os.WriteFile(path, []byte("small-log\n"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := readTailFile(path, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	if got != "small-log\n" {
		t.Fatalf("got %q", got)
	}
}

func TestReadTailFileZeroLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routerforge.log")
	if err := os.WriteFile(path, []byte("ignored"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := readTailFile(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("got %q want empty", got)
	}
}
