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

func TestEngineServicePolicyDefaultsPreserve(t *testing.T) {
	got := serviceSnapshot("knockd", t.TempDir())
	if got.FirewallPolicy != "preserve" {
		t.Fatalf("policy = %q, want preserve", got.FirewallPolicy)
	}
}

func TestInstallAllowlistAndRunner(t *testing.T) {
	calls := 0
	invoke := func(_ context.Context, pkg string) error {
		calls++
		if pkg != "knockd" {
			t.Fatalf("package %q", pkg)
		}
		return nil
	}
	if err := installExistingEngine(context.Background(), "../fwknopd", invoke); err == nil {
		t.Fatal("accepted untrusted package")
	}
	if calls != 0 {
		t.Fatal("runner called for unknown engine")
	}
	if err := installExistingEngine(context.Background(), "knockd", invoke); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls %d", calls)
	}
}
