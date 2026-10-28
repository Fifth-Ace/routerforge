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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
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
	out, err := safety.RunCommand(ctx, 65537, "ndmc", "-c", command)
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

// realProbeM verifies real answers via the native ndnproxy UDP/TCP paths.
// ndmc can publish a new DNS zone in its inventory before ndnproxy begins
// forwarding the zone. Each bounded retry uses a fresh query name so a
// temporary negative cache entry cannot make subsequent attempts falsely fail.
func (realProbeM) Check(ctx context.Context) error {
	readyCtx, cancel := context.WithTimeout(ctx, 16*time.Second)
	defer cancel()
	for i, dest := range []string{"127.0.0.1:53", "192.168.10.1:53"} {
		for j, network := range []string{"udp", "tcp"} {
			if err := probeNDNReadyM(readyCtx, network, dest, i*2+j); err != nil {
				return fmt.Errorf("%s %s: %w", network, dest, err)
			}
		}
	}
	return nil
}

func probeNDNReadyM(ctx context.Context, network, dest string, ordinal int) error {
	return probeNDNReadyWithQueryM(ctx, network, dest, ordinal, probeDNSM)
}

func probeNDNReadyWithQueryM(ctx context.Context, network, dest string, ordinal int, check func(context.Context, string, string, string, uint16) error) error {
	const maxAttempts = 12
	var last error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("readiness deadline after %d tries: %w; last=%v", attempt, err, last)
		}
		// Domain is fixed to a DNS subtree of the exact one-zone allowlist.
		// Different names avoid a stale negative result in ndnproxy's cache.
		name := fmt.Sprintf("p%d-%d.probe.%s", ordinal, attempt, mZone)
		id := uint16(0x6d30 + ordinal*16 + attempt)
		last = check(ctx, network, dest, name, id)
		if last == nil {
			return nil
		}
		if attempt+1 < maxAttempts {
			select {
			case <-ctx.Done():
				return fmt.Errorf("readiness interrupted after %d tries: %w; last=%v", attempt+1, ctx.Err(), last)
			case <-time.After(350 * time.Millisecond):
			}
		}
	}
	return fmt.Errorf("DNS not ready after %d verified queries: %w", maxAttempts, last)
}

func probeDNSM(ctx context.Context, network, dest, name string, id uint16) error {
	if dest != "127.0.0.1:53" && dest != "192.168.10.1:53" {
		return errors.New("DNS probe destination is outside lab allowlist")
	}
	if network != "udp" && network != "tcp" {
		return errors.New("invalid DNS probe transport")
	}
	if !strings.HasSuffix(name, "."+mZone) || len(name) > 110 {
		return errors.New("DNS probe name is outside lab zone")
	}
	q := []byte{byte(id >> 8), byte(id), 1, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	for _, label := range strings.Split(name, ".") {
		if len(label) < 1 || len(label) > 63 {
			return errors.New("DNS label invalid")
		}
		q = append(q, byte(len(label)))
		q = append(q, []byte(label)...)
	}
	q = append(q, 0, 0, 1, 0, 1)
	dial := net.Dialer{Timeout: 900 * time.Millisecond}
	c, err := dial.DialContext(ctx, network, dest)
	if err != nil {
		return fmt.Errorf("DNS dial: %w", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(1200 * time.Millisecond))
	if network == "tcp" {
		h := make([]byte, 2)
		binary.BigEndian.PutUint16(h, uint16(len(q)))
		q = append(h, q...)
	}
	if _, err = c.Write(q); err != nil {
		return fmt.Errorf("DNS write: %w", err)
	}
	var r []byte
	if network == "tcp" {
		h := make([]byte, 2)
		if _, err = io.ReadFull(c, h); err != nil {
			return fmt.Errorf("DNS TCP header: %w", err)
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
		return fmt.Errorf("DNS read: %w", err)
	}
	if len(r) < 12 {
		return fmt.Errorf("DNS short response: bytes=%d", len(r))
	}
	gotID := binary.BigEndian.Uint16(r[:2])
	flags := binary.BigEndian.Uint16(r[2:4])
	rcode := flags & 15
	answers := binary.BigEndian.Uint16(r[6:8])
	if gotID != id || flags&0x8000 == 0 || rcode != 0 || answers == 0 {
		return fmt.Errorf("DNS response mismatch: id=%04x expected=%04x flags=%04x rcode=%d answers=%d bytes=%d", gotID, id, flags, rcode, answers, len(r))
	}
	// Validate the expected synthetic IPv4 in a real DNS answer rather than
	// relying on trailing bytes; a resolver may append EDNS OPT records.
	at := 12
	for {
		if at >= len(r) {
			return errors.New("DNS question is truncated")
		}
		l := int(r[at])
		at++
		if l == 0 {
			break
		}
		if l > 63 || at+l > len(r) {
			return errors.New("DNS question has invalid label")
		}
		at += l
	}
	if at+4 > len(r) {
		return errors.New("DNS question type/class truncated")
	}
	if binary.BigEndian.Uint16(r[at:at+2]) != 1 || binary.BigEndian.Uint16(r[at+2:at+4]) != 1 {
		return errors.New("DNS question type/class changed")
	}
	at += 4
	want := net.ParseIP(mExpectedIPv4).To4()
	for i := 0; i < int(answers); i++ {
		if at >= len(r) {
			return errors.New("DNS answer truncated")
		}
		if r[at]&0xc0 == 0xc0 {
			if at+2 > len(r) {
				return errors.New("DNS compressed name truncated")
			}
			at += 2
		} else {
			for {
				if at >= len(r) {
					return errors.New("DNS answer name truncated")
				}
				l := int(r[at])
				at++
				if l == 0 {
					break
				}
				if l > 63 || at+l > len(r) {
					return errors.New("DNS invalid answer name")
				}
				at += l
			}
		}
		if at+10 > len(r) {
			return errors.New("DNS answer metadata truncated")
		}
		typ := binary.BigEndian.Uint16(r[at : at+2])
		class := binary.BigEndian.Uint16(r[at+2 : at+4])
		length := int(binary.BigEndian.Uint16(r[at+8 : at+10]))
		at += 10
		if at+length > len(r) {
			return errors.New("DNS answer RDATA truncated")
		}
		if typ == 1 && class == 1 && length == 4 && string(r[at:at+4]) == string(want) {
			return nil
		}
		at += length
	}
	return fmt.Errorf("DNS expected fixture A record absent: rcode=%d answers=%d bytes=%d", rcode, answers, len(r))
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

// ndmc may decorate JSON with terminal CSI sequences even when stdout is
// redirected. Strip ONLY well-formed ANSI CSI controls, then validate the full
// JSON document and canonically serialize it before snapshot comparisons.
// Unexpected prefix/suffix text or unknown ESC controls remain a hard failure.
var ndmcCSISequenceM = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")

// parseNDMCTextInventoryM accepts the Keenetic CLI `show ip name-server`
// hierarchical text format. Preserve every field including service/interface,
// and the original server order, because priority/order drift matters.
// No unrecognized sections or duplicate fields are silently ignored.
func parseNDMCTextInventoryM(raw string) (map[string]any, error) {
	rows := []any{}
	var current map[string]any
	for n, row := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(row)
		if line == "" {
			continue
		}
		if strings.ContainsRune(line, '\x00') {
			return nil, fmt.Errorf("ndmc inventory line %d contains NUL", n+1)
		}
		if line == "server:" {
			current = make(map[string]any)
			rows = append(rows, current)
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || len(key) == 0 || len(key) > 64 || strings.ContainsAny(key, " \t=:,{[]}") {
			return nil, fmt.Errorf("invalid ndmc inventory field at line %d", n+1)
		}
		value = strings.TrimSpace(value)
		if current == nil {
			// Command control metadata, not resolver configuration.
			// Unknown top-level sections fail closed.
			if key == "prompt" && value != "" {
				continue
			}
			if key == "status" && (value == "0" || strings.EqualFold(value, "ok") || strings.EqualFold(value, "success")) {
				continue
			}
			return nil, fmt.Errorf("unexpected top-level ndmc field %q at line %d", key, n+1)
		}
		if !strings.HasPrefix(row, " ") && !strings.HasPrefix(row, "\t") {
			return nil, fmt.Errorf("unindented ndmc inventory field at line %d", n+1)
		}
		if _, exists := current[key]; exists {
			return nil, fmt.Errorf("duplicate ndmc inventory field %q", key)
		}
		if key == "port" {
			if value == "" {
				current[key] = 0
				continue
			}
			num, err := strconv.Atoi(value)
			if err != nil || num < 0 || num > 65535 {
				return nil, fmt.Errorf("invalid ndmc port at line %d", n+1)
			}
			current[key] = num
		} else {
			// Preserve even fields not interpreted by this transaction; comparing
			// the entire normalized snapshot cannot hide changes to those fields.
			if len(value) > 1024 {
				return nil, fmt.Errorf("oversized ndmc field at line %d", n+1)
			}
			current[key] = value
		}
	}
	if len(rows) == 0 {
		return nil, errors.New("ndmc inventory has no server records")
	}
	for i, entry := range rows {
		row := entry.(map[string]any)
		address, ok := row["address"].(string)
		if !ok || address == "" {
			return nil, fmt.Errorf("ndmc server %d missing address", i)
		}
		if _, ok := row["domain"]; !ok {
			return nil, fmt.Errorf("ndmc server %d missing domain", i)
		}
		if _, ok := row["port"]; !ok {
			row["port"] = 0
		}
	}
	return map[string]any{"server": rows}, nil
}

func canonicalNDMCSnapshotM(raw string) (string, error) {
	if len(raw) > 65536 {
		return "", errors.New("ndmc inventory exceeds 64 KiB")
	}
	clean := strings.TrimSpace(ndmcCSISequenceM.ReplaceAllString(raw, ""))
	if strings.ContainsRune(clean, 0x1b) {
		return "", errors.New("ndmc inventory contains unknown terminal escape")
	}
	var doc map[string]any
	if strings.HasPrefix(clean, "{") {
		if err := json.Unmarshal([]byte(clean), &doc); err != nil {
			return "", fmt.Errorf("invalid ndmc inventory JSON: %w", err)
		}
	} else {
		var err error
		doc, err = parseNDMCTextInventoryM(clean)
		if err != nil {
			return "", err
		}
	}
	servers, present := doc["server"]
	if !present || servers == nil {
		return "", errors.New("ndmc inventory missing server array")
	}
	if _, ok := servers.([]any); !ok {
		return "", errors.New("ndmc inventory server is not an array")
	}
	stable, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(stable), nil
}

func trialSnapshotM(ctx context.Context, d NDMCExecutorM) (string, error) {
	raw, err := d.Command(ctx, "show ip name-server")
	if err != nil {
		return "", err
	}
	return canonicalNDMCSnapshotM(raw)
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
	baseline, err := canonicalNDMCSnapshotM(j.Baseline)
	if err != nil {
		return err
	}
	if absent, err := absentFromSnapshotM(baseline); err != nil {
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
		if err == nil && state != baseline {
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
