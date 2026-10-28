package dnsrouter

// DPR2-N: opt-in, bounded and automatically reverted multi-zone *trial*.
// Never saves the Keenetic running configuration. It is not persistent apply.
// The native ndnproxy DNS service, firewall and NFQUEUE are never restarted.

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const managedMaxZonesN = 8

var ErrManagedTokenN = errors.New("stale or missing managed plan confirmation token")

type ManagedPrepareN struct {
	Revision          string             `json:"revision"`
	Token             string             `json:"token,omitempty"`
	Listen            string             `json:"listen,omitempty"`
	Zones             []DomainSplitZoneL `json:"zones"`
	Conflicts         []string           `json:"conflicts"`
	Ready             bool               `json:"ready"`
	ChangesApplied    bool               `json:"changes_applied"`
	AutomaticRollback bool               `json:"automatic_rollback"`
	Mode              string             `json:"mode"`
}

type managedJournalN struct {
	Parent   int      `json:"parent"`
	Deadline int64    `json:"deadline"`
	Baseline string   `json:"baseline"`
	Listen   string   `json:"listen"`
	Domains  []string `json:"domains"`
}

type ManagedResultN struct {
	State            string   `json:"state"`
	Revision         string   `json:"revision"`
	Zones            []string `json:"zones"`
	Registered       int      `json:"registered"`
	Verified         int      `json:"verified"`
	BaselineRestored bool     `json:"baseline_restored"`
	WatchdogArmed    bool     `json:"watchdog_armed"`
	DurableChange    bool     `json:"durable_change"`
}

type managedGuardN interface{ Finish() error }
type managedArmN func(context.Context, managedJournalN) (managedGuardN, error)
type managedProbeN func(context.Context, []string) error

type ManagedTrialN struct {
	Store     *ConfigStore
	Lifecycle *LifecycleManager
	Executor  NDMCExecutorM
	Arm       managedArmN
	Probe     managedProbeN
	// Unit tests inject readiness but production requires the active exact CAS.
	Ready func(string, string) bool
}

func managedZonesN(cfg Config) (string, []DomainSplitZoneL, error) {
	p := BuildDomainSplitPlanL(cfg)
	if !p.Ready || len(p.Zones) == 0 || len(p.Zones) > managedMaxZonesN {
		return "", nil, errors.New("managed trial requires 1..8 valid non-overlapping suffix zones and safe LAN listener")
	}
	if err := ValidateListenerConfigL(cfg.Listener); err != nil {
		return "", nil, err
	}
	if cfg.Listener == nil || cfg.Listener.Mode != "lan" || p.Listener != cfg.Listener.Listen {
		return "", nil, errors.New("managed trial requires a LAN listener")
	}
	// Require one-to-one coverage: exact/subdomains-only rules need separate
	// semantics, so they are never silently activated by domain-wide split.
	for _, r := range cfg.Rules {
		if r.Match != "suffix" {
			return "", nil, errors.New("managed trial does not support exact/subdomains-only rules")
		}
	}
	// No listener-to-upstream loop, including any explicit fallback and any
	// primary upstream whose port is the manager's own LAN listener.
	for _, u := range cfg.Upstreams {
		if u.Address == p.Listener {
			return "", nil, fmt.Errorf("upstream %q loops to managed listener", u.ID)
		}
	}
	for _, z := range p.Zones {
		d, e := normalizedDomain(z.Domain)
		if e != nil || d != z.Domain || isProtected(d) || !strings.Contains(d, ".") || len(d) > 200 {
			return "", nil, errors.New("unsafe managed DNS zone")
		}
	}
	return p.Listener, p.Zones, nil
}

func snapshotDomainsN(snapshot string) ([]struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	Domain  string `json:"domain"`
}, error) {
	var doc struct {
		Servers []struct {
			Address string `json:"address"`
			Port    int    `json:"port"`
			Domain  string `json:"domain"`
		} `json:"server"`
	}
	if e := json.Unmarshal([]byte(snapshot), &doc); e != nil {
		return nil, e
	}
	if doc.Servers == nil {
		return nil, errors.New("missing ndmc server list")
	}
	return doc.Servers, nil
}

func conflictingZonesN(snapshot string, zones []DomainSplitZoneL) ([]string, error) {
	servers, e := snapshotDomainsN(snapshot)
	if e != nil {
		return nil, e
	}
	conflicts := []string{}
	for _, z := range zones {
		for _, s := range servers {
			d := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s.Domain), "."))
			if d == "" {
				continue
			} // global name-server remains untouched
			if d == z.Domain || strings.HasSuffix(d, "."+z.Domain) || strings.HasSuffix(z.Domain, "."+d) {
				conflicts = append(conflicts, fmt.Sprintf("%s overlaps existing %s (%s:%d)", z.Domain, d, s.Address, s.Port))
			}
		}
	}
	return conflicts, nil
}

func containsAllManagedN(snapshot string, zones []DomainSplitZoneL, listener string) (bool, error) {
	servers, e := snapshotDomainsN(snapshot)
	if e != nil {
		return false, e
	}
	host, portS, e := net.SplitHostPort(listener)
	if e != nil {
		return false, e
	}
	var port int
	_, e = fmt.Sscanf(portS, "%d", &port)
	if e != nil {
		return false, e
	}
	for _, z := range zones {
		found := 0
		for _, s := range servers {
			if s.Address == host && s.Port == port && s.Domain == z.Domain {
				found++
			}
		}
		if found != 1 {
			return false, nil
		}
	}
	return true, nil
}

// rollbackManagedN removes only registrations that are actually present for the
// exact listener, then checks the complete baseline. It never removes an
// existing entry owned by another server or a zone that was not added.
func rollbackManagedN(ctx context.Context, executor NDMCExecutorM, baseline, listener string, zones []DomainSplitZoneL) error {
	current, err := trialSnapshotM(ctx, executor)
	if err != nil {
		return err
	}
	servers, err := snapshotDomainsN(current)
	if err != nil {
		return err
	}
	host, portText, err := net.SplitHostPort(listener)
	if err != nil {
		return err
	}
	var port int
	if _, err = fmt.Sscanf(portText, "%d", &port); err != nil {
		return err
	}
	var failures []string
	for i := len(zones) - 1; i >= 0; i-- {
		z := zones[i]
		exact := 0
		for _, server := range servers {
			if server.Domain != z.Domain {
				continue
			}
			if server.Address != host || server.Port != port {
				return fmt.Errorf("DNS zone %s changed owner during rollback", z.Domain)
			}
			exact++
		}
		if exact > 1 {
			return fmt.Errorf("DNS zone %s unexpectedly duplicated during rollback", z.Domain)
		}
		if exact == 1 {
			if _, err = executor.Command(ctx, z.Remove); err != nil {
				failures = append(failures, z.Domain+": "+err.Error())
			}
		}
	}
	after, err := trialSnapshotM(ctx, executor)
	if err != nil {
		failures = append(failures, err.Error())
	} else if after != baseline {
		failures = append(failures, "DNS inventory not exactly restored")
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func (m *ManagedTrialN) preflight(ctx context.Context, expected string) (ManagedPrepareN, string, error) {
	result := ManagedPrepareN{Revision: expected, Zones: []DomainSplitZoneL{}, Conflicts: []string{}, Mode: "temporary-trial-only", AutomaticRollback: true}
	if m == nil || m.Store == nil || m.Executor == nil || m.Arm == nil || m.Probe == nil {
		return result, "", errors.New("managed trial capability unavailable")
	}
	cfg, rev, e := m.Store.Load()
	if e != nil {
		return result, "", e
	}
	result.Revision = rev
	if expected == "" || expected != rev {
		return result, "", ErrStale
	}
	listener, zones, e := managedZonesN(cfg)
	if e != nil {
		return result, "", e
	}
	result.Listen = listener
	result.Zones = zones
	ready := false
	if m.Ready != nil {
		ready = m.Ready(rev, listener)
	} else if m.Lifecycle != nil {
		s := m.Lifecycle.Status()
		ready = s.Active && s.AppliedRevision == rev && s.Listen == listener
	}
	if !ready {
		return result, "", errors.New("exact saved revision must be running on explicit LAN listener")
	}
	base, e := trialSnapshotM(ctx, m.Executor)
	if e != nil {
		return result, "", e
	}
	conflicts, e := conflictingZonesN(base, zones)
	if e != nil {
		return result, "", e
	}
	result.Conflicts = conflicts
	if len(conflicts) > 0 {
		return result, base, nil
	}
	// Token binds the exact revision, list/order of zone commands and native
	// inventory. Any changed baseline invalidates the user's confirmation.
	raw, e := json.Marshal(struct {
		Revision string
		Baseline string
		Listen   string
		Zones    []DomainSplitZoneL
	}{rev, base, listener, zones})
	if e != nil {
		return result, "", e
	}
	sum := sha256.Sum256(raw)
	result.Token = "sha256:" + hex.EncodeToString(sum[:])
	result.Ready = true
	return result, base, nil
}

func (m *ManagedTrialN) Prepare(ctx context.Context, expected string) (ManagedPrepareN, error) {
	r, _, e := m.preflight(ctx, expected)
	return r, e
}

func (m *ManagedTrialN) Run(ctx context.Context, expected, token string) (out ManagedResultN, retErr error) {
	out = ManagedResultN{State: "precheck", DurableChange: false, Zones: []string{}}
	trialGlobalMuM.Lock()
	defer trialGlobalMuM.Unlock()
	fd, e := syscall.Open("/tmp/routerforge-dpr2m-trial.lock", syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return out, e
	}
	defer syscall.Close(fd)
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return out, fmt.Errorf("another DNS transaction is running: %w", e)
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	plan, base, e := m.preflight(ctx, expected)
	if e != nil {
		return out, e
	}
	out.Revision = plan.Revision
	if !plan.Ready {
		return out, fmt.Errorf("managed zone conflict: %v", plan.Conflicts)
	}
	if token == "" || token != plan.Token {
		return out, ErrManagedTokenN
	}
	for _, z := range plan.Zones {
		out.Zones = append(out.Zones, z.Domain)
	}
	guard, e := m.Arm(ctx, managedJournalN{Parent: os.Getpid(), Deadline: time.Now().Add(115 * time.Second).Unix(), Baseline: base, Listen: plan.Listen, Domains: out.Zones})
	if e != nil {
		return out, fmt.Errorf("independent rollback watchdog unavailable: %w", e)
	}
	out.WatchdogArmed = true
	// Keep normal work decisively shorter than the 115-second independent
	// watchdog lease. Even a failed eight-zone attempt needs rollback time.
	workCtx, cancelWork := context.WithTimeout(ctx, 65*time.Second)
	defer cancelWork()
	// Full cleanup is required even for partial/ambiguous additions. The guard
	// remains active whenever exact baseline restoration is not established.
	defer func() {
		rollback, stop := context.WithTimeout(context.Background(), 35*time.Second)
		defer stop()
		var failures []string
		if err := rollbackManagedN(rollback, m.Executor, base, plan.Listen, plan.Zones); err != nil {
			failures = append(failures, err.Error())
		} else {
			out.BaselineRestored = true
			if err := guard.Finish(); err != nil {
				failures = append(failures, "watchdog acknowledgement: "+err.Error())
			}
		}
		if len(failures) > 0 {
			out.State = "ambiguous"
			if retErr != nil {
				retErr = fmt.Errorf("%v; rollback: %s", retErr, strings.Join(failures, "; "))
			} else {
				retErr = errors.New("rollback: " + strings.Join(failures, "; "))
			}
			return
		}
		if retErr != nil {
			out.State = "rolled_back_after_failure"
		} else {
			out.State = "pass"
		}
	}()
	for _, z := range plan.Zones {
		if _, e = m.Executor.Command(workCtx, z.Add); e != nil {
			return out, e
		}
		out.Registered++
	}
	// Registration must be observed in the runtime inventory before DNS probes.
	added := false
	for i := 0; i < 6; i++ {
		s, se := trialSnapshotM(workCtx, m.Executor)
		if se == nil {
			added, se = containsAllManagedN(s, plan.Zones, plan.Listen)
		}
		if se == nil && added {
			break
		}
		select {
		case <-workCtx.Done():
			return out, workCtx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	if !added {
		return out, errors.New("ndmc did not register every managed zone")
	}
	if e = m.Probe(workCtx, out.Zones); e != nil {
		return out, fmt.Errorf("managed DNS forwarding verification failed: %w", e)
	}
	out.Verified = len(out.Zones)
	return out, nil
}

// ProbeManagedN sends fresh UDP and TCP queries through ndnproxy, ensuring the
// active managed forwarder sees them. NXDOMAIN is valid for a user-supplied zone;
// a forwarded request, not a particular public IP, is the proof of routing.
func ProbeManagedN(l *LifecycleManager) managedProbeN {
	return func(ctx context.Context, domains []string) error {
		if l == nil {
			return errors.New("lifecycle unavailable")
		}
		f := l.Forwarder()
		if f == nil || !f.Active() {
			return errors.New("managed DNS forwarder inactive")
		}
		for n, domain := range domains {
			for j, peer := range []string{"127.0.0.1:53", "192.168.10.1:53"} {
				for k, proto := range []string{"udp", "tcp"} {
					before := f.Stats().Requests
					success := false
					var last error
					for a := 0; a < 10; a++ {
						name := fmt.Sprintf("n%d-%d-%d-%d.%s", n, j, k, a, domain)
						id := uint16(0x7300 + n*128 + j*32 + k*16 + a)
						last = queryManagedDNSN(ctx, proto, peer, name, id)
						if last == nil && f.Stats().Requests > before {
							success = true
							break
						}
						select {
						case <-ctx.Done():
							return ctx.Err()
						case <-time.After(350 * time.Millisecond):
						}
					}
					if !success {
						return fmt.Errorf("%s %s %s: forwarding not verified: %v", proto, peer, domain, last)
					}
				}
			}
		}
		return nil
	}
}

func queryManagedDNSN(ctx context.Context, proto, dest, name string, id uint16) error {
	q := []byte{byte(id >> 8), byte(id), 1, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	for _, label := range strings.Split(name, ".") {
		if len(label) < 1 || len(label) > 63 {
			return errors.New("invalid probe label")
		}
		q = append(q, byte(len(label)))
		q = append(q, label...)
	}
	q = append(q, 0, 0, 1, 0, 1)
	dial := net.Dialer{Timeout: 900 * time.Millisecond}
	c, e := dial.DialContext(ctx, proto, dest)
	if e != nil {
		return e
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(1100 * time.Millisecond))
	data := q
	if proto == "tcp" {
		data = append([]byte{byte(len(q) >> 8), byte(len(q))}, q...)
	}
	if _, e = c.Write(data); e != nil {
		return e
	}
	var r []byte
	if proto == "tcp" {
		h := make([]byte, 2)
		if _, e = io.ReadFull(c, h); e != nil {
			return e
		}
		sz := int(binary.BigEndian.Uint16(h))
		if sz < 12 || sz > 4096 {
			return errors.New("invalid TCP reply size")
		}
		r = make([]byte, sz)
		_, e = io.ReadFull(c, r)
	} else {
		buf := make([]byte, 4096)
		var n int
		n, e = c.Read(buf)
		r = buf[:n]
	}
	if e != nil {
		return e
	}
	if !sameDNSQuestion(q, r) {
		return errors.New("DNS response/question mismatch")
	}
	rcode := r[3] & 15
	if rcode != 0 && rcode != 3 {
		return fmt.Errorf("DNS upstream returned RCODE %d", rcode)
	}
	return nil
}

type osGuardN struct {
	dir string
	cmd *exec.Cmd
}

func (g *osGuardN) Finish() error {
	if e := os.WriteFile(filepath.Join(g.dir, "complete"), []byte("exact rollback verified\n"), 0600); e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() { done <- g.cmd.Wait() }()
	select {
	case e := <-done:
		if e != nil {
			return e
		}
		return os.RemoveAll(g.dir)
	case <-time.After(5 * time.Second):
		return errors.New("managed watchdog did not acknowledge completion")
	}
}
func ArmOSGuardN(ctx context.Context, j managedJournalN) (managedGuardN, error) {
	dir, e := os.MkdirTemp("/tmp", "rf-dpr2n-guard-")
	if e != nil {
		return nil, e
	}
	if e = os.Chmod(dir, 0700); e != nil {
		_ = os.RemoveAll(dir)
		return nil, e
	}
	raw, e := json.Marshal(j)
	if e != nil {
		_ = os.RemoveAll(dir)
		return nil, e
	}
	if e = os.WriteFile(filepath.Join(dir, "journal.json"), raw, 0600); e != nil {
		_ = os.RemoveAll(dir)
		return nil, e
	}
	command := exec.Command("/proc/self/exe", "--ndmc-managed-rollback-worker", dir)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if e = command.Start(); e != nil {
		_ = os.RemoveAll(dir)
		return nil, e
	}
	for i := 0; i < 50; i++ {
		if _, e = os.Stat(filepath.Join(dir, "ready")); e == nil {
			return &osGuardN{dir, command}, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil, errors.New("managed watchdog did not arm; journal retained")
}
func ManagedWorkerN(dir string) error {
	if e := NDMCTestHostGateM(); e != nil {
		return e
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid managed journal directory")
	}
	raw, e := os.ReadFile(filepath.Join(dir, "journal.json"))
	if e != nil {
		return e
	}
	var j managedJournalN
	if e = json.Unmarshal(raw, &j); e != nil {
		return e
	}
	if j.Parent < 1 || j.Deadline <= time.Now().Unix() || j.Deadline > time.Now().Add(120*time.Second).Unix() {
		return errors.New("invalid managed lease")
	}
	if len(j.Domains) < 1 || len(j.Domains) > managedMaxZonesN {
		return errors.New("invalid managed zone count")
	}
	host, port, e := net.SplitHostPort(j.Listen)
	if e != nil {
		return e
	}
	if host != "192.168.10.1" || port == "53" || port == "" {
		return errors.New("managed worker only permits exact DSTest LAN")
	}
	for _, d := range j.Domains {
		n, e := normalizedDomain(d)
		if e != nil || n != d || isProtected(d) || !strings.Contains(d, ".") {
			return errors.New("invalid managed worker domain")
		}
	}
	if _, e = canonicalNDMCSnapshotM(j.Baseline); e != nil {
		return e
	}
	zones := make([]DomainSplitZoneL, 0, len(j.Domains))
	for _, d := range j.Domains {
		zones = append(zones, DomainSplitZoneL{Domain: d, Remove: "no ip name-server " + j.Listen + " " + d})
	}
	if conflicts, e := conflictingZonesN(j.Baseline, zones); e != nil || len(conflicts) > 0 {
		return errors.New("managed baseline contains conflicting zones")
	}
	if e = os.WriteFile(filepath.Join(dir, "ready"), []byte("armed\n"), 0600); e != nil {
		return e
	}
	for {
		if _, e = os.Stat(filepath.Join(dir, "complete")); e == nil {
			return nil
		}
		if e = syscall.Kill(j.Parent, 0); e != nil || time.Now().Unix() >= j.Deadline {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	errorsFound := []string{}
	if err := rollbackManagedN(ctx, realNDMCM{}, j.Baseline, j.Listen, zones); err != nil {
		errorsFound = append(errorsFound, err.Error())
	}
	result := "WATCHDOG_N_ROLLBACK_PASS\n"
	if len(errorsFound) > 0 {
		result = "WATCHDOG_N_ROLLBACK_FAIL: " + strings.Join(errorsFound, "; ") + "\n"
	}
	_ = os.WriteFile(filepath.Join(dir, "result.txt"), []byte(result), 0600)
	if len(errorsFound) > 0 {
		return errors.New(strings.Join(errorsFound, "; "))
	}
	return nil
}
