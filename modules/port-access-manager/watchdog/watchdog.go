package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// watchdog only acts on an explicitly armed, private transaction directory.
// It is not currently shipped or launched by Port Access Manager.
func run(ctx context.Context, dir string, deadline time.Time, now func() time.Time, wait func(context.Context, time.Duration) error, rollback func(context.Context, string) error) error {
	if dir == "" || !filepath.IsAbs(dir) {
		return errors.New("absolute transaction directory required")
	}
	if deadline.IsZero() {
		return errors.New("deadline required")
	}
	marker := filepath.Join(dir, "confirmed")
	for {
		if _, err := os.Stat(marker); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		remaining := deadline.Sub(now())
		if remaining <= 0 {
			return rollback(ctx, dir)
		}
		if remaining > time.Second {
			remaining = time.Second
		}
		if err := wait(ctx, remaining); err != nil {
			return err
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func rollbackScript(ctx context.Context, dir string) error {
	// Script name and directory are fixed; external commands are not accepted.
	script := filepath.Join(dir, "rollback.sh")
	info, err := os.Lstat(script)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0700 {
		return errors.New("rollback script must be a regular file with mode 0700")
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rollback failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func main() {
	dir := flag.String("transaction-dir", "", "absolute private transaction directory")
	deadline := flag.Int64("deadline-unix", 0, "absolute Unix rollback deadline")
	flag.Parse()
	if *deadline <= 0 {
		fmt.Fprintln(os.Stderr, "deadline required")
		os.Exit(2)
	}
	if err := run(context.Background(), *dir, time.Unix(*deadline, 0), time.Now, sleep, rollbackScript); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
