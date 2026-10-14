package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNF1ShellForSyntaxFallsBackFromUnsupportedShell(t *testing.T) {
	dir := t.TempDir()
	unsupported := filepath.Join(dir, "unsupported-sh")
	if err := os.WriteFile(unsupported, []byte("#!/bin/sh\nexit 245\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := nf1ShellForSyntax(ctx, []string{unsupported, "/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "/bin/sh" {
		t.Fatalf("expected fallback to /bin/sh, got %q", got)
	}
}

func TestNF1ShellForSyntaxFailsClosedWithoutCompatibleShell(t *testing.T) {
	dir := t.TempDir()
	unsupported := filepath.Join(dir, "unsupported-sh")
	if err := os.WriteFile(unsupported, []byte("#!/bin/sh\nexit 245\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := nf1ShellForSyntax(ctx, []string{unsupported, filepath.Join(dir, "missing")})
	if got != "" || err == nil || !strings.Contains(err.Error(), "поддержкой -n") {
		t.Fatalf("expected fail-closed, got path %q error %v", got, err)
	}
}
