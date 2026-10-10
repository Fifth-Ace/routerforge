package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIndependentWatchdogTimeout(t *testing.T) {
	dir := privateTestDir(t)
	now := time.Unix(100, 0)
	calls := 0
	wait := func(_ context.Context, _ time.Duration) error { now = now.Add(time.Second); return nil }
	rollback := func(_ context.Context, _ string) error { calls++; return nil }
	err := run(context.Background(), dir, now.Add(2*time.Second), func() time.Time { return now }, wait, rollback)
	if err != nil || calls != 1 {
		t.Fatalf("err=%v rollback=%d", err, calls)
	}
}
func TestIndependentWatchdogConfirmation(t *testing.T) {
	dir := privateTestDir(t)
	now := time.Unix(100, 0)
	calls := 0
	if err := os.WriteFile(filepath.Join(dir, "confirmed"), []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	err := run(context.Background(), dir, now, func() time.Time { return now }, func(context.Context, time.Duration) error { return nil }, func(context.Context, string) error { calls++; return nil })
	if err != nil || calls != 0 {
		t.Fatalf("err=%v rollback=%d", err, calls)
	}
}
func TestIndependentWatchdogFailSafe(t *testing.T) {
	dir := privateTestDir(t)
	now := time.Unix(100, 0)
	expected := errors.New("rollback failed")
	err := run(context.Background(), dir, now, func() time.Time { return now }, func(context.Context, time.Duration) error { return nil }, func(context.Context, string) error { return expected })
	if !errors.Is(err, expected) {
		t.Fatalf("rollback error lost: %v", err)
	}
	if err := run(context.Background(), "relative", now, func() time.Time { return now }, nil, nil); err == nil {
		t.Fatal("relative path accepted")
	}
}
func TestScriptPermissions(t *testing.T) {
	dir := privateTestDir(t)
	file := filepath.Join(dir, "rollback.sh")
	if err := os.WriteFile(file, []byte("exit 0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := rollbackScript(context.Background(), dir); err == nil {
		t.Fatal("unsafe script permissions accepted")
	}
	if err := os.Chmod(file, 0700); err != nil {
		t.Fatal(err)
	}
	if err := rollbackScript(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
}
