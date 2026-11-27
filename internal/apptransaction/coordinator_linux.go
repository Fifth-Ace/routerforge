//go:build linux

package apptransaction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Coordinator owns the durable single active slot and the shared package
// mutation lease. All cooperating package mutations must enter via this lease.
// A non-completed slot is never silently replayed, discarded, or overwritten.
type Coordinator struct{ root string }

type SlotStatus struct {
	State   string   `json:"state"`
	Plan    *Plan    `json:"plan,omitempty"`
	Journal *Journal `json:"journal,omitempty"`
}

const activeSlot = "active"

func OpenCoordinator(root string) (*Coordinator, error) {
	if _, err := Open(root); err != nil {
		return nil, err
	}
	return &Coordinator{root}, nil
}

// DeviceCoordinator canonicalizes the Entware mount (/opt may be a symlink).
// The persistent var parent must be an owned directory not writable by others.
// Only our private subdirectory is created. No opkg, network or config edits.
func DeviceCoordinator() (*Coordinator, error) {
	return createCoordinatorUnder("/opt/var")
}
func createCoordinatorUnder(parent string) (*Coordinator, error) {
	original, err := os.Lstat(parent)
	if err != nil {
		return nil, err
	}
	if !original.IsDir() || original.Mode()&os.ModeSymlink != 0 || original.Mode().Perm()&0022 != 0 {
		return nil, errors.New("unsafe transaction parent")
	}
	owner, ok := original.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("unowned transaction parent")
	}
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(resolved, "routerforge-app-transactions")
	if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	c, err := OpenCoordinator(root)
	if err != nil {
		return nil, err
	}
	if err = syncDirectory(resolved); err != nil {
		return nil, err
	}
	return c, nil
}
func syncDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func (c *Coordinator) slot() string { return filepath.Join(c.root, activeSlot) }

// inspect validates the durable request and checkpoint together. A missing
// checkpoint is valid only while a launcher/worker holds the global lease.
func (c *Coordinator) inspect() (SlotStatus, error) {
	dir := c.slot()
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return SlotStatus{State: "idle"}, nil
	} else if err != nil {
		return SlotStatus{}, err
	}
	s, err := Open(dir)
	if err != nil {
		return SlotStatus{}, err
	}
	p, err := ReadWorkerRequest(dir)
	if err != nil {
		return SlotStatus{}, err
	}
	j, err := s.Load()
	if errors.Is(err, os.ErrNotExist) {
		return SlotStatus{State: "queued", Plan: &p}, nil
	}
	if err != nil {
		return SlotStatus{}, err
	}
	h, err := p.Hash()
	if err != nil || j.PlanSHA256 != h {
		return SlotStatus{}, errors.New("request/checkpoint mismatch")
	}
	return SlotStatus{State: string(j.Phase), Plan: &p, Journal: &j}, nil
}

// Status does not write a checkpoint or start any process. Lease acquisition
// is a nonblocking probe; no free lease means the slot may still be changing.
// When the lease is free, any noncompleted slot is recovery-required.
func (c *Coordinator) Status() (SlotStatus, error) {
	lease, err := AcquireMutationLease(c.root)
	if errors.Is(err, ErrBusy) {
		v, e := c.inspect()
		if e != nil {
			return SlotStatus{State: "busy"}, nil
		}
		if v.State == "idle" {
			v.State = "busy"
		}
		return v, nil
	}
	if err != nil {
		return SlotStatus{}, err
	}
	defer lease.Close()
	v, err := c.inspect()
	if err != nil {
		return SlotStatus{State: string(RecoveryRequired)}, err
	}
	if v.State != "idle" && v.State != string(Completed) {
		v.State = string(RecoveryRequired)
	}
	return v, nil
}

// AcquireLegacy excludes old Core package paths while a new worker is active
// or a transaction is unresolved. External shell opkg is outside this contract.
func (c *Coordinator) AcquireLegacy() (*os.File, error) {
	lease, err := AcquireMutationLease(c.root)
	if err != nil {
		return nil, err
	}
	v, err := c.inspect()
	if err != nil || (v.State != "idle" && v.State != string(Completed)) {
		lease.Close()
		return nil, ErrRecovery
	}
	return lease, nil
}

// Start is deliberately not an HTTP entry point. The integrating service must
// authorize the exact selected plan and choose the trusted executable/args.
// A completed slot is archived, never deleted; ambiguous slots block launch.
func (c *Coordinator) Start(executable string, args []string, p Plan) (int, error) {
	if _, err := p.Hash(); err != nil {
		return 0, err
	}
	lease, err := c.AcquireLegacy()
	if err != nil {
		return 0, err
	}
	defer lease.Close()
	v, err := c.inspect()
	if err != nil {
		return 0, err
	}
	// Reject replay before changing the existing slot or history.
	if _, err = os.Lstat(filepath.Join(c.root, "completed-"+p.ID)); !errors.Is(err, os.ErrNotExist) {
		return 0, ErrRecovery
	}
	if v.State == string(Completed) {
		if v.Plan.ID == p.ID {
			return 0, ErrRecovery
		}
		dest := filepath.Join(c.root, "completed-"+v.Plan.ID)
		if _, err = os.Lstat(dest); !errors.Is(err, os.ErrNotExist) {
			return 0, ErrRecovery
		}
		if err = os.Rename(c.slot(), dest); err != nil {
			return 0, err
		}
		if err = syncDirectory(c.root); err != nil {
			return 0, err
		}
	}
	if err = os.Mkdir(c.slot(), 0700); err != nil {
		return 0, err
	}
	if err = syncDirectory(c.root); err != nil {
		return 0, err
	}
	return startDetachedWithLease(c.slot(), c.root, executable, args, p, lease)
}

// RunWorker is the detached process entry point. Hook factory must implement
// trusted local lifecycle verification; no user-supplied command is accepted.
// The inherited lease is validated before reading the request or running hooks.
func (c *Coordinator) RunWorker(ctx context.Context, factory func(string, Plan, *os.File) (Hooks, error)) (Journal, error) {
	var zero Journal
	if factory == nil {
		return zero, errors.New("missing worker hooks")
	}
	lease, err := AdoptMutationLease(c.root, MutationLeaseFD)
	if err != nil {
		return zero, err
	}
	defer lease.Close()
	p, err := ReadWorkerRequest(c.slot())
	if err != nil {
		return zero, err
	}
	s, err := Open(c.slot())
	if err != nil {
		return zero, err
	}
	h, err := factory(c.slot(), p, lease)
	if err != nil {
		return zero, fmt.Errorf("worker hook setup: %w", err)
	}
	return s.Execute(ctx, p, h)
}

// ValidateDevUpgrade exposes the same no-downgrade contract used by the actual
// executor, so the server-side planner cannot advertise a rejected upgrade.
func ValidateDevUpgrade(current, target string) error { return allowUpgrade(current, target) }

// RunVerifiedWorker binds the trusted staging and opkg adapters to the
// coordinator. A production caller must supply the accepted lifecycle proof
// function. No HTTP path in this delivery calls this method.
func (c *Coordinator) RunVerifiedWorker(ctx context.Context, lifecycle func(context.Context, Package) error) (Journal, error) {
	if lifecycle == nil {
		return Journal{}, errors.New("missing lifecycle verifier")
	}
	return c.RunWorker(ctx, func(dir string, p Plan, lease *os.File) (Hooks, error) {
		staging, err := NewStaging(dir, p)
		if err != nil {
			return nil, err
		}
		return NewVerifiedOPKG(staging, c.root, lease, lifecycle)
	})
}
