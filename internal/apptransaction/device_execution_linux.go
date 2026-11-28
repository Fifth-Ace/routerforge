//go:build linux

package apptransaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const deviceWorkerPath = "/opt/bin/rf-app-worker"

var ErrExecutionLocked = errors.New("hardware acceptance policy does not authorize execution")

// An acceptance document is independent of the installed package: embedding
// a record of its own exact commit in the source would be self-referential.
// No package, HTTP handler or worker creates this document. Root-controlled
// explicit hardware approval is the trust boundary; root can falsify evidence.
// The evidence digest identifies the reviewed report, not its truthfulness.
const acceptancePolicyPath = "/opt/var/routerforge-app-transactions/accepted-execution-policy.json"

type acceptedExecution struct {
	WorkerVersion string                  `json:"worker_version"`
	Commit        string                  `json:"commit"`
	Packages      []Package               `json:"packages"`
	Initial       CooperativeVersionSet   `json:"initial"`
	Accepted      []CooperativeVersionSet `json:"accepted"`
}
type deviceAcceptancePolicy struct {
	Schema         int                 `json:"schema"`
	Approval       string              `json:"approval"`
	EvidenceSHA256 string              `json:"evidence_sha256"`
	WorkerVersion  string              `json:"worker_version"`
	WorkerSHA256   string              `json:"worker_sha256"`
	Entries        []acceptedExecution `json:"entries"`
}

func executionDeviceScope(version string) bool {
	if runtime.GOOS != "linux" || runtime.GOARCH != "arm64" || os.Geteuid() != 0 {
		return false
	}
	_, err := devRevision(version)
	return err == nil
}

// Existing but unreadable/invalid policy blocks old official update path too.
// A new Core version cannot silently fall back to legacy mutation.
func DeviceExecutionPolicyActive(version string) bool {
	if !executionDeviceScope(version) {
		return false
	}
	_, err := os.Lstat(acceptancePolicyPath)
	return !errors.Is(err, os.ErrNotExist)
}
func DeviceExecutionConfigured(version string) bool {
	if !executionDeviceScope(version) {
		return false
	}
	policy, err := readDeviceAcceptance()
	return err == nil && policy.WorkerVersion == version
}
func readDeviceAcceptance() (deviceAcceptancePolicy, error) {
	var zero deviceAcceptancePolicy
	dir, err := filepath.EvalSymlinks(filepath.Dir(acceptancePolicyPath))
	if err != nil {
		return zero, ErrExecutionLocked
	}
	return readAcceptancePolicy(dir)
}
func readAcceptancePolicy(dir string) (deviceAcceptancePolicy, error) {
	var p deviceAcceptancePolicy
	if _, err := Open(dir); err != nil {
		return p, err
	}
	path := filepath.Join(dir, "accepted-execution-policy.json")
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return p, err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !privateRegular(st) || st.Size() > maxJournal {
		return p, errors.New("unsafe acceptance policy")
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxJournal+1))
	if err != nil || int64(len(raw)) != st.Size() {
		return p, errors.New("acceptance policy read failed")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(st, after) || st.ModTime() != after.ModTime() || st.Mode() != after.Mode() || st.Size() != after.Size() {
		return p, errors.New("acceptance policy changed")
	}
	// Canonical JSON rejects duplicate/unknown keys, including nested ones.
	if err = json.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	canonical, err := json.Marshal(p)
	if err != nil || strings.TrimSpace(string(raw)) != string(canonical) {
		return p, errors.New("noncanonical acceptance policy")
	}
	if p.Schema != 1 || p.Approval != "RF_HARDWARE_ACCEPTED" || !digest.MatchString(p.EvidenceSHA256) || !digest.MatchString(p.WorkerSHA256) || len(p.Entries) == 0 || len(p.Entries) > 32 {
		return p, ErrExecutionLocked
	}
	if _, err = devRevision(p.WorkerVersion); err != nil {
		return p, err
	}
	for _, e := range p.Entries {
		if e.WorkerVersion != p.WorkerVersion {
			return p, ErrExecutionLocked
		}
		plan := Plan{ID: "acceptance", Channel: "dev", Target: "aarch64-3.10", Commit: e.Commit, Packages: e.Packages}
		if _, err = selectExecutionPolicy(p.WorkerVersion, plan, e.Initial, p.Entries); err != nil {
			return p, err
		}
	}
	return p, nil
}

func copyVersionSet(in CooperativeVersionSet) CooperativeVersionSet {
	out := CooperativeVersionSet{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
func selectExecutionPolicy(version string, p Plan, installed CooperativeVersionSet, entries []acceptedExecution) ([]CooperativeVersionSet, error) {
	if _, err := p.Hash(); err != nil {
		return nil, err
	}
	if err := validCooperativeSet(installed); err != nil {
		return nil, err
	}
	// The initial hardware campaign covers all eleven installed packages. No
	// ambiguous absence, legacy aliases, install/remove or partial inventory.
	for _, v := range installed {
		if v == "" {
			return nil, ErrExecutionLocked
		}
	}
	if installed["routerforge-core"] != version {
		return nil, ErrExecutionLocked
	}
	var result []CooperativeVersionSet
	for _, e := range entries {
		if e.WorkerVersion != version || e.Commit != p.Commit || len(e.Packages) != len(p.Packages) || !equalCooperativeSets(e.Initial, installed) {
			continue
		}
		match := true
		for i, pkg := range p.Packages {
			if e.Packages[i] != pkg {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if err := CheckCooperativeTransition(p, installed, e.Accepted); err != nil {
			return nil, err
		}
		if result != nil {
			return nil, errors.New("ambiguous execution acceptance policy")
		}
		current := copyVersionSet(installed)
		result = []CooperativeVersionSet{copyVersionSet(current)}
		for _, pkg := range p.Packages {
			current[pkg.Name] = pkg.Version
			result = append(result, copyVersionSet(current))
		}
	}
	if result == nil {
		return nil, ErrExecutionLocked
	}
	return result, nil
}

func devicePackageNames() []string {
	names := make([]string, 0, len(official))
	for name := range official {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func parseDeviceInstalledList(raw string) (CooperativeVersionSet, error) {
	out := CooperativeVersionSet{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		name, version, ok := strings.Cut(line, " - ")
		if !ok || name == "" || strings.TrimSpace(name) != name {
			return nil, errors.New("invalid installed package list")
		}
		if strings.HasPrefix(name, "dns-monitor") || (strings.HasPrefix(name, "routerforge-") && !official[name]) {
			return nil, fmt.Errorf("unsupported legacy or unknown RouterForge package: %s", name)
		}
		if !official[name] {
			continue
		}
		if _, exists := out[name]; exists {
			return nil, errors.New("ambiguous official package list")
		}
		if _, err := devRevision(version); err != nil {
			return nil, err
		}
		out[name] = version
	}
	if err := validCooperativeSet(out); err != nil {
		return nil, err
	}
	return out, nil
}
func readDeviceVersionSet(ctx context.Context) (CooperativeVersionSet, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd, err := safety.CommandContext(ctx, "/opt/bin/opkg", "list-installed")
	if err != nil {
		return nil, err
	}
	var out opkgOutput
	cmd.Stdout = &out
	if err = cmd.Run(); err != nil || out.overflow {
		return nil, errors.New("installed list unavailable")
	}
	versions, err := parseDeviceInstalledList(string(out.data))
	if err != nil {
		return nil, err
	}
	for _, name := range devicePackageNames() {
		v, err := readInstalledDevVersion(ctx, "/opt/bin/opkg", name)
		if err != nil || v != versions[name] {
			return nil, fmt.Errorf("installed version not independently confirmed: %s", name)
		}
	}
	return versions, nil
}

// CheckDeviceExecution performs no mutations or launch. The caller must hold
// the cooperative lease when using it for admission, not merely UI display.
func CheckDeviceExecution(ctx context.Context, version string, p Plan) error {
	if !DeviceExecutionConfigured(version) {
		return ErrExecutionLocked
	}
	v, err := readDeviceVersionSet(ctx)
	if err != nil {
		return err
	}
	policy, err := readDeviceAcceptance()
	if err != nil || policy.WorkerVersion != version {
		return ErrExecutionLocked
	}
	_, err = selectExecutionPolicy(version, p, v, policy.Entries)
	if err != nil {
		return err
	}
	return trustedDeviceWorker(policy.WorkerSHA256)
}

func trustedDeviceWorker(expected string) error {
	if !digest.MatchString(expected) || !safeEvidenceDirectory("/opt/bin", 0, false) {
		return errors.New("unsafe worker directory or digest")
	}
	fd, err := syscall.Open(deviceWorkerPath, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), deviceWorkerPath)
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || !coreEvidenceOwned(st, 0) || st.Mode().Perm()&0022 != 0 || st.Mode().Perm()&0111 == 0 || st.Size() <= 0 || st.Size() > 64<<20 {
		return errors.New("independent worker unavailable")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, (64<<20)+1))
	if err != nil || n != st.Size() || hex.EncodeToString(h.Sum(nil)) != expected {
		return errors.New("worker artifact digest mismatch")
	}
	after, err := os.Lstat(deviceWorkerPath)
	if err != nil || !os.SameFile(st, after) || st.ModTime() != after.ModTime() || st.Size() != after.Size() || st.Mode() != after.Mode() {
		return errors.New("worker artifact changed")
	}
	return nil
}

// Launch is fixed to the separately packaged executable. No HTTP-supplied
// command, executable, path or environment can reach this function. Recheck
// the exact acceptance policy under the same lease as durable slot creation.
func (c *Coordinator) StartDeviceExecution(ctx context.Context, version string, p Plan, revalidate func() error) (int, error) {
	if !DeviceExecutionConfigured(version) {
		return 0, ErrExecutionLocked
	}
	if revalidate == nil {
		return 0, errors.New("missing confirmed plan revalidation")
	}
	return c.startChecked(deviceWorkerPath, nil, p, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := revalidate(); err != nil {
			return err
		}
		if err := CheckDeviceExecution(ctx, version, p); err != nil {
			return err
		}
		return ctx.Err()
	})
}

// Read-only observations are allowed to settle within the existing per-probe
// deadline after a service restart. Never retry an installation.
func acceptedDeviceProbe(ctx context.Context, p Package) (LifecycleObservation, error) {
	for {
		v, err := observeDeviceLifecycle(ctx, p)
		if err == nil {
			return v, nil
		}
		if ctx.Err() != nil {
			return LifecycleObservation{}, err
		}
		select {
		case <-ctx.Done():
			return LifecycleObservation{}, err
		case <-time.After(400 * time.Millisecond):
		}
	}
}
func observeDeviceLifecycle(ctx context.Context, p Package) (LifecycleObservation, error) {
	installed, err := readInstalledDevVersion(ctx, "/opt/bin/opkg", p.Name)
	if err != nil {
		return LifecycleObservation{}, err
	}
	if specialLifecyclePackage(p.Name) {
		core := installed
		if p.Name == "routerforge-profiling" {
			core, err = readInstalledDevVersion(ctx, "/opt/bin/opkg", "routerforge-core")
			if err != nil {
				return LifecycleObservation{}, err
			}
		}
		return ProbeSpecialLifecycleCandidate(ctx, p.Name, installed, core)
	}
	return probeRunningBootIntent(ctx, p.Name, installed)
}

// Device execution uses the same tested verified adapter as the isolated OPKG
// acceptance, plus an immutable compatibility sequence and final re-observation
// of every installed module. Running-only native observations are candidates;
// hardware acceptance must prove their sufficiency before any entry is added.
func (c *Coordinator) RunAcceptedDeviceWorker(ctx context.Context, version string) (Journal, error) {
	if !DeviceExecutionConfigured(version) {
		return Journal{}, ErrExecutionLocked
	}
	policy, err := readDeviceAcceptance()
	if err != nil || policy.WorkerVersion != version {
		return Journal{}, ErrExecutionLocked
	}
	if err = trustedDeviceWorker(policy.WorkerSHA256); err != nil {
		return Journal{}, err
	}
	self, err := os.Stat("/proc/self/exe")
	if err != nil {
		return Journal{}, err
	}
	installedWorker, err := os.Lstat(deviceWorkerPath)
	if err != nil || !os.SameFile(self, installedWorker) {
		return Journal{}, errors.New("worker executable identity mismatch")
	}
	return c.runVerifiedWorker(ctx, acceptedDeviceProbe, func(p Plan, h *lifecycleProofHooks) (Hooks, error) {
		installed, err := readDeviceVersionSet(ctx)
		if err != nil {
			return nil, err
		}
		sequence, err := selectExecutionPolicy(version, p, installed, policy.Entries)
		if err != nil {
			return nil, err
		}
		boundary, err := newExecutionBoundary(ctx, p, h, sequence, readDeviceVersionSet, func(ctx context.Context, pkg Package) (lifecycleEvidence, error) { return h.proof.observe(ctx, pkg) })
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(boundary.before)
		if err != nil {
			return nil, err
		}
		if err = durableExclusive(h.proof.store, "execution-before.json", b); err != nil {
			return nil, err
		}
		boundary.saveFinal = func(evidence map[string]lifecycleEvidence) error {
			b, err := json.Marshal(evidence)
			if err != nil {
				return err
			}
			return durableExclusive(h.proof.store, "execution-after.json", b)
		}
		return boundary, nil
	})
}

// executionBoundary never changes the plan or resumes a checkpoint. It also
// checks unselected packages, because a Core/Profiling postinst restarts Core
// and may affect other modules. A failure after a mutation is recovery-required.
type executionBoundary struct {
	Hooks
	plan      Plan
	sequence  []CooperativeVersionSet
	read      func(context.Context) (CooperativeVersionSet, error)
	observe   func(context.Context, Package) (lifecycleEvidence, error)
	before    map[string]lifecycleEvidence
	saveFinal func(map[string]lifecycleEvidence) error
	next      int
}

func newExecutionBoundary(ctx context.Context, p Plan, h Hooks, sequence []CooperativeVersionSet, read func(context.Context) (CooperativeVersionSet, error), observe func(context.Context, Package) (lifecycleEvidence, error)) (*executionBoundary, error) {
	if h == nil || read == nil || observe == nil || len(sequence) != len(p.Packages)+1 {
		return nil, errors.New("invalid execution boundary")
	}
	e := &executionBoundary{Hooks: h, plan: p, sequence: sequence, read: read, observe: observe, before: map[string]lifecycleEvidence{}}
	if err := e.checkVector(ctx, 0); err != nil {
		return nil, err
	}
	for _, name := range devicePackageNames() {
		v, err := observe(ctx, Package{Name: name})
		if err != nil {
			return nil, fmt.Errorf("initial lifecycle %s: %w", name, err)
		}
		if v.InstalledVersion != sequence[0][name] {
			return nil, errors.New("initial lifecycle version drift")
		}
		e.before[name] = v
	}
	if err := e.checkVector(ctx, 0); err != nil {
		return nil, err
	}
	return e, nil
}
func (e *executionBoundary) checkVector(ctx context.Context, index int) error {
	v, err := e.read(ctx)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(e.sequence) || !equalCooperativeSets(v, e.sequence[index]) {
		return errors.New("accepted package version vector drift")
	}
	return nil
}
func sameIntent(a, b LifecycleObservation) bool {
	return a.Mode == b.Mode && a.Enabled == b.Enabled && a.Ready == b.Ready && a.IntentSHA256 == b.IntentSHA256
}
func (e *executionBoundary) checkLifecycle(ctx context.Context, index int) error {
	final := map[string]lifecycleEvidence{}
	for _, name := range devicePackageNames() {
		old := e.before[name]
		now, err := e.observe(ctx, Package{Name: name})
		if err != nil {
			return fmt.Errorf("lifecycle %s: %w", name, err)
		}
		if now.InstalledVersion != e.sequence[index][name] || !sameIntent(old.Observation, now.Observation) {
			return fmt.Errorf("lifecycle drift: %s", name)
		}
		final[name] = now
	}
	if err := e.checkVector(ctx, index); err != nil {
		return err
	}
	if e.saveFinal != nil {
		return e.saveFinal(final)
	}
	return nil
}
func (e *executionBoundary) Check(ctx context.Context, p Package) error {
	if e.next >= len(e.plan.Packages) || e.plan.Packages[e.next] != p {
		return errors.New("execution order drift")
	}
	if err := e.checkVector(ctx, e.next); err != nil {
		return err
	}
	return e.Hooks.Check(ctx, p)
}
func (e *executionBoundary) Install(ctx context.Context, p Package) error {
	if e.next >= len(e.plan.Packages) || e.plan.Packages[e.next] != p {
		return errors.New("execution order drift")
	}
	if err := e.checkVector(ctx, e.next); err != nil {
		return err
	}
	return e.Hooks.Install(ctx, p)
}
func (e *executionBoundary) Verify(ctx context.Context, p Package) error {
	if e.next >= len(e.plan.Packages) || e.plan.Packages[e.next] != p {
		return errors.New("execution order drift")
	}
	if err := e.Hooks.Verify(ctx, p); err != nil {
		return err
	}
	if err := e.checkVector(ctx, e.next+1); err != nil {
		return err
	}
	if e.next+1 == len(e.plan.Packages) {
		if err := e.checkLifecycle(ctx, e.next+1); err != nil {
			return err
		}
	}
	e.next++
	return nil
}
