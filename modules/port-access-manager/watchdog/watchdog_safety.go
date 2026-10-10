package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// checkConfirmation refuses symlinks and non-regular markers. The watchdog
// never interprets untrusted marker contents as a command.
func checkConfirmation(dir string) (bool, error) {
	info, err := os.Lstat(filepath.Join(dir, "confirmed"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return false, errors.New("invalid confirmation marker")
	}
	return true, nil
}

// watchTransaction keeps an independent rollback context when the parent's
// context is cancelled: cancellation must never silently skip cleanup.
func watchTransaction(ctx context.Context, dir string, deadline time.Time, now func() time.Time, wait func(context.Context, time.Duration) error, rollback func(context.Context, string) error) error {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || deadline.IsZero() || now == nil || wait == nil || rollback == nil {
		return errors.New("invalid watchdog transaction")
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("transaction directory must be private")
	}
	for {
		confirmed, err := checkConfirmation(dir)
		if err != nil {
			return rollbackWithReason(rollback, dir, fmt.Errorf("confirmation check: %w", err))
		}
		if confirmed {
			return nil
		}
		remaining := deadline.Sub(now())
		if remaining <= 0 {
			return rollbackWithReason(rollback, dir, nil)
		}
		if remaining > time.Second {
			remaining = time.Second
		}
		if err = wait(ctx, remaining); err != nil {
			return rollbackWithReason(rollback, dir, fmt.Errorf("watchdog interrupted: %w", err))
		}
	}
}
func rollbackWithReason(rollback func(context.Context, string) error, dir string, cause error) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return errors.Join(cause, rollback(cleanupCtx, dir))
}
