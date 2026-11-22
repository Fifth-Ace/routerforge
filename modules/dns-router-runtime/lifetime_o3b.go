package dnsrouter

// DPR2-O3B: opt-in, independent, DSTest-only daemon lifetime guardian.
// A dead DNS forwarder must not leave an owned ndnproxy zone pointing to it.
// The durable ownership ledger is retained for verified startup recovery.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const lifetimePrefixO3B = "/tmp/rf-dpr2o3-lifetime-"

type lifetimeJournalO3B struct {
	Schema      int    `json:"schema"`
	Parent      int    `json:"parent"`
	ParentStart string `json:"parent_start"`
	LedgerPath  string `json:"ledger_path"`
	Listen      string `json:"listen"`
	Started     int64  `json:"started"`
}

type LifetimeResultO3B struct {
	State          string   `json:"state"`
	Zones          []string `json:"zones"`
	LedgerRetained bool     `json:"ledger_retained"`
	ExactBaseline  bool     `json:"exact_baseline"`
}

// A PID alone is not a lifetime identity: Linux may reuse it after a crash.
func procStartO3B(pid int) (string, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	index := strings.LastIndexByte(string(b), ')')
	if index < 0 {
		return "", errors.New("invalid proc stat")
	}
	fields := strings.Fields(string(b[index+1:]))
	if len(fields) <= 19 {
		return "", errors.New("missing proc starttime")
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return "", err
	}
	return fields[19], nil
}

func isParentAliveO3B(j lifetimeJournalO3B) bool {
	start, err := procStartO3B(j.Parent)
	return err == nil && start == j.ParentStart
}

func validLifetimeJournalO3B(j lifetimeJournalO3B) error {
	if j.Schema != 1 || j.Parent < 2 || j.ParentStart == "" || j.Started <= 0 || j.Listen != "192.168.10.1:55358" {
		return errors.New("invalid lifetime journal identity")
	}
	if !filepath.IsAbs(j.LedgerPath) || filepath.Clean(j.LedgerPath) != j.LedgerPath ||
		!strings.HasSuffix(j.LedgerPath, ".managed-state.json") ||
		!(strings.HasPrefix(j.LedgerPath, "/opt/etc/routerforge/dns-router/") ||
			strings.HasPrefix(j.LedgerPath, "/tmp/rf-")) {
		return errors.New("lifetime ledger outside experimental roots")
	}
	return nil
}

func currentShortGuardsO3B() ([]string, bool, error) {
	var paths []string
	for _, pattern := range []string{"/tmp/rf-dpr2n-guard-*", "/tmp/rf-dpr2o2-guard-*"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, false, err
		}
		paths = append(paths, matches...)
	}
	for _, path := range paths {
		st, err := os.Lstat(path)
		if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return nil, false, fmt.Errorf("invalid short guard: %s", path)
		}
		b, err := os.ReadFile(filepath.Join(path, "result.txt"))
		if errors.Is(err, os.ErrNotExist) {
			return paths, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		result := strings.TrimSpace(string(b))
		if result != "WATCHDOG_N_ROLLBACK_PASS" && result != "WATCHDOG_O2_RESTORE_PASS" {
			return nil, false, fmt.Errorf("short watchdog has unsafe result: %s", path)
		}
	}
	return paths, true, nil
}

func awaitShortGuardsO3B(ctx context.Context) ([]string, error) {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		paths, ready, err := currentShortGuardsO3B()
		if err != nil {
			return nil, err
		}
		if ready {
			return paths, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tick.C:
		}
	}
}

// CleanupOnDaemonExitO3B is deliberately strict: ownership is never inferred.
// Ledger is retained, allowing O3 startup to restore after a later reboot.
func cleanupOnDaemonExitO3B(ctx context.Context, ledgerStore *LedgerStoreO, executor NDMCExecutorM, hostGate func() error) (LifetimeResultO3B, error) {
	out := LifetimeResultO3B{State: "precheck", Zones: []string{}, LedgerRetained: true}
	if ledgerStore == nil || executor == nil {
		return out, errors.New("lifetime cleanup unavailable")
	}
	if hostGate == nil {
		return out, errors.New("host gate absent")
	}
	if err := hostGate(); err != nil {
		return out, err
	}
	unlock, err := lockNDMCForO2()
	if err != nil {
		return out, err
	}
	defer unlock()
	ledger, _, err := ledgerStore.Load()
	if err != nil {
		return out, err
	}
	if ledger == nil {
		out.State = "no_ownership"
		return out, nil
	}
	if ledger.Baseline == "" {
		return out, errors.New("ledger missing exact baseline")
	}
	if ledger.State != "active" && ledger.State != "pending" && ledger.State != "deactivating" {
		return out, errors.New("unsupported ledger state")
	}
	snapshot, err := trialSnapshotM(ctx, executor)
	if err != nil {
		return out, err
	}
	present, _, err := classifyStartupO3(snapshot, *ledger)
	if err != nil {
		return out, err
	}
	out.Zones = present
	if len(present) == 0 {
		if snapshot != ledger.Baseline {
			return out, errors.New("baseline drift with no owned zones")
		}
		out.State = "baseline_already_restored"
		out.ExactBaseline = true
		return out, nil
	}
	if err := rollbackManagedN(ctx, executor, ledger.Baseline, ledger.Listen, zonesFromLedgerO2(*ledger)); err != nil {
		return out, err
	}
	out.State = "dead_listener_zones_removed"
	out.ExactBaseline = true
	return out, nil
}

func CleanupOnDaemonExitO3B(ctx context.Context, store *LedgerStoreO, executor NDMCExecutorM) (LifetimeResultO3B, error) {
	return cleanupOnDaemonExitO3B(ctx, store, executor, NDMCTestHostGateM)
}

func archiveShortGuardsO3B(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	destination, err := os.MkdirTemp("/tmp", "rf-dpr2o3-closed-short-")
	if err != nil {
		return err
	}
	for _, from := range paths {
		to := filepath.Join(destination, filepath.Base(from))
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	return nil
}

// Existing active lifetime journals block a new daemon until cleanup is done.
func AwaitPreviousLifetimeO3B(ctx context.Context) error {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		paths, err := filepath.Glob(lifetimePrefixO3B + "*")
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return nil
		}
		for _, p := range paths {
			b, err := os.ReadFile(filepath.Join(p, "result.txt"))
			if err == nil {
				return fmt.Errorf("previous lifetime guard not archived: %s: %s", p, strings.TrimSpace(string(b)))
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("previous lifetime guard still running: %w", ctx.Err())
		case <-tick.C:
		}
	}
}

func ArmLifetimeO3B(ctx context.Context, ledger *LedgerStoreO, life *LifecycleManager) (string, error) {
	if err := NDMCTestHostGateM(); err != nil {
		return "", err
	}
	if ledger == nil || life == nil {
		return "", errors.New("missing ownership ledger or lifecycle")
	}
	status := life.Status()
	if !status.Active || status.Listen != "192.168.10.1:55358" {
		return "", errors.New("lifetime guard needs exact active DSTest LAN listener")
	}
	start, err := procStartO3B(os.Getpid())
	if err != nil {
		return "", err
	}
	j := lifetimeJournalO3B{Schema: 1, Parent: os.Getpid(), ParentStart: start, LedgerPath: ledger.path, Listen: status.Listen, Started: time.Now().Unix()}
	if err := validLifetimeJournalO3B(j); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("/tmp", "rf-dpr2o3-lifetime-")
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(j)
	if err != nil {
		return "", err
	}
	if err := safety.WriteFileAtomic(filepath.Join(dir, "journal.json"), raw, 0600); err != nil {
		return "", err
	}
	cmd, err := safety.CommandContext(context.Background(), "/proc/self/exe", "--ndmc-lifetime-worker", dir)
	if err != nil {
		return "", err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil {
			return dir, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return "", fmt.Errorf("lifetime guard not ready: %s", dir)
}

func LifetimeWorkerO3B(dir string) error {
	if err := NDMCTestHostGateM(); err != nil {
		return err
	}
	st, err := os.Lstat(dir)
	if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || !strings.HasPrefix(dir, lifetimePrefixO3B) {
		return errors.New("invalid lifetime journal directory")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "journal.json"))
	if err != nil {
		return err
	}
	var j lifetimeJournalO3B
	if err := json.Unmarshal(raw, &j); err != nil {
		return err
	}
	if err := validLifetimeJournalO3B(j); err != nil {
		return err
	}
	if err := safety.WriteFileAtomic(filepath.Join(dir, "ready"), []byte("armed\n"), 0600); err != nil {
		return err
	}
	failures := 0
	for isParentAliveO3B(j) {
		conn, err := net.DialTimeout("tcp", j.Listen, 600*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			failures = 0
		} else {
			failures++
		}
		if failures >= 3 {
			break
		} // listener failed, even if management PID lives
		time.Sleep(time.Second)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 165*time.Second)
	defer cancel()
	short, err := awaitShortGuardsO3B(ctx)
	if err == nil {
		cleanupCtx, stop := context.WithTimeout(ctx, 35*time.Second)
		_, err = CleanupOnDaemonExitO3B(cleanupCtx, NewLedgerStoreO(j.LedgerPath), NewRealNDMCM())
		stop()
	}
	if err == nil {
		err = archiveShortGuardsO3B(short)
	}
	result := "LIFETIME_O3_CLEANUP_PASS\n"
	if err != nil {
		result = "LIFETIME_O3_CLEANUP_FAIL: " + err.Error() + "\n"
	}
	_ = safety.WriteFileAtomic(filepath.Join(dir, "result.txt"), []byte(result), 0600)
	if err != nil {
		return err
	}
	completed := strings.Replace(dir, "/rf-dpr2o3-lifetime-", "/rf-dpr2o3-completed-", 1)
	return os.Rename(dir, completed)
}
