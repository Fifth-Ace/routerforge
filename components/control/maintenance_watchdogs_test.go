package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAdminWatchdogDefinitionWhitelist(t *testing.T) {
	for _, id := range []string{"nfqws2", "awg-manager", "adguard-home"} {
		if _, ok := adminWatchdogDefinition(id); !ok {
			t.Fatalf("missing watchdog definition %q", id)
		}
	}
	if _, ok := adminWatchdogDefinition("arbitrary-service"); ok {
		t.Fatal("arbitrary service unexpectedly accepted")
	}
}

func TestFindAdminWatchdogService(t *testing.T) {
	definition, _ := adminWatchdogDefinition("nfqws2")
	services := []serviceInfo{
		{ID: "S99other", Name: "other", Path: "/opt/etc/init.d/S99other"},
		{ID: "S51nfqws2", Name: "nfqws2", Path: "/opt/etc/init.d/S51nfqws2", Executable: true, Running: true},
	}
	service, ok := findAdminWatchdogService(definition, services)
	if !ok || service.ID != "S51nfqws2" || !service.Running {
		t.Fatalf("unexpected service: %+v ok=%v", service, ok)
	}
}

func TestSafeAdminWatchdogServicePath(t *testing.T) {
	old := serviceInitDir
	serviceInitDir = t.TempDir()
	defer func() { serviceInitDir = old }()

	good := filepath.Join(serviceInitDir, "S99demo")
	if err := os.WriteFile(good, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if !safeAdminWatchdogServicePath(good) {
		t.Fatal("expected executable regular init script")
	}

	outside := filepath.Join(t.TempDir(), "S99demo")
	if err := os.WriteFile(outside, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if safeAdminWatchdogServicePath(outside) {
		t.Fatal("outside init script accepted")
	}
}

func TestWatchdogAttemptWindowPruning(t *testing.T) {
	runtime := adminWatchdogRuntime{
		config:      adminWatchdogConfig{Version: 1, Items: map[string]adminWatchdogItem{}},
		attempts:    map[string][]time.Time{},
		lastAttempt: map[string]adminWatchdogAttempt{},
	}
	now := time.Now()
	runtime.attempts["nfqws2"] = []time.Time{
		now.Add(-2 * time.Hour),
		now.Add(-30 * time.Minute),
	}
	runtime.pruneAttemptsLocked(now)
	if got := len(runtime.attempts["nfqws2"]); got != 1 {
		t.Fatalf("attempts=%d want=1", got)
	}
}
