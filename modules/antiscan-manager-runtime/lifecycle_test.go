package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeAntiscanLifecycleAction(t *testing.T) {
	for _, action := range []string{"start", "STOP", " reload "} {
		if _, err := normalizeAntiscanLifecycleAction(action); err != nil {
			t.Fatalf("action %q rejected: %v", action, err)
		}
	}
	for _, action := range []string{"", "restart", "flush", "status"} {
		if _, err := normalizeAntiscanLifecycleAction(action); err == nil {
			t.Fatalf("action %q accepted", action)
		}
	}
}

func TestApplyAntiscanLifecycleStartReloadStop(t *testing.T) {
	cfg := fakeAntiscanLifecycleConfig(t)

	start, status, err := applyAntiscanLifecycle(context.Background(), cfg, "start")
	if err != nil || status != http.StatusOK || !start.Changed || !start.Verified || !start.AfterRunning {
		t.Fatalf("start status=%d err=%v result=%+v", status, err, start)
	}

	reload, status, err := applyAntiscanLifecycle(context.Background(), cfg, "reload")
	if err != nil || status != http.StatusOK || !reload.Changed || !reload.Verified || !reload.BeforeRunning || !reload.AfterRunning {
		t.Fatalf("reload status=%d err=%v result=%+v", status, err, reload)
	}

	stop, status, err := applyAntiscanLifecycle(context.Background(), cfg, "stop")
	if err != nil || status != http.StatusOK || !stop.Changed || !stop.Verified || stop.AfterRunning {
		t.Fatalf("stop status=%d err=%v result=%+v", status, err, stop)
	}
}

func TestApplyAntiscanLifecycleIdempotentStartStop(t *testing.T) {
	cfg := fakeAntiscanLifecycleConfig(t)
	if err := os.WriteFile(cfg.StatusFile, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	start, status, err := applyAntiscanLifecycle(context.Background(), cfg, "start")
	if err != nil || status != http.StatusOK || start.Changed || !start.Verified || !start.AfterRunning {
		t.Fatalf("idempotent start status=%d err=%v result=%+v", status, err, start)
	}
	if err := os.Remove(cfg.StatusFile); err != nil {
		t.Fatal(err)
	}

	stop, status, err := applyAntiscanLifecycle(context.Background(), cfg, "stop")
	if err != nil || status != http.StatusOK || stop.Changed || !stop.Verified || stop.AfterRunning {
		t.Fatalf("idempotent stop status=%d err=%v result=%+v", status, err, stop)
	}
}

func TestApplyAntiscanLifecycleRejectsReloadWhenStopped(t *testing.T) {
	cfg := fakeAntiscanLifecycleConfig(t)
	result, status, err := applyAntiscanLifecycle(context.Background(), cfg, "reload")
	if err == nil || status != http.StatusConflict || result.Verified {
		t.Fatalf("status=%d err=%v result=%+v", status, err, result)
	}
}

func TestApplyAntiscanLifecycleRejectsUpstreamLock(t *testing.T) {
	cfg := fakeAntiscanLifecycleConfig(t)
	if err := os.WriteFile(cfg.StatusFile, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.ConfigLockFile, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, status, err := applyAntiscanLifecycle(context.Background(), cfg, "reload")
	if err == nil || status != http.StatusConflict || result.Verified {
		t.Fatalf("status=%d err=%v result=%+v", status, err, result)
	}
}

func TestAntiscanMutationGateSerializesActions(t *testing.T) {
	if !acquireAntiscanMutationGate() {
		t.Fatal("first mutation gate acquisition failed")
	}
	if acquireAntiscanMutationGate() {
		releaseAntiscanMutationGate()
		t.Fatal("second mutation gate acquisition unexpectedly succeeded")
	}
	releaseAntiscanMutationGate()
	if !acquireAntiscanMutationGate() {
		t.Fatal("mutation gate did not reopen")
	}
	releaseAntiscanMutationGate()
}

func fakeAntiscanLifecycleConfig(t *testing.T) runtimeConfig {
	t.Helper()
	dir := t.TempDir()
	statusPath := filepath.Join(dir, "ascn.run")
	scriptPath := filepath.Join(dir, "S99ascn")
	script := fmt.Sprintf(`#!/bin/sh
STATUS=%q
case "$1" in
  start)
    printf '1\n' > "$STATUS"
    printf 'started\n'
    ;;
  stop)
    rm -f "$STATUS"
    printf 'stopped\n'
    ;;
  reload)
    [ -f "$STATUS" ] || exit 3
    printf 'reloaded\n'
    ;;
  *)
    exit 4
    ;;
esac
`, statusPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return runtimeConfig{
		InitScript:     scriptPath,
		StatusFile:     statusPath,
		ConfigLockFile: filepath.Join(dir, "ascn.lock"),
		GeoLockFile:    filepath.Join(dir, "ascn_geo.lock"),
	}
}
