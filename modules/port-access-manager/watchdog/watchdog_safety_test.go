package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchTransactionTimeoutRecovery(t *testing.T) {
	dir := privateTestDir(t)
	now := time.Unix(100, 0)
	calls := 0
	wait := func(context.Context, time.Duration) error { now = now.Add(time.Second); return nil }
	err := watchTransaction(context.Background(), dir, now.Add(2*time.Second), func() time.Time { return now }, wait, func(ctx context.Context, _ string) error {
		if ctx.Err() != nil {
			t.Fatal("cancelled rollback context")
		}
		calls++
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
func TestWatchTransactionCancelledStillRollsBack(t *testing.T) {
	dir := privateTestDir(t)
	calls := 0
	err := watchTransaction(context.Background(), dir, time.Now().Add(time.Hour), time.Now, func(context.Context, time.Duration) error { return context.Canceled }, func(ctx context.Context, _ string) error {
		if ctx.Err() != nil {
			t.Fatal("cancelled rollback")
		}
		calls++
		return nil
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
func TestWatchTransactionRejectsSymlinkConfirmation(t *testing.T) {
	dir := privateTestDir(t)
	real := filepath.Join(dir, "real")
	if err := os.WriteFile(real, []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(dir, "confirmed")); err != nil {
		t.Skip(err)
	}
	calls := 0
	err := watchTransaction(context.Background(), dir, time.Now().Add(time.Hour), time.Now, func(context.Context, time.Duration) error { t.Fatal("must not wait"); return nil }, func(context.Context, string) error { calls++; return nil })
	if err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
func TestWatchTransactionAcceptsPrivateConfirmation(t *testing.T) {
	dir := privateTestDir(t)
	if err := os.WriteFile(filepath.Join(dir, "confirmed"), []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	err := watchTransaction(context.Background(), dir, time.Now().Add(time.Hour), time.Now, func(context.Context, time.Duration) error { t.Fatal("must not wait"); return nil }, func(context.Context, string) error { t.Fatal("must not rollback"); return nil })
	if err != nil {
		t.Fatal(err)
	}
}

func privateTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
