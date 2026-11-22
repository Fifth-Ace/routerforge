package dnsrouter

// DPR2-O2: opt-in, DSTest-only persistent activation of owned DNS zones.
// No automatic startup recovery, no Keenetic configuration persistence, and no port 53
// listener. DPR2-O3 will implement explicit reboot/startup recovery.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

func hashSnapshotO2(snapshot string) string {
	sum := sha256.Sum256([]byte(snapshot))
	return "sha256:" + hex.EncodeToString(sum[:])
}

type DurableManagerO2 struct {
	Store      *ConfigStore
	Ledger     *LedgerStoreO
	Lifecycle  *LifecycleManager
	Executor   NDMCExecutorM
	Arm        managedArmN
	ArmDisable func(context.Context, durableJournalO2) (managedGuardN, error)
	Probe      managedProbeN
	Ready      func(string, string) bool // test fixture, never wired from main
	HostGate   func() error              // test fixture, main uses NDMCTestHostGateM
}

type DurableResultO2 struct {
	State            string   `json:"state"`
	Revision         string   `json:"revision,omitempty"`
	LedgerRevision   string   `json:"ledger_revision,omitempty"`
	Zones            []string `json:"zones"`
	WatchdogArmed    bool     `json:"watchdog_armed"`
	BaselineRestored bool     `json:"baseline_restored"`
	DurableChange    bool     `json:"durable_change"`
}

// Sharing the exact file lock with DPR2-M/N excludes any concurrent managed
// trial, not just another request handled by this Go process.
func lockNDMCForO2() (func(), error) {
	trialGlobalMuM.Lock()
	fd, err := syscall.Open("/tmp/routerforge-dpr2m-trial.lock", syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		trialGlobalMuM.Unlock()
		return nil, err
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		syscall.Close(fd)
		trialGlobalMuM.Unlock()
		return nil, fmt.Errorf("another DNS transaction owns lock: %w", err)
	}
	return func() { _ = syscall.Flock(fd, syscall.LOCK_UN); _ = syscall.Close(fd); trialGlobalMuM.Unlock() }, nil
}

func (m *DurableManagerO2) guard() error {
	if m == nil || m.Store == nil || m.Ledger == nil || m.Executor == nil || m.Arm == nil || m.ArmDisable == nil || m.Probe == nil {
		return errors.New("persistent managed capability not fully configured")
	}
	if m.HostGate == nil {
		return errors.New("DSTest host gate is not configured")
	}
	return m.HostGate()
}

func (m *DurableManagerO2) trial() *ManagedTrialN {
	return &ManagedTrialN{Store: m.Store, Lifecycle: m.Lifecycle, Executor: m.Executor, Arm: m.Arm, Probe: m.Probe, Ready: m.Ready, MaxZones: 63}
}

func (m *DurableManagerO2) Prepare(ctx context.Context, rev string) (ManagedPrepareN, error) {
	if err := m.guard(); err != nil {
		return ManagedPrepareN{}, err
	}
	ledger, _, err := m.Ledger.Load()
	if err != nil {
		return ManagedPrepareN{}, err
	}
	if ledger != nil {
		return ManagedPrepareN{}, errors.New("ownership ledger exists: disable or recover before new activation")
	}
	return m.trial().Prepare(ctx, rev)
}

func zonesFromLedgerO2(ledger LedgerO) []DomainSplitZoneL {
	zs := make([]DomainSplitZoneL, 0, len(ledger.Zones))
	for _, d := range ledger.Zones {
		zs = append(zs, DomainSplitZoneL{Domain: d, Add: "ip name-server " + ledger.Listen + " " + d, Remove: "no ip name-server " + ledger.Listen + " " + d})
	}
	return zs
}

// Snapshot without exactly the records owned by this ledger must equal the
// saved baseline. Extra or foreign entries make activation/disable refuse.
func withoutOwnedO2(current string, ledger LedgerO) (string, error) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(current), &doc); err != nil {
		return "", err
	}
	raw, ok := doc["server"].([]any)
	if !ok {
		return "", errors.New("missing ndmc server array")
	}
	host, portText, err := net.SplitHostPort(ledger.Listen)
	if err != nil {
		return "", err
	}
	var port int
	if _, err = fmt.Sscanf(portText, "%d", &port); err != nil {
		return "", err
	}
	owned := make(map[string]int, len(ledger.Zones))
	for _, z := range ledger.Zones {
		owned[z] = 0
	}
	filtered := make([]any, 0, len(raw))
	for _, entry := range raw {
		row, ok := entry.(map[string]any)
		if !ok {
			return "", errors.New("ndmc server is not a map")
		}
		d, _ := row["domain"].(string)
		if _, isOwned := owned[d]; isOwned {
			addr, _ := row["address"].(string)
			p, ok := row["port"].(float64)
			if !ok || addr != host || p != float64(port) {
				return "", fmt.Errorf("zone %q changed owner", d)
			}
			owned[d]++
		} else {
			filtered = append(filtered, entry)
		}
	}
	for z, n := range owned {
		if n != 1 {
			return "", fmt.Errorf("zone %q expected once, got %d", z, n)
		}
	}
	doc["server"] = filtered
	b, err := json.Marshal(doc)
	return string(b), err
}

func ensureExactOwnedO2(ctx context.Context, executor NDMCExecutorM, ledger LedgerO) (string, error) {
	current, err := trialSnapshotM(ctx, executor)
	if err != nil {
		return "", err
	}
	stripped, err := withoutOwnedO2(current, ledger)
	if err != nil {
		return "", err
	}
	if stripped != ledger.Baseline {
		return "", errors.New("foreign DNS inventory drift: refusing to alter any registration")
	}
	return current, nil
}

func (m *DurableManagerO2) Apply(ctx context.Context, rev, token string) (out DurableResultO2, retErr error) {
	out = DurableResultO2{State: "precheck", Zones: []string{}, DurableChange: false}
	if err := m.guard(); err != nil {
		return out, err
	}
	unlock, err := lockNDMCForO2()
	if err != nil {
		return out, err
	}
	defer unlock()
	old, _, err := m.Ledger.Load()
	if err != nil {
		return out, err
	}
	if old != nil {
		return out, errors.New("ownership exists: refuse overwrite")
	}
	plan, base, err := m.trial().preflight(ctx, rev)
	if err != nil {
		return out, err
	}
	if !plan.Ready {
		return out, fmt.Errorf("managed plan conflicts: %v", plan.Conflicts)
	}
	if token == "" || token != plan.Token {
		return out, ErrManagedTokenN
	}
	out.Revision = plan.Revision
	for _, z := range plan.Zones {
		out.Zones = append(out.Zones, z.Domain)
	}
	sort.Strings(out.Zones)
	ledger := LedgerO{SchemaVersion: 1, State: "pending", Revision: rev, Listen: plan.Listen, Zones: out.Zones, Baseline: base, BaselineHash: hashSnapshotO2(base)}
	pendingSHA, err := m.Ledger.SaveCAS(ledger, "")
	if err != nil {
		return out, err
	}
	// Journal is durable before any ndmc mutation.
	armed := false
	guard, err := m.Arm(ctx, managedJournalN{Extended63: true, Parent: os.Getpid(), Deadline: time.Now().Add(115 * time.Second).Unix(), Baseline: base, Listen: plan.Listen, Domains: out.Zones})
	if err != nil {
		// No mutation has been attempted. Do not discard a questionable ledger
		// if the inventory is no longer identical.
		if snap, e := trialSnapshotM(ctx, m.Executor); e == nil && snap == base {
			_ = m.Ledger.DeleteCAS(pendingSHA)
		}
		return out, fmt.Errorf("activation watchdog unavailable: %w", err)
	}
	armed = true
	out.WatchdogArmed = true
	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		zones := zonesFromLedgerO2(ledger)
		if err := rollbackManagedN(rollbackCtx, m.Executor, base, plan.Listen, zones); err != nil {
			out.State = "ambiguous"
			retErr = fmt.Errorf("activation failed (%v); rollback failed: %w", retErr, err)
			return // preserve pending ledger and independent watchdog
		}
		out.BaselineRestored = true
		current, sha, e := m.Ledger.Load()
		if e != nil || current == nil || current.State != "pending" || sha != pendingSHA {
			out.State = "ambiguous"
			retErr = fmt.Errorf("activation ledger changed during rollback: %v", e)
			return
		}
		if e = m.Ledger.DeleteCAS(sha); e != nil {
			out.State = "ambiguous"
			retErr = fmt.Errorf("ledger removal failed: %w", e)
			return
		}
		if armed {
			if e = guard.Finish(); e != nil {
				out.State = "ambiguous"
				retErr = fmt.Errorf("watchdog acknowledgement failed: %w", e)
				return
			}
		}
		out.State = "rolled_back_after_failure"
	}()
	workCtx, cancel := context.WithTimeout(ctx, 65*time.Second)
	defer cancel()
	for _, z := range plan.Zones {
		if _, err = m.Executor.Command(workCtx, z.Add); err != nil {
			return out, err
		}
	}
	verified := false
	for i := 0; i < 6; i++ {
		snap, e := trialSnapshotM(workCtx, m.Executor)
		if e == nil {
			verified, e = containsAllManagedN(snap, plan.Zones, plan.Listen)
		}
		if e == nil && verified {
			break
		}
		select {
		case <-workCtx.Done():
			return out, workCtx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	if !verified {
		return out, errors.New("ndmc did not register all zones")
	}
	if err = m.Probe(workCtx, out.Zones); err != nil {
		return out, fmt.Errorf("DNS forwarding: %w", err)
	}
	if _, err = ensureExactOwnedO2(workCtx, m.Executor, ledger); err != nil {
		return out, err
	}
	ledger.State = "active"
	activeSHA, err := m.Ledger.SaveCAS(ledger, pendingSHA)
	if err != nil {
		return out, err
	}
	// If guard Finish fails, never claim PASS. An orphaned guard may still
	// restore baseline. Keep the active ledger for subsequent reconciliation.
	committed = true
	out.LedgerRevision = activeSHA
	if err = guard.Finish(); err != nil {
		out.State = "ambiguous"
		return out, fmt.Errorf("activation committed but watchdog ack failed: %w", err)
	}
	out.State = "active"
	out.DurableChange = true
	return out, nil
}

// Deactivation journals its intent before mutations. Its independent watchdog
// does the REVERSE of activation rollback: it re-adds missing OWNED zones.
func (m *DurableManagerO2) Disable(ctx context.Context, rev, expectedLedgerSHA string) (out DurableResultO2, retErr error) {
	out = DurableResultO2{State: "precheck", Zones: []string{}, DurableChange: false}
	if err := m.guard(); err != nil {
		return out, err
	}
	unlock, err := lockNDMCForO2()
	if err != nil {
		return out, err
	}
	defer unlock()
	ledger, sha, err := m.Ledger.Load()
	if err != nil {
		return out, err
	}
	if ledger == nil || ledger.State != "active" || ledger.Baseline == "" {
		return out, errors.New("no active O2 ownership ledger")
	}
	if rev == "" || rev != ledger.Revision {
		return out, ErrStale
	}
	cfg, configRev, err := m.Store.Load()
	if err != nil {
		return out, err
	}
	_ = cfg
	if configRev != rev {
		return out, ErrStale
	}
	if expectedLedgerSHA == "" || expectedLedgerSHA != sha {
		return out, ErrStale
	}
	before, err := ensureExactOwnedO2(ctx, m.Executor, *ledger)
	if err != nil {
		return out, err
	}
	out.Revision = rev
	out.Zones = append(out.Zones, ledger.Zones...)
	staged := *ledger
	staged.State = "deactivating"
	stageSHA, err := m.Ledger.SaveCAS(staged, sha)
	if err != nil {
		return out, err
	}
	journal := durableJournalO2{Parent: os.Getpid(), Deadline: time.Now().Add(115 * time.Second).Unix(), Before: before, Ledger: staged}
	guard, err := m.ArmDisable(ctx, journal)
	if err != nil {
		// No deletion attempted. Preserve active ownership if state unchanged.
		staged.State = "active"
		_, _ = m.Ledger.SaveCAS(staged, stageSHA)
		return out, fmt.Errorf("deactivation watchdog unavailable: %w", err)
	}
	out.WatchdogArmed = true
	committed := false
	defer func() {
		if committed {
			return
		}
		// In case a command fails, attempt exact reconstruction while the
		// independently armed watchdog remains available.
		restoreCtx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		if e := restoreOwnedO2(restoreCtx, m.Executor, journal); e != nil {
			out.State = "ambiguous"
			retErr = fmt.Errorf("disable failed (%v); restore failed: %w", retErr, e)
			return
		}
		staged.State = "active"
		activeSHA, e := m.Ledger.SaveCAS(staged, stageSHA)
		if e != nil {
			out.State = "ambiguous"
			retErr = fmt.Errorf("restore ledger failed: %w", e)
			return
		}
		out.LedgerRevision = activeSHA
		if e = guard.Finish(); e != nil {
			out.State = "ambiguous"
			retErr = fmt.Errorf("disable guard ack failed: %w", e)
			return
		}
		out.State = "restored_after_failure"
	}()
	workCtx, cancel := context.WithTimeout(ctx, 65*time.Second)
	defer cancel()
	zones := zonesFromLedgerO2(*ledger)
	if err = rollbackManagedN(workCtx, m.Executor, ledger.Baseline, ledger.Listen, zones); err != nil {
		return out, err
	}
	out.BaselineRestored = true
	// Stop the reverse watchdog BEFORE removing the ownership journal.
	// Otherwise an unacknowledged watchdog could recreate zones after we
	// have already discarded the only ownership record.
	if err = guard.Finish(); err != nil {
		committed = true // leave deactivating ledger for explicit recovery
		out.State = "ambiguous"
		return out, fmt.Errorf("zones removed but disable watchdog ack failed; ledger retained: %w", err)
	}
	committed = true
	if err = m.Ledger.DeleteCAS(stageSHA); err != nil {
		out.State = "ambiguous"
		return out, fmt.Errorf("zones removed; safe deactivating ledger retained: %w", err)
	}
	out.State = "disabled"
	out.DurableChange = true
	return out, nil
}

type durableJournalO2 struct {
	Parent   int     `json:"parent"`
	Deadline int64   `json:"deadline"`
	Before   string  `json:"before"`
	Ledger   LedgerO `json:"ledger"`
}

// restoreOwnedO2 only re-adds ledger-listed registrations that are absent.
// It refuses foreign owner drift and validates the exact full pre-disable state.
func restoreOwnedO2(ctx context.Context, executor NDMCExecutorM, j durableJournalO2) error {
	current, err := trialSnapshotM(ctx, executor)
	if err != nil {
		return err
	}
	servers, err := snapshotDomainsN(current)
	if err != nil {
		return err
	}
	host, portText, err := net.SplitHostPort(j.Ledger.Listen)
	if err != nil {
		return err
	}
	var port int
	if _, err = fmt.Sscanf(portText, "%d", &port); err != nil {
		return err
	}
	for _, z := range zonesFromLedgerO2(j.Ledger) {
		present := 0
		for _, s := range servers {
			if s.Domain == z.Domain {
				if s.Address != host || s.Port != port {
					return fmt.Errorf("zone %s has foreign owner", z.Domain)
				}
				present++
			}
		}
		if present > 1 {
			return fmt.Errorf("zone %s duplicated", z.Domain)
		}
		if present == 0 {
			if _, err = executor.Command(ctx, z.Add); err != nil {
				return err
			}
		}
	}
	after, err := trialSnapshotM(ctx, executor)
	if err != nil {
		return err
	}
	if after != j.Before {
		return errors.New("deactivation rollback did not restore exact pre-disable DNS inventory")
	}
	return nil
}

type durableGuardO2 struct {
	dir  string
	wait func() error
}

func (g *durableGuardO2) Finish() error {
	if err := safety.WriteFileAtomic(filepath.Join(g.dir, "complete"), []byte("verified\n"), 0600); err != nil {
		return err
	}
	result := make(chan error, 1)
	go func() { result <- g.wait() }()
	select {
	case err := <-result:
		if err != nil {
			return err
		}
		return os.RemoveAll(g.dir)
	case <-time.After(5 * time.Second):
		return errors.New("durable disable watchdog did not terminate")
	}
}

func ArmDisableO2(ctx context.Context, j durableJournalO2) (managedGuardN, error) {
	dir, err := os.MkdirTemp("/tmp", "rf-dpr2o2-guard-")
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(dir, 0700); err != nil {
		return nil, err
	}
	b, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	if err = safety.WriteFileAtomic(filepath.Join(dir, "journal.json"), b, 0600); err != nil {
		return nil, err
	}
	// CommandContext retains RouterForge P15 safety boundary, no shell.
	cmd, err := safety.CommandContext(context.Background(), "/proc/self/exe", "--ndmc-durable-disable-worker", dir)
	if err != nil {
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	for i := 0; i < 50; i++ {
		if _, err = os.Stat(filepath.Join(dir, "ready")); err == nil {
			return &durableGuardO2{dir: dir, wait: cmd.Wait}, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil, errors.New("durable disable watchdog not ready; journal preserved")
}

func DurableDisableWorkerO2(dir string) error {
	if err := NDMCTestHostGateM(); err != nil {
		return err
	}
	st, err := os.Lstat(dir)
	if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid guard directory")
	}
	b, err := os.ReadFile(filepath.Join(dir, "journal.json"))
	if err != nil {
		return err
	}
	var j durableJournalO2
	if err = json.Unmarshal(b, &j); err != nil {
		return err
	}
	if err = validateLedgerO(j.Ledger); err != nil {
		return err
	}
	if j.Ledger.State != "deactivating" || j.Ledger.Baseline == "" || j.Parent < 1 || j.Deadline <= time.Now().Unix() || j.Deadline > time.Now().Add(120*time.Second).Unix() {
		return errors.New("invalid disable lease")
	}
	if _, err = canonicalNDMCSnapshotM(j.Before); err != nil {
		return err
	}
	baseline, err := withoutOwnedO2(j.Before, j.Ledger)
	if err != nil || baseline != j.Ledger.Baseline {
		return errors.New("journal before state does not match owned baseline")
	}
	if err = safety.WriteFileAtomic(filepath.Join(dir, "ready"), []byte("armed\n"), 0600); err != nil {
		return err
	}
	for {
		if _, err = os.Stat(filepath.Join(dir, "complete")); err == nil {
			return nil
		}
		if err = syscall.Kill(j.Parent, 0); err != nil || time.Now().Unix() >= j.Deadline {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	err = restoreOwnedO2(ctx, realNDMCM{}, j)
	msg := "WATCHDOG_O2_RESTORE_PASS\n"
	if err != nil {
		msg = "WATCHDOG_O2_RESTORE_FAIL: " + err.Error() + "\n"
	}
	_ = safety.WriteFileAtomic(filepath.Join(dir, "result.txt"), []byte(msg), 0600)
	return err
}

// Only CLI's exact opt-in flag may instantiate this. No autostart at O2.
func NewRealDurableO2(store *ConfigStore, ledger *LedgerStoreO, life *LifecycleManager) *DurableManagerO2 {
	return &DurableManagerO2{Store: store, Ledger: ledger, Lifecycle: life, Executor: NewRealNDMCM(), Arm: ArmOSGuardN, ArmDisable: ArmDisableO2, Probe: ProbeManagedN(life), HostGate: NDMCTestHostGateM}
}
