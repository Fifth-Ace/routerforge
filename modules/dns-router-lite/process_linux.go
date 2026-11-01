//go:build linux

package lite

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

type socketOwner struct {
	Version int    `json:"version"`
	PID     int    `json:"pid"`
	Start   string `json:"start"`
	Device  uint64 `json:"device"`
	Inode   uint64 `json:"inode"`
	CTime   int64  `json:"ctime_ns"`
}

func processStart(pid int) (string, error) {
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if e != nil {
		return "", e
	}
	i := strings.LastIndex(string(b), ") ")
	if i < 0 {
		return "", errors.New("invalid process identity")
	}
	fields := strings.Fields(string(b[i+2:]))
	if len(fields) <= 19 {
		return "", errors.New("short process identity")
	}
	return fields[19], nil
}
func AcquireProcessLock(path string) (func(), error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("absolute process lock required")
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	fd, e := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	var stat syscall.Stat_t
	if e = syscall.Fstat(fd, &stat); e != nil || stat.Mode&syscall.S_IFMT != syscall.S_IFREG || stat.Uid != uint32(os.Geteuid()) {
		syscall.Close(fd)
		return nil, errors.New("untrusted process lock file")
	}
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("another DNS controller owns this process lock: %w", e)
	}
	return func() { _ = syscall.Flock(fd, syscall.LOCK_UN); _ = syscall.Close(fd) }, nil
}
func RecordSocketOwner(socket string) error {
	fi, e := os.Lstat(socket)
	if e != nil {
		return e
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return errors.New("management endpoint is not a socket")
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("socket identity unavailable")
	}
	started, e := processStart(os.Getpid())
	if e != nil {
		return e
	}
	owner := socketOwner{Version: 1, PID: os.Getpid(), Start: started, Device: uint64(stat.Dev), Inode: stat.Ino, CTime: int64(stat.Ctim.Sec)*1e9 + int64(stat.Ctim.Nsec)}
	b, e := jsonMarshal(owner)
	if e != nil {
		return e
	}
	return atomicFile(socket+".owner.json", b)
}
func RecoverOwnedSocket(socket string) error {
	fi, e := os.Lstat(socket)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return errors.New("foreign file/symlink at management socket path")
	}
	var owner socketOwner
	if e = readState(socket+".owner.json", &owner, 4096); e != nil {
		return fmt.Errorf("management socket ownership unproven: %w", e)
	}
	if owner.Version != 1 || owner.PID < 1 || owner.Start == "" {
		return errors.New("invalid management socket ownership")
	}
	if actual, err := processStart(owner.PID); err == nil {
		if actual == owner.Start {
			return errors.New("management socket belongs to a live process")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("management socket owner liveness unproven: %w", err)
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || uint64(stat.Dev) != owner.Device || stat.Ino != owner.Inode || int64(stat.Ctim.Sec)*1e9+int64(stat.Ctim.Nsec) != owner.CTime {
		return errors.New("management socket identity changed; refusing unlink")
	}
	if e = os.Remove(socket); e != nil {
		return e
	}
	return os.Remove(socket + ".owner.json")
}
func RunSupervised(ctx context.Context, config, socket, ui string) error {
	release, e := AcquireProcessLock(config + ".supervisor.lock")
	if e != nil {
		return e
	}
	defer release()
	executable, e := os.Executable()
	if e != nil {
		return e
	}
	cleanup := func(ctx context.Context) error {
		k := &LinuxGate{Tables: &XTables{Exec: systemFirewallIO}, OwnershipPath: config + ".gate-owned.json", Conntrack: newSystemConntrack()}
		return cleanupController(ctx, socket, k)
	}
	supervisor := Supervisor{Log: os.Stderr, Cleanup: cleanup, Start: func() (SupervisedChild, error) {
		cmd, e := safety.CommandContext(context.Background(), executable, "--config", config, "--socket", socket, "--ui", ui, "--serve")
		if e != nil {
			return nil, e
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if e = cmd.Start(); e != nil {
			return nil, e
		}
		return &execChild{wait: cmd.Wait, terminate: func() error { return cmd.Process.Signal(syscall.SIGTERM) }}, nil
	}}
	return supervisor.Run(ctx)
}

func cleanupController(ctx context.Context, socket string, k *LinuxGate) error {
	plan, e := k.receipt()
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if e == nil {
		// Deactivate our dead DNS path even if a foreign socket blocks restart.
		// No xtables lock or foreign socket mutation is needed for this step.
		if err := k.Deactivate(ctx, plan); err != nil {
			fmt.Fprintln(os.Stderr, "DNS_SUPERVISOR: Gate cleanup pending; starting management for recovery:", err)
		}
	}
	return RecoverOwnedSocket(socket)
}

// Avoid exposing raw shell/process arguments through the lifecycle API.
type execChild struct {
	wait      func() error
	terminate func() error
}

func (c *execChild) Wait() error      { return c.wait() }
func (c *execChild) Terminate() error { return c.terminate() }
