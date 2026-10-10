package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestServiceDiscoveryIsExactAndRejectsAmbiguity(t *testing.T) {
	dir := t.TempDir()
	mk := func(s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, s), []byte("#!/bin/sh\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	mk("S80knockd")
	if _, err := findEngineInitScript("knockd", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := findEngineInitScript("other", dir); err == nil {
		t.Fatal("unknown accepted")
	}
	mk("S91knockd")
	if _, err := findEngineInitScript("knockd", dir); err == nil {
		t.Fatal("ambiguous accepted")
	}
}
func TestServiceActionsRejectedBeforeCommand(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "S90fwknopd"), []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatal(err)
	}
	calls := 0
	invoke := func(context.Context, string, string) error { calls++; return nil }
	if err := runExistingEngineService(context.Background(), "fwknopd", "rm -rf /", dir, invoke); err == nil {
		t.Fatal("action accepted")
	}
	if calls != 0 {
		t.Fatal("command ran")
	}
}
