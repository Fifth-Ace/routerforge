//go:build linux

package apptransaction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// VerifiedOPKG is an update-only adapter. It does not expose an HTTP endpoint
// or start a worker. The integrating caller must hold the global lease for the
// entire transaction and provide module-specific lifecycle verification.
// No forced reinstall, downgrade, removal or automatic repair is allowed.
type VerifiedOPKG struct {
	*Staging
	lease      *os.File
	lockDir    string
	executable string
	lifecycle  func(context.Context, Package) error
}

func NewVerifiedOPKG(staging *Staging, lockDir string, lease *os.File, lifecycle func(context.Context, Package) error) (*VerifiedOPKG, error) {
	return newVerifiedOPKG(staging, lockDir, lease, "/opt/bin/opkg", lifecycle)
}

func newVerifiedOPKG(staging *Staging, lockDir string, lease *os.File, executable string, lifecycle func(context.Context, Package) error) (*VerifiedOPKG, error) {
	if staging == nil || lease == nil || lifecycle == nil || !filepath.IsAbs(executable) || filepath.Clean(executable) != executable {
		return nil, errors.New("invalid opkg adapter")
	}
	a := &VerifiedOPKG{staging, lease, lockDir, executable, lifecycle}
	if err := a.checkLease(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *VerifiedOPKG) checkLease() error {
	s, err := Open(a.lockDir)
	if err != nil {
		return err
	}
	st, err := a.lease.Stat()
	if err != nil || !privateRegular(st) {
		return errors.New("invalid mutation lease")
	}
	path, err := os.Lstat(filepath.Join(s.dir, mutationLockName))
	if err != nil || !privateRegular(path) || !os.SameFile(st, path) {
		return errors.New("mutation lease identity mismatch")
	}
	if err = syscall.Flock(int(a.lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return err
	}
	return nil
}

// Limit captured diagnostics without blocking a child that writes more.
type opkgOutput struct {
	data     []byte
	overflow bool
}

func (b *opkgOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := (128 << 10) - len(b.data)
	if n > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	b.data = append(b.data, p...)
	return n, nil
}

// status is read-only and bounded. The mutation itself is deliberately NOT
// killed by HTTP cancellation or a deadline. A stuck opkg must remain visible
// and keep the lease; recovery is a separate, explicit operator action.
func (a *VerifiedOPKG) status(ctx context.Context, p Package) (string, error) {
	return readInstalledDevVersion(ctx, a.executable, p.Name)
}

func readInstalledDevVersion(ctx context.Context, executable, name string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd, err := safety.CommandContext(ctx, executable, "status", name)
	if err != nil {
		return "", err
	}
	var out opkgOutput
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("opkg status %s: %w", name, err)
	}
	if out.overflow {
		return "", errors.New("opkg status output exceeds limit")
	}
	return installedDevVersion(string(out.data), name)
}

func installedDevVersion(raw, name string) (string, error) {
	var version string
	matches := 0
	for _, block := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n\n") {
		fields := map[string]string{}
		duplicate := false
		for _, line := range strings.Split(block, "\n") {
			key, value, ok := strings.Cut(line, ": ")
			if !ok {
				continue
			}
			switch key {
			case "Package", "Version", "Architecture", "Status":
				if _, ok := fields[key]; ok {
					duplicate = true
				}
				fields[key] = strings.TrimSpace(value)
			}
		}
		if fields["Package"] != name {
			continue
		}
		matches++
		if duplicate || fields["Architecture"] != "aarch64-3.10" || fields["Status"] != "install user installed" {
			return "", errors.New("package is not normally installed for the target architecture")
		}
		version = fields["Version"]
	}
	if matches != 1 || version == "" {
		return "", errors.New("missing or ambiguous installed package")
	}
	return version, nil
}
func devRevision(v string) (uint64, error) {
	s, ok := strings.CutPrefix(v, "1~dev.r")
	if !ok {
		return 0, errors.New("not a dev version")
	}
	n, sha, ok := strings.Cut(s, ".")
	if !ok || len(sha) != 12 {
		return 0, errors.New("invalid dev version")
	}
	for _, c := range sha {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return 0, errors.New("invalid dev version")
		}
	}
	for _, c := range n {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid revision")
		}
	}
	return strconv.ParseUint(n, 10, 64)
}

func (a *VerifiedOPKG) Prepare(ctx context.Context, p Package) error {
	if err := a.checkLease(); err != nil {
		return err
	}
	if _, err := a.path(p); err != nil {
		return err
	}
	current, err := a.status(ctx, p)
	if err != nil {
		return err
	}
	if err = allowUpgrade(current, p.Version); err != nil {
		return err
	}
	return a.Staging.Prepare(ctx, p)
}
func allowUpgrade(current, target string) error {
	old, err := devRevision(current)
	if err != nil {
		return err
	}
	next, err := devRevision(target)
	if err != nil {
		return err
	}
	if current != target && next <= old {
		return errors.New("downgrade or same-revision replacement rejected")
	}
	return nil
}
func (a *VerifiedOPKG) Install(ctx context.Context, p Package) error {
	if err := a.checkLease(); err != nil {
		return err
	}
	path, err := a.path(p)
	if err != nil {
		return err
	}
	current, err := a.status(ctx, p)
	if err != nil {
		return err
	}
	if err = allowUpgrade(current, p.Version); err != nil {
		return err
	}
	f, err := a.OpenVerified(p)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	actual, err := os.Lstat(path)
	if err != nil || !os.SameFile(st, actual) {
		return errors.New("staged path identity mismatch")
	}
	if current == p.Version {
		return nil
	} // Verify still runs; never force-reinstall.
	// The private staging directory protects the pathname against unprivileged
	// replacement. Root is trusted. Keep the cooperative lease in the child as
	// well: if the worker dies, opkg retains it until its last inherited copy
	// closes. Hardware acceptance must check lifecycle scripts for FD leaks.
	cmd, err := safety.CommandContext(context.Background(), a.executable, "install", path)
	if err != nil {
		return err
	}
	intent := ""
	if p.Name == "routerforge-profiling" {
		intent, err = recordProfilingMigration(a.store, p, current, profilingMarkerPath, uint32(os.Geteuid()))
		if err != nil {
			return err
		}
	}
	cmd.Env, err = executionEnvironment(cmd.Environ(), intent)
	if err != nil {
		return err
	}
	cmd.ExtraFiles = []*os.File{a.lease}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("opkg install %s: %w", p.Name, err)
	}
	// Only the fixed device adapter scans real process descriptors; fixture
	// executables in tests exercise the observer separately. A holder or an
	// ambiguous observation is a recovery-required result, never an unlock.
	if a.executable == "/opt/bin/opkg" {
		pid, auditErr := foreignLeaseHolder("/proc", a.lease, uint32(os.Geteuid()), os.Getpid())
		if auditErr != nil {
			return fmt.Errorf("lease descriptor audit: %w", auditErr)
		}
		if pid != 0 {
			return fmt.Errorf("foreign lease descriptor remains in PID %d", pid)
		}
	}
	return nil
}
func (a *VerifiedOPKG) Verify(ctx context.Context, p Package) error {
	if err := a.checkLease(); err != nil {
		return err
	}
	if _, err := a.path(p); err != nil {
		return err
	}
	v, err := a.status(ctx, p)
	if err != nil {
		return err
	}
	if v != p.Version {
		return fmt.Errorf("installed version mismatch: %s", p.Name)
	}
	return a.lifecycle(ctx, p)
}
