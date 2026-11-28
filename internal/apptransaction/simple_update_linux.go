//go:build linux

package apptransaction

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Use a RouterForge-owned parent if Entware's var belongs to another user.
// Keep the old lock path on existing compatible installations. Never split
// an existing lock or move an unresolved journal to a different filesystem.
func deviceCoordinatorAt(varDir, configDir string) (*Coordinator, error) {
	if _, err := os.Lstat(filepath.Join(varDir, "routerforge-app-transactions")); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return nil, err
		}
		return createCoordinatorUnder(varDir)
	}
	if _, err := os.Lstat(filepath.Join(configDir, "routerforge-app-transactions")); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return nil, err
		}
		return createCoordinatorUnder(configDir)
	}
	st, err := os.Lstat(varDir)
	if err == nil && st.IsDir() && st.Mode().Perm()&0022 == 0 {
		if owner, ok := st.Sys().(*syscall.Stat_t); ok && owner.Uid == uint32(os.Geteuid()) {
			return createCoordinatorUnder(varDir)
		}
	}
	return createCoordinatorUnder(configDir)
}

// Package operations exclude actual concurrent mutation, not a historical
// failed job. A new user-confirmed update archives the prior slot, never replays it.
func (c *Coordinator) AcquirePackageMutation() (*os.File, error) { return AcquireMutationLease(c.root) }
func (c *Coordinator) startSimpleUpdate(p Plan, check func() error, launch func(string, *os.File) (int, error)) (int, error) {
	if _, err := p.Hash(); err != nil {
		return 0, err
	}
	lease, err := c.AcquirePackageMutation()
	if err != nil {
		return 0, err
	}
	defer lease.Close()
	if check == nil || launch == nil {
		return 0, errors.New("missing update admission")
	}
	if err = check(); err != nil {
		return 0, err
	}
	for _, prefix := range []string{"completed-", "failed-", "interrupted-"} {
		if _, err = os.Lstat(filepath.Join(c.root, prefix+p.ID)); !errors.Is(err, os.ErrNotExist) {
			return 0, ErrRecovery
		}
	}
	if _, err = os.Lstat(c.slot()); err == nil {
		old, readErr := c.inspect()
		name := "interrupted-" + p.ID
		if readErr == nil && old.Plan != nil {
			if old.Plan.ID == p.ID {
				return 0, ErrRecovery
			}
			name = "failed-" + old.Plan.ID
			if old.State == string(Completed) {
				name = "completed-" + old.Plan.ID
			}
		}
		dest := filepath.Join(c.root, name)
		if _, err = os.Lstat(dest); !errors.Is(err, os.ErrNotExist) {
			return 0, ErrRecovery
		}
		if err = os.Rename(c.slot(), dest); err != nil {
			return 0, err
		}
		if err = syncDirectory(c.root); err != nil {
			return 0, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	if err = os.Mkdir(c.slot(), 0700); err != nil {
		return 0, err
	}
	if err = syncDirectory(c.root); err != nil {
		return 0, err
	}
	return launch(c.slot(), lease)
}

// No per-build approval document or whole-device version matrix. The fixed
// packaged executable, the authenticated server-side plan and OPKG's normal
// dependency handling remain the boundary. No client command/path/URL allowed.
func DeviceUpdateAvailable(version string) bool {
	if !executionDeviceScope(version) {
		return false
	}
	st, err := os.Lstat(deviceWorkerPath)
	return err == nil && st.Mode().IsRegular() && st.Mode().Perm()&0111 != 0 && st.Mode().Perm()&0022 == 0 && coreEvidenceOwned(st, 0)
}
func (c *Coordinator) StartSimpleDeviceUpdate(ctx context.Context, version string, p Plan, revalidate func() error) (int, error) {
	if !DeviceUpdateAvailable(version) || revalidate == nil {
		return 0, errors.New("update worker unavailable")
	}
	return c.startSimpleUpdate(p, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !DeviceUpdateAvailable(version) {
			return errors.New("update worker unavailable")
		}
		if err := revalidate(); err != nil {
			return err
		}
		return ctx.Err()
	}, func(dir string, lease *os.File) (int, error) {
		return startDetachedWithLease(dir, c.root, deviceWorkerPath, nil, p, lease)
	})
}

// Keep exact staged hashes and OPKG installed-version verification. Disabled
// services are not required to start; runtime management checks are advisory.
// VerifiedOPKG preserves the Profiling marker across the old package prerm.
func (c *Coordinator) RunSimpleDeviceWorker(ctx context.Context, version string) (Journal, error) {
	if !DeviceUpdateAvailable(version) {
		return Journal{}, errors.New("update worker unavailable")
	}
	self, err := os.Stat("/proc/self/exe")
	if err != nil {
		return Journal{}, err
	}
	actual, err := os.Lstat(deviceWorkerPath)
	if err != nil || !os.SameFile(self, actual) {
		return Journal{}, errors.New("worker identity mismatch")
	}
	return c.RunWorker(ctx, func(dir string, p Plan, lease *os.File) (Hooks, error) {
		staging, err := NewStaging(dir, p)
		if err != nil {
			return nil, err
		}
		adapter, err := NewVerifiedOPKG(staging, c.root, lease, func(context.Context, Package) error { return nil })
		if err != nil {
			return nil, err
		}
		staging.report = func(pkg Package, progress DownloadProgress) { writeDownloadProgress(dir, p.ID, pkg, progress) }
		return &simpleUpdateHooks{VerifiedOPKG: adapter, dir: dir, plan: p}, nil
	})
}

type simpleUpdateHooks struct {
	*VerifiedOPKG
	dir  string
	plan Plan
}

func (h *simpleUpdateHooks) Prepare(ctx context.Context, p Package) error {
	fmt.Println("DOWNLOAD", p.Name)
	// The OPKG status command itself takes a lock; do not run it in parallel.
	// Install performs the version admission and exact asset check just before mutation.
	started := time.Now()
	err := h.Staging.Prepare(ctx, p)
	state := "downloaded"
	n := p.Size
	if err != nil {
		state = "failed"
		n = 0
	}
	writeDownloadProgress(h.dir, h.plan.ID, p, DownloadProgress{State: state, Bytes: n})
	fmt.Printf("DOWNLOAD_DONE %s elapsed_ms=%d error=%v\n", p.Name, time.Since(started).Milliseconds(), err)
	return err
}
func (h *simpleUpdateHooks) PrepareBatch(ctx context.Context, packages []Package) (int, error) {
	return prepareParallel(ctx, packages, h.Prepare)
}

// Install already checks the exact staged SHA256 before calling OPKG.
func (h *simpleUpdateHooks) Check(ctx context.Context, p Package) error { return ctx.Err() }
func (h *simpleUpdateHooks) Install(ctx context.Context, p Package) error {
	fmt.Println("INSTALL", p.Name)
	started := time.Now()
	err := h.VerifiedOPKG.Install(ctx, p)
	fmt.Printf("INSTALL_DONE %s elapsed_ms=%d error=%v\n", p.Name, time.Since(started).Milliseconds(), err)
	return err
}
func (h *simpleUpdateHooks) Verify(ctx context.Context, p Package) error {
	fmt.Println("VERIFY", p.Name)
	return h.VerifiedOPKG.Verify(ctx, p)
}
func (h *simpleUpdateHooks) Cleanup() {
	for _, p := range h.plan.Packages {
		if err := os.Remove(filepath.Join(h.dir, p.Asset)); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Println("Cleanup", p.Name, err)
		}
	}
}

// Read a bounded tail of the active worker's technical log. No write/replay.
func (c *Coordinator) UpdateLog() string {
	fd, err := syscall.Open(filepath.Join(c.slot(), "worker.log"), syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return ""
	}
	f := os.NewFile(uintptr(fd), "worker.log")
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !privateRegular(st) {
		return ""
	}
	if st.Size() > 16384 {
		if _, err = f.Seek(-16384, io.SeekEnd); err != nil {
			return ""
		}
	}
	b, err := io.ReadAll(io.LimitReader(f, 16384))
	if err != nil {
		return ""
	}
	return string(b)
}
