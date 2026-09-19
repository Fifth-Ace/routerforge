package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	benchMutationCommandTimeout = 4 * time.Second
	benchCandidateStartTimeout  = 3 * time.Second
	benchCandidateStopTimeout   = 3 * time.Second
	benchProbeTimeout           = 5 * time.Second
	benchSmokeTimeout           = 15 * time.Second
	benchSmokeConfirm           = "ROUTERFORGE_BENCH_SMOKE"
)

type benchSmokeRequest struct {
	DestinationIPv4      string `json:"destination_ipv4"`
	ExpectedConfigSHA256 string `json:"expected_config_sha256"`
	Confirm              string `json:"confirm"`
}

type benchSmokeResponse struct {
	OK                   bool                   `json:"ok"`
	SessionID            string                 `json:"session_id"`
	DestinationIPv4      string                 `json:"destination_ipv4"`
	LocalPort            int                    `json:"local_port"`
	Queue                int                    `json:"queue"`
	Result               benchTransactionResult `json:"result"`
	CleanupBaselineAfter bool                   `json:"cleanup_baseline_after"`
	BenchEnabled         bool                   `json:"bench_enabled"`
	SafeToBench          bool                   `json:"safe_to_bench"`
}

type benchSystemOps struct {
	iptablesPath      string
	iptablesSavePath  string
	candidateBinary   string
	baselineConfigSHA string
	baselineProcesses []benchProcessInfo
}

var benchSmokeActive int32

func registerBenchSmokeRoute(mux *http.ServeMux) {
	mux.HandleFunc("/v1/bench-smoke", mutationOnly(handleBenchSmoke))
}

func benchCandidatePIDFile(sessionID string) string {
	return "/tmp/routerforge-bench-" + sessionID + ".pid"
}

func benchCandidateArgs(spec benchTransactionSpec) []string {
	return []string{
		"--daemon",
		"--pidfile=" + benchCandidatePIDFile(spec.SessionID),
		"--user=nobody",
		"--qnum=" + strconv.Itoa(spec.Queue),
		"--fwmark=0x40000000",
	}
}

func newBenchSessionID() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "rf-" + hex.EncodeToString(raw[:]), nil
}

func allocateBenchLocalPort() (int, error) {
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return 0, err
	}
	if port < 1024 || port > 65535 {
		return 0, errors.New("allocated local port is outside allowed range")
	}
	return port, nil
}

var benchFirewallCommandMu sync.Mutex

func benchFirewallCommand(program string) bool {
	return program == "iptables" || program == "iptables-save" ||
		strings.HasSuffix(program, "/iptables") || strings.HasSuffix(program, "/iptables-save")
}

func runBenchCommand(ctx context.Context, program string, args ...string) ([]byte, error) {
	if benchFirewallCommand(program) {
		benchFirewallCommandMu.Lock()
		defer benchFirewallCommandMu.Unlock()
	}
	commandCtx, cancel := context.WithTimeout(ctx, benchMutationCommandTimeout)
	output, err := safety.RunCommand(commandCtx, benchOutputMax, program, args...)
	ctxErr := commandCtx.Err()
	cancel()
	if ctxErr != nil {
		return output, ctxErr
	}
	return output, err
}

func readBenchCandidatePID(sessionID string) (int, error) {
	data, err := os.ReadFile(benchCandidatePIDFile(sessionID))
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 {
		return 0, errors.New("invalid bench candidate pidfile")
	}
	return pid, nil
}

func benchCandidateProcessMatches(pid int, spec benchTransactionSpec) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	cmdline := strings.ReplaceAll(string(data), "\x00", " ")
	return strings.Contains(cmdline, "nfqws2") &&
		strings.Contains(cmdline, "--qnum="+strconv.Itoa(spec.Queue)) &&
		strings.Contains(cmdline, "--pidfile="+benchCandidatePIDFile(spec.SessionID))
}

func benchQueueIsBound(queue int) (bool, error) {
	queues, ok := readKernelQueueInventory()
	if !ok {
		return false, errors.New("kernel NFQUEUE inventory unavailable")
	}
	for _, q := range queues {
		if q == queue {
			return true, nil
		}
	}
	return false, nil
}

func (o *benchSystemOps) sessionCommentPresent(ctx context.Context, sessionID string) (bool, error) {
	output, err := runBenchCommand(ctx, o.iptablesSavePath)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(output), "routerforge-bench:"+sessionID+":"), nil
}

func (o *benchSystemOps) rulePresent(ctx context.Context, rule benchRuleSpec) (bool, error) {
	output, err := runBenchCommand(ctx, o.iptablesPath, "-t", rule.Table, "-S", rule.Chain)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(output), rule.Comment), nil
}

func (o *benchSystemOps) anchorPosition(ctx context.Context, rule benchRuleSpec) (int, error) {
	output, err := runBenchCommand(ctx, o.iptablesPath, "-t", rule.Table, "-S", rule.Chain)
	if err != nil {
		return 0, err
	}
	position := 0
	found := 0
	foundPosition := 0
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 || fields[0] != "-A" || fields[1] != rule.Chain {
			continue
		}
		position++
		if len(fields) == 4 && fields[2] == "-j" && fields[3] == rule.Anchor {
			found++
			foundPosition = position
		}
	}
	if found != 1 {
		return 0, fmt.Errorf("expected exactly one %s anchor in %s, found %d", rule.Anchor, rule.Chain, found)
	}
	return foundPosition, nil
}

func (o *benchSystemOps) StartCandidate(ctx context.Context, spec benchTransactionSpec) error {
	pidFile := benchCandidatePIDFile(spec.SessionID)
	if _, err := os.Stat(pidFile); err == nil {
		return errors.New("bench candidate pidfile already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	startCtx, cancel := context.WithTimeout(ctx, benchCandidateStartTimeout)
	output, err := safety.RunCommand(startCtx, benchOutputMax, o.candidateBinary, benchCandidateArgs(spec)...)
	ctxErr := startCtx.Err()
	cancel()
	if err != nil || ctxErr != nil {
		_ = o.StopCandidate(context.Background(), spec)
		if ctxErr != nil {
			return fmt.Errorf("candidate start timed out: %w", ctxErr)
		}
		return fmt.Errorf("candidate start failed: %w output=%s", err, strings.TrimSpace(string(output)))
	}

	deadline := time.Now().Add(benchCandidateStartTimeout)
	for time.Now().Before(deadline) {
		pid, pidErr := readBenchCandidatePID(spec.SessionID)
		bound, queueErr := benchQueueIsBound(spec.Queue)
		if pidErr == nil && queueErr == nil && bound && benchCandidateProcessMatches(pid, spec) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	_ = o.StopCandidate(context.Background(), spec)
	return errors.New("candidate did not prove pid identity and NFQUEUE binding")
}

func (o *benchSystemOps) InstallRule(ctx context.Context, rule benchRuleSpec) error {
	present, err := o.rulePresent(ctx, rule)
	if err != nil {
		return err
	}
	if present {
		return errors.New("bench rule already exists before install")
	}

	anchor, err := o.anchorPosition(ctx, rule)
	if err != nil {
		return err
	}
	insertAt := anchor
	if rule.Relation == "after" {
		insertAt = anchor + 1
	} else if rule.Relation != "before" {
		return errors.New("unsupported bench rule relation")
	}

	args := []string{"-t", rule.Table, "-I", rule.Chain, strconv.Itoa(insertAt)}
	args = append(args, rule.RuleArgs...)
	output, err := runBenchCommand(ctx, o.iptablesPath, args...)
	if err != nil {
		if leftover, checkErr := o.rulePresent(context.Background(), rule); checkErr == nil && leftover {
			_ = o.DeleteRule(context.Background(), rule)
		}
		return fmt.Errorf("iptables insert failed: %w output=%s", err, strings.TrimSpace(string(output)))
	}

	present, err = o.rulePresent(ctx, rule)
	if err != nil {
		return err
	}
	if !present {
		return errors.New("inserted bench rule was not observable")
	}
	return nil
}

func (o *benchSystemOps) VerifyInstalled(ctx context.Context, spec benchTransactionSpec, rules []benchRuleSpec) error {
	if len(rules) != 6 {
		return fmt.Errorf("unexpected bench rule count %d", len(rules))
	}
	pid, err := readBenchCandidatePID(spec.SessionID)
	if err != nil || !benchCandidateProcessMatches(pid, spec) {
		return errors.New("candidate process identity is not proven")
	}
	bound, err := benchQueueIsBound(spec.Queue)
	if err != nil || !bound {
		return errors.New("candidate NFQUEUE binding is not proven")
	}
	for _, rule := range rules {
		present, err := o.rulePresent(ctx, rule)
		if err != nil || !present {
			return fmt.Errorf("bench rule %s is not proven", rule.Name)
		}
	}
	for _, chainSpec := range []struct {
		chain      string
		markName   string
		queueName  string
		clearName  string
		anchorName string
	}{
		{"POSTROUTING", "out-mark", "out-queue", "out-clear", "nfqws_post"},
		{"PREROUTING", "in-mark", "in-queue", "in-clear", "nfqws_pre"},
	} {
		output, err := runBenchCommand(ctx, o.iptablesPath, "-t", "mangle", "-S", chainSpec.chain)
		if err != nil {
			return err
		}
		lines := strings.Split(string(output), "\n")
		index := func(needle string) int {
			for i, line := range lines {
				if strings.Contains(line, needle) {
					return i
				}
			}
			return -1
		}
		mark := index("routerforge-bench:" + spec.SessionID + ":" + chainSpec.markName)
		queue := index("routerforge-bench:" + spec.SessionID + ":" + chainSpec.queueName)
		clear := index("routerforge-bench:" + spec.SessionID + ":" + chainSpec.clearName)
		anchor := index("-A " + chainSpec.chain + " -j " + chainSpec.anchorName)
		if mark < 0 || queue < 0 || clear < 0 || anchor < 0 || !(mark < queue && queue < anchor && anchor < clear) {
			return fmt.Errorf("bench rule order is not proven in %s", chainSpec.chain)
		}
	}
	if current := readStatus().ConfigSHA256; current != o.baselineConfigSHA {
		return errors.New("production config changed during bench setup")
	}
	return nil
}

func (o *benchSystemOps) Probe(ctx context.Context, spec benchTransactionSpec) error {
	probeCtx, cancel := context.WithTimeout(ctx, benchProbeTimeout)
	defer cancel()

	dialer := net.Dialer{
		Timeout: benchProbeTimeout,
		LocalAddr: &net.TCPAddr{
			IP:   net.IPv4zero,
			Port: spec.LocalPort,
		},
	}
	conn, err := dialer.DialContext(probeCtx, "tcp4", net.JoinHostPort(spec.DestinationIPv4, "443"))
	if err != nil {
		return err
	}
	return conn.Close()
}

func (o *benchSystemOps) DeleteRule(ctx context.Context, rule benchRuleSpec) error {
	present, err := o.rulePresent(ctx, rule)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	args := []string{"-t", rule.Table, "-D", rule.Chain}
	args = append(args, rule.RuleArgs...)
	output, err := runBenchCommand(ctx, o.iptablesPath, args...)
	if err != nil {
		stillPresent, checkErr := o.rulePresent(context.Background(), rule)
		if checkErr == nil && !stillPresent {
			return nil
		}
		return fmt.Errorf("iptables delete failed: %w output=%s", err, strings.TrimSpace(string(output)))
	}
	stillPresent, err := o.rulePresent(ctx, rule)
	if err != nil {
		return err
	}
	if stillPresent {
		return errors.New("deleted bench rule is still observable")
	}
	return nil
}

func (o *benchSystemOps) StopCandidate(_ context.Context, spec benchTransactionSpec) error {
	pidFile := benchCandidatePIDFile(spec.SessionID)
	pid, err := readBenchCandidatePID(spec.SessionID)
	if errors.Is(err, os.ErrNotExist) {
		bound, queueErr := benchQueueIsBound(spec.Queue)
		if queueErr != nil {
			return queueErr
		}
		if bound {
			return errors.New("bench queue remains bound without candidate pidfile")
		}
		return nil
	}
	if err != nil {
		return err
	}
	if !benchCandidateProcessMatches(pid, spec) {
		return errors.New("refusing to signal unproven candidate pid")
	}

	_ = syscall.Kill(pid, syscall.SIGTERM)
	deadline := time.Now().Add(benchCandidateStopTimeout)
	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(fmt.Sprintf("/proc/%d", pid)); errors.Is(statErr, os.ErrNotExist) {
			_ = os.Remove(pidFile)
			bound, queueErr := benchQueueIsBound(spec.Queue)
			if queueErr == nil && !bound {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	if benchCandidateProcessMatches(pid, spec) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, statErr := os.Stat(fmt.Sprintf("/proc/%d", pid))
		bound, queueErr := benchQueueIsBound(spec.Queue)
		if errors.Is(statErr, os.ErrNotExist) && queueErr == nil && !bound {
			_ = os.Remove(pidFile)
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("candidate process or NFQUEUE binding survived stop")
}

func (o *benchSystemOps) VerifyCleanup(ctx context.Context, spec benchTransactionSpec) error {
	var problems []string
	if present, err := o.sessionCommentPresent(ctx, spec.SessionID); err != nil {
		problems = append(problems, "firewall inventory: "+err.Error())
	} else if present {
		problems = append(problems, "session firewall rules remain")
	}
	if bound, err := benchQueueIsBound(spec.Queue); err != nil {
		problems = append(problems, "kernel queue inventory: "+err.Error())
	} else if bound {
		problems = append(problems, "reserved queue remains bound")
	}
	if _, err := os.Stat(benchCandidatePIDFile(spec.SessionID)); err == nil {
		problems = append(problems, "candidate pidfile remains")
	} else if !errors.Is(err, os.ErrNotExist) {
		problems = append(problems, "candidate pidfile stat: "+err.Error())
	}
	if current := readStatus().ConfigSHA256; current != o.baselineConfigSHA {
		problems = append(problems, "production config SHA changed")
	}
	processes, _ := readBenchProcesses()
	if !reflect.DeepEqual(processes, o.baselineProcesses) {
		problems = append(problems, "nfqws2 process snapshot changed")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func validateBenchSmokeRequest(request benchSmokeRequest) error {
	if request.Confirm != benchSmokeConfirm {
		return errors.New("confirm must equal " + benchSmokeConfirm)
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(request.ExpectedConfigSHA256)) {
		return errors.New("expected_config_sha256 must be SHA256")
	}
	ip := net.ParseIP(strings.TrimSpace(request.DestinationIPv4))
	if ip == nil || ip.To4() == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return errors.New("destination_ipv4 must be a public unicast IPv4 address")
	}
	return nil
}

func handleBenchSmoke(w http.ResponseWriter, r *http.Request) {
	var request benchSmokeRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid bench smoke request"})
		return
	}
	if err := validateBenchSmokeRequest(request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !atomic.CompareAndSwapInt32(&benchSmokeActive, 0, 1) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "another bench smoke session is active"})
		return
	}
	defer atomic.StoreInt32(&benchSmokeActive, 0)

	status := readStatus()
	if !strings.EqualFold(status.ConfigSHA256, strings.TrimSpace(request.ExpectedConfigSHA256)) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":          "production config changed before bench smoke",
			"current_sha256": status.ConfigSHA256,
		})
		return
	}

	capabilities := readBenchCapabilities()
	if !capabilities.CandidateSpawnCapable || !capabilities.QueueInventoryComplete ||
		!capabilities.CleanupBaselineProven || capabilities.RecommendedQueue == 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":        "bench smoke preconditions are not proven",
			"capabilities": capabilities,
		})
		return
	}

	localPort, err := allocateBenchLocalPort()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "allocate local probe port: " + err.Error()})
		return
	}
	sessionID, err := newBenchSessionID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create bench session id: " + err.Error()})
		return
	}
	spec := benchTransactionSpec{
		SessionID:       sessionID,
		DestinationIPv4: strings.TrimSpace(request.DestinationIPv4),
		LocalPort:       localPort,
		Queue:           capabilities.RecommendedQueue,
	}
	if err := validateBenchTransactionSpec(spec); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}

	baselineProcesses, _ := readBenchProcesses()
	ops := &benchSystemOps{
		iptablesPath:      capabilities.IPTablesPath,
		iptablesSavePath:  capabilities.IPTablesSavePath,
		candidateBinary:   capabilities.CandidateBinary,
		baselineConfigSHA: status.ConfigSHA256,
		baselineProcesses: baselineProcesses,
	}

	ctx, cancel := context.WithTimeout(r.Context(), benchSmokeTimeout)
	result := runBenchTransaction(ctx, ops, spec)
	cancel()

	after := readBenchCapabilities()
	ok := result.Error == "" && result.CleanupAttempted && result.CleanupProven &&
		result.State == benchLifecycleStateClean && after.CleanupBaselineProven

	response := benchSmokeResponse{
		OK:                   ok,
		SessionID:            spec.SessionID,
		DestinationIPv4:      spec.DestinationIPv4,
		LocalPort:            spec.LocalPort,
		Queue:                spec.Queue,
		Result:               result,
		CleanupBaselineAfter: after.CleanupBaselineProven,
		BenchEnabled:         after.BenchEnabled,
		SafeToBench:          after.SafeToBench,
	}
	if !ok {
		writeJSON(w, http.StatusBadGateway, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}
