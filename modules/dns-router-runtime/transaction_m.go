package dnsrouter

// DPR2-M: one-shot, bounded DSTest-only ndmc domain transaction. It never calls
// "system configuration save", changes the global resolver, or binds port 53.
// The independent watchdog reverts the exact test entry after a parent crash.

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	mZone         = "dpr2m.test"
	mListener     = "192.168.10.1:55358"
	mUpstream     = "192.168.10.1:55354"
	mProbe        = "probe.dpr2m.test"
	mExpectedIPv4 = "203.0.113.42"
)

type NDMCExecutorM interface {
	Command(context.Context, string) (string, error)
}
type realNDMCM struct{}

func (realNDMCM) Command(parent context.Context, command string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ndmc", "-c", command).CombinedOutput()
	if len(out) > 65536 {
		return "", errors.New("ndmc output too large")
	}
	if err != nil {
		return "", fmt.Errorf("ndmc command failed: %w", err)
	}
	// Keenetic can encode a command error in output while exiting zero.
	if strings.Contains(strings.ToLower(string(out)), "error:") {
		return "", errors.New("ndmc returned error")
	}
	return string(out), nil
}

type NDMCProbeM interface{ Check(context.Context) error }
type realProbeM struct{}

func (realProbeM) Check(ctx context.Context) error {
	for _, dest := range []string{"127.0.0.1:53", "192.168.10.1:53"} {
		for _, network := range []string{"udp", "tcp"} {
			if err := probeDNSM(ctx, network, dest); err != nil {
				return fmt.Errorf("%s %s: %w", network, dest, err)
			}
		}
	}
	return nil
}
func probeDNSM(ctx context.Context, network, dest string) error {
	q := []byte{0x6d, 0x31, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	for _, label := range strings.Split(mProbe, ".") {
		q = append(q, byte(len(label)))
		q = append(q, []byte(label)...)
	}
	q = append(q, 0, 0, 1, 0, 1)
	dial := net.Dialer{Timeout: 2 * time.Second}
	c, err := dial.DialContext(ctx, network, dest)
	if err != nil {
		return err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	if network == "tcp" {
		h := make([]byte, 2)
		binary.BigEndian.PutUint16(h, uint16(len(q)))
		q = append(h, q...)
	}
	if _, err = c.Write(q); err != nil {
		return err
	}
	var r []byte
	if network == "tcp" {
		h := make([]byte, 2)
		if _, err = io.ReadFull(c, h); err != nil {
			return err
		}
		n := int(binary.BigEndian.Uint16(h))
		if n < 12 || n > 4096 {
			return errors.New("invalid DNS reply size")
		}
		r = make([]byte, n)
		_, err = io.ReadFull(c, r)
	} else {
		r = make([]byte, 4096)
		var n int
		n, err = c.Read(r)
		r = r[:n]
	}
	if err != nil {
		return err
	}
	if len(r) < 16 || r[0] != 0x6d || r[1] != 0x31 || r[2]&0x80 == 0 || r[3]&0x0f != 0 {
		return errors.New("DNS reply identity/status mismatch")
	}
	want := net.ParseIP(mExpectedIPv4).To4()
	if string(r[len(r)-4:]) != string(want) {
		return errors.New("synthetic A record missing")
	}
	return nil
}

type trialJournalM struct {
	Parent   int    `json:"parent"`
	Baseline string `json:"baseline"`
	Deadline int64  `json:"deadline"`
}
type trialGuardM interface{ Finish() error }
type armGuardM func(context.Context, string) (trialGuardM, error)

var trialGlobalMuM sync.Mutex

type NDMCTrialM struct {
	Store     *ConfigStore
	Lifecycle *LifecycleManager
	Executor  NDMCExecutorM
	Probe     NDMCProbeM
	Arm       armGuardM
	Ready     func(string) bool // injected in tests only, runtime uses Lifecycle
}
type TrialResultM struct {
	State             string `json:"state"`
	Revision          string `json:"revision"`
	Zone              string `json:"zone"`
	AddVerified       bool   `json:"add_verified"`
	UDPAndTCPVerified bool   `json:"udp_tcp_verified"`
	BaselineRestored  bool   `json:"baseline_restored"`
	WatchdogArmed     bool   `json:"watchdog_armed"`
	DurableChange     bool   `json:"durable_change"`
}

func exactTrialPlanM(cfg Config) error {
	p := BuildDomainSplitPlanL(cfg)
	if !p.Ready || len(p.Zones) != 1 || p.Zones[0].Domain != mZone || p.Listener != mListener {
		return errors.New("DPR2-M is limited to one DSTest dpr2m.test zone/listener")
	}
	if len(cfg.Upstreams) != 1 || cfg.Upstreams[0].Address != mUpstream {
		return errors.New("DPR2-M requires exact private fixture upstream")
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].Match != "suffix" || cfg.Rules[0].Domain != mZone {
		return errors.New("DPR2-M requires exact suffix lab rule")
	}
	return nil
}
func trialCommandsM() (add, remove string) {
	return "ip name-server " + mListener + " " + mZone, "no ip name-server " + mListener + " " + mZone
}
func trialSnapshotM(ctx context.Context, d NDMCExecutorM) (string, error) {
	return d.Command(ctx, "show ip name-server")
}
func absentFromSnapshotM(raw string) (bool, error) {
	var doc struct {
		Servers []struct {
			Address string `json:"address"`
			Port    int    `json:"port"`
			Domain  string `json:"domain"`
		} `json:"server"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return false, fmt.Errorf("ndmc inventory is not JSON: %w", err)
	}
	if doc.Servers == nil {
		return false, errors.New("ndmc inventory missing server array")
	}
	for _, s := range doc.Servers {
		if s.Domain == mZone {
			return false, nil
		}
	}
	return true, nil
}
func presentInSnapshotM(raw string) (bool, error) {
	var doc struct {
		Servers []struct {
			Address string `json:"address"`
			Port    int    `json:"port"`
			Domain  string `json:"domain"`
		} `json:"server"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return false, err
	}
	if doc.Servers == nil {
		return false, errors.New("ndmc inventory missing server array")
	}
	for _, s := range doc.Servers {
		if s.Domain == mZone && s.Address == "192.168.10.1" && s.Port == 55358 {
			return true, nil
		}
	}
	return false, nil
}

func (t *NDMCTrialM) Run(ctx context.Context, expected string) (result TrialResultM, retErr error) {
	result = TrialResultM{State: "precheck", Zone: mZone}
	trialGlobalMuM.Lock()
	defer trialGlobalMuM.Unlock()
	fd, lockErr := syscall.Open("/tmp/routerforge-dpr2m-trial.lock", syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if lockErr != nil {
		return result, lockErr
	}
	defer syscall.Close(fd)
	if lockErr = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); lockErr != nil {
		return result, fmt.Errorf("another trial is already running: %w", lockErr)
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	if t == nil || t.Store == nil || t.Executor == nil || t.Probe == nil || t.Arm == nil {
		return result, errors.New("DPR2-M transaction dependencies missing")
	}
	cfg, rev, err := t.Store.Load()
	if err != nil {
		return result, err
	}
	result.Revision = rev
	if expected == "" || expected != rev {
		return result, ErrStale
	}
	if err = exactTrialPlanM(cfg); err != nil {
		return result, err
	}
	ready := false
	if t.Ready != nil {
		ready = t.Ready(rev)
	} else if t.Lifecycle != nil {
		active := t.Lifecycle.Status()
		ready = active.Active && active.AppliedRevision == rev && active.Listen == mListener
	}
	if !ready {
		return result, errors.New("exact saved revision must already be active on DSTest LAN listener")
	}
	before, err := trialSnapshotM(ctx, t.Executor)
	if err != nil {
		return result, err
	}
	absent, err := absentFromSnapshotM(before)
	if err != nil {
		return result, err
	}
	if !absent {
		return result, errors.New("test zone already exists, refusing modification")
	}
	guard, err := t.Arm(ctx, before)
	if err != nil {
		return result, fmt.Errorf("independent rollback guard unavailable: %w", err)
	}
	result.WatchdogArmed = true
	add, remove := trialCommandsM()
	// Set the mutation flag before issuing add: command may partially succeed.
	attempted := true
	defer func() {
		if !attempted {
			return
		}
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		var rollbackErr error
		if _, err := t.Executor.Command(rollbackCtx, remove); err != nil {
			rollbackErr = err
		}
		after, err := trialSnapshotM(rollbackCtx, t.Executor)
		if err != nil {
			rollbackErr = err
		} else if after != before {
			rollbackErr = errors.New("DNS inventory not exactly restored")
		}
		if rollbackErr == nil {
			result.BaselineRestored = true
			if err := guard.Finish(); err != nil {
				rollbackErr = fmt.Errorf("rollback watchdog did not acknowledge completion: %w", err)
			}
		}
		if rollbackErr != nil {
			result.State = "ambiguous"
			if retErr != nil {
				retErr = fmt.Errorf("%v; rollback: %w", retErr, rollbackErr)
			} else {
				retErr = fmt.Errorf("rollback: %w", rollbackErr)
			}
			return
		}
		if retErr != nil {
			result.State = "rolled_back_after_failure"
		} else {
			result.State = "pass"
		}
	}()
	if _, err = t.Executor.Command(ctx, add); err != nil {
		return result, err
	}
	for i := 0; i < 4; i++ {
		state, e := trialSnapshotM(ctx, t.Executor)
		if e == nil {
			yes, e2 := presentInSnapshotM(state)
			if e2 == nil && yes {
				result.AddVerified = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !result.AddVerified {
		return result, errors.New("new test zone not verified")
	}
	if err := t.Probe.Check(ctx); err != nil {
		return result, fmt.Errorf("ndnproxy hardware DNS verification: %w", err)
	}
	result.UDPAndTCPVerified = true
	return result, nil
}

type osGuardM struct {
	dir     string
	command *exec.Cmd
}

func (g *osGuardM) Finish() error {
	if err := os.WriteFile(filepath.Join(g.dir, "complete"), []byte("rollback-verified\n"), 0600); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- g.command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return err
		}
		return os.RemoveAll(g.dir)
	case <-time.After(5 * time.Second):
		return errors.New("rollback watchdog did not terminate")
	}
}
func ArmOSGuardM(ctx context.Context, baseline string) (trialGuardM, error) {
	dir, err := os.MkdirTemp("/tmp", "rf-dpr2m-guard-")
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	journal := trialJournalM{Parent: os.Getpid(), Baseline: baseline, Deadline: time.Now().Add(115 * time.Second).Unix()}
	raw, err := json.Marshal(journal)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(dir, "journal.json"), raw, 0600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	command := exec.Command("/proc/self/exe", "--ndmc-rollback-worker", dir)
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err = command.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	for i := 0; i < 50; i++ {
		if _, err = os.Stat(filepath.Join(dir, "ready")); err == nil {
			return &osGuardM{dir: dir, command: command}, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil, errors.New("watchdog did not arm (journal preserved for audit)")
}

// NDMCWatchdogWorkerM is invoked ONLY by the separate --ndmc-rollback-worker
// command; the journal has prevalidated exact test-only targets.
func NDMCWatchdogWorkerM(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid journal dir")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "journal.json"))
	if err != nil {
		return err
	}
	var j trialJournalM
	if err = json.Unmarshal(raw, &j); err != nil {
		return err
	}
	if j.Parent < 1 || j.Deadline <= time.Now().Unix() || j.Deadline > time.Now().Add(120*time.Second).Unix() {
		return errors.New("bad rollback lease")
	}
	if absent, err := absentFromSnapshotM(j.Baseline); err != nil {
		return err
	} else if !absent {
		return errors.New("watchdog journal baseline contains test zone")
	}
	if err = os.WriteFile(filepath.Join(dir, "ready"), []byte("armed\n"), 0600); err != nil {
		return err
	}
	for {
		if _, err = os.Stat(filepath.Join(dir, "complete")); err == nil {
			return nil
		}
		if err := syscall.Kill(j.Parent, 0); err != nil || time.Now().Unix() >= j.Deadline {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, rm := trialCommandsM()
	_, err = realNDMCM{}.Command(ctx, rm)
	if err == nil {
		var state string
		state, err = trialSnapshotM(ctx, realNDMCM{})
		if err == nil && state != j.Baseline {
			err = errors.New("watchdog restore snapshot drift")
		}
	}
	msg := "WATCHDOG_ROLLBACK_PASS\n"
	if err != nil {
		msg = "WATCHDOG_ROLLBACK_FAILED: " + err.Error() + "\n"
	}
	_ = os.WriteFile(filepath.Join(dir, "result.txt"), []byte(msg), 0600)
	return err
}

func NewRealNDMCM() NDMCExecutorM { return realNDMCM{} }
func NewRealProbeM() NDMCProbeM   { return realProbeM{} }

// Harden the opt-in execution to the verified DSTest target only.
func NDMCTestHostGateM() error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	if hostname != "DSTest" {
		return errors.New("ndmc trial requires DSTest hostname")
	}
	ifaces, err := net.InterfaceAddrs()
	if err != nil {
		return err
	}
	for _, a := range ifaces {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.String() == "192.168.10.1" {
			return nil
		}
	}
	return errors.New("ndmc trial requires DSTest IPv4 192.168.10.1")
}
