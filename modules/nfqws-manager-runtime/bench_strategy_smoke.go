package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	benchTLSStrategySmokeConfirm = "ROUTERFORGE_TLS_STRATEGY_SMOKE"
	benchTLSStrategySmokeTimeout = 20 * time.Second
)

type benchTLSStrategySmokeRequest struct {
	ProfileIndex         int    `json:"profile_index"`
	ServerName           string `json:"server_name"`
	ExpectedConfigSHA256 string `json:"expected_config_sha256"`
	Confirm              string `json:"confirm"`
}

type benchTLSStrategySmokeResponse struct {
	OK                    bool                   `json:"ok"`
	SessionID             string                 `json:"session_id"`
	ProfileIndex          int                    `json:"profile_index"`
	ServerName            string                 `json:"server_name"`
	DestinationIPv4       string                 `json:"destination_ipv4"`
	LocalPort             int                    `json:"local_port"`
	Queue                 int                    `json:"queue"`
	CandidateArgCount     int                    `json:"candidate_arg_count"`
	TLSHandshakeComplete  bool                   `json:"tls_handshake_complete"`
	TLSVersion            uint16                 `json:"tls_version,omitempty"`
	CipherSuite           uint16                 `json:"cipher_suite,omitempty"`
	OutboundQueuePackets  uint64                 `json:"outbound_queue_packets"`
	InboundQueuePackets   uint64                 `json:"inbound_queue_packets"`
	StrategyPathExercised bool                   `json:"strategy_path_exercised"`
	Result                benchTransactionResult `json:"result"`
	CleanupBaselineAfter  bool                   `json:"cleanup_baseline_after"`
	BenchEnabled          bool                   `json:"bench_enabled"`
	SafeToBench           bool                   `json:"safe_to_bench"`
}

type benchTLSStrategyOps struct {
	*benchSystemOps
	candidateArgs         []string
	serverName            string
	tlsHandshakeComplete  bool
	tlsVersion            uint16
	cipherSuite           uint16
	outboundQueuePackets  uint64
	inboundQueuePackets   uint64
	strategyPathExercised bool
}

func registerBenchTLSStrategySmokeRoute(mux *http.ServeMux) {
	mux.HandleFunc("/v1/bench-strategy-smoke", mutationOnly(handleBenchTLSStrategySmoke))
}

func normalizeBenchServerName(raw string) (string, error) {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	if !checkHostPattern.MatchString(host) || net.ParseIP(host) != nil {
		return "", errors.New("server_name must be a DNS hostname")
	}
	return host, nil
}

func profileAllowsBenchServerName(profile benchStrategyProfile, host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	for _, domain := range profile.HostlistDomains {
		domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
		if domain != "" && (host == domain || strings.HasSuffix(host, "."+domain)) {
			return true
		}
	}
	return false
}

func resolveBenchServerIPv4(ctx context.Context, host string) (string, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}
	var candidates []string
	seen := map[string]bool{}
	for _, addr := range addrs {
		ip := addr.IP
		if ip == nil || ip.To4() == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() ||
			ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
			continue
		}
		value := ip.To4().String()
		if !seen[value] {
			seen[value] = true
			candidates = append(candidates, value)
		}
	}
	if len(candidates) == 0 {
		return "", errors.New("no public IPv4 address resolved")
	}
	sort.Strings(candidates)
	return candidates[0], nil
}

func parseIPTablesSavePacketCounter(line string) (uint64, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 || len(fields[0]) < 5 || fields[0][0] != '[' {
		return 0, false
	}
	token := strings.Trim(fields[0], "[]")
	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	value, err := strconv.ParseUint(parts[0], 10, 64)
	return value, err == nil
}

func (o *benchTLSStrategyOps) readRulePackets(ctx context.Context, comment string) (uint64, error) {
	output, err := runBenchCommand(ctx, o.iptablesSavePath, "-c", "-t", "mangle")
	if err != nil {
		return 0, err
	}
	found := 0
	var packets uint64
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.Contains(line, comment) {
			continue
		}
		value, ok := parseIPTablesSavePacketCounter(line)
		if !ok {
			return 0, errors.New("unable to parse iptables packet counter")
		}
		found++
		packets = value
	}
	if found != 1 {
		return 0, fmt.Errorf("expected exactly one counter line for %s, found %d", comment, found)
	}
	return packets, nil
}

func (o *benchTLSStrategyOps) StartCandidate(ctx context.Context, spec benchTransactionSpec) error {
	return startBenchCandidateWithArgs(ctx, o.benchSystemOps, spec, o.candidateArgs)
}

func (o *benchTLSStrategyOps) VerifyInstalled(ctx context.Context, spec benchTransactionSpec, rules []benchRuleSpec) error {
	if err := o.benchSystemOps.VerifyInstalled(ctx, spec, rules); err != nil {
		return err
	}
	pid, err := readBenchCandidatePID(spec.SessionID)
	if err != nil || !candidateProcessMatchesArgv(pid, spec, o.candidateArgs) {
		return errors.New("strategy candidate exact argv identity is not proven")
	}
	return nil
}

func (o *benchTLSStrategyOps) Probe(ctx context.Context, spec benchTransactionSpec) error {
	outComment := benchRuleComment(spec.SessionID, "out-queue")
	inComment := benchRuleComment(spec.SessionID, "in-queue")

	outBefore, err := o.readRulePackets(ctx, outComment)
	if err != nil {
		return fmt.Errorf("read outbound queue counter before probe: %w", err)
	}
	inBefore, err := o.readRulePackets(ctx, inComment)
	if err != nil {
		return fmt.Errorf("read inbound queue counter before probe: %w", err)
	}

	dialer := net.Dialer{
		Timeout: benchProbeTimeout,
		LocalAddr: &net.TCPAddr{
			IP:   net.IPv4zero,
			Port: spec.LocalPort,
		},
	}
	tcpConn, err := dialer.DialContext(ctx, "tcp4", net.JoinHostPort(spec.DestinationIPv4, "443"))
	if err != nil {
		return fmt.Errorf("tcp connect: %w", err)
	}

	tlsConn := tls.Client(tcpConn, &tls.Config{
		ServerName:         o.serverName,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		NextProtos:         []string{"http/1.1"},
	})
	handshakeCtx, cancel := context.WithTimeout(ctx, benchProbeTimeout)
	err = tlsConn.HandshakeContext(handshakeCtx)
	cancel()
	if err != nil {
		_ = tlsConn.Close()
		return fmt.Errorf("tls handshake: %w", err)
	}

	state := tlsConn.ConnectionState()
	o.tlsHandshakeComplete = state.HandshakeComplete
	o.tlsVersion = state.Version
	o.cipherSuite = state.CipherSuite
	_ = tlsConn.Close()

	outAfter, err := o.readRulePackets(ctx, outComment)
	if err != nil {
		return fmt.Errorf("read outbound queue counter after probe: %w", err)
	}
	inAfter, err := o.readRulePackets(ctx, inComment)
	if err != nil {
		return fmt.Errorf("read inbound queue counter after probe: %w", err)
	}
	if outAfter <= outBefore {
		return errors.New("outbound NFQUEUE rule counter did not increase")
	}
	if inAfter <= inBefore {
		return errors.New("inbound NFQUEUE rule counter did not increase")
	}
	o.outboundQueuePackets = outAfter - outBefore
	o.inboundQueuePackets = inAfter - inBefore

	pid, err := readBenchCandidatePID(spec.SessionID)
	if err != nil || !candidateProcessMatchesArgv(pid, spec, o.candidateArgs) {
		return errors.New("strategy candidate identity was lost during TLS probe")
	}
	bound, err := benchQueueIsBound(spec.Queue)
	if err != nil || !bound {
		return errors.New("strategy candidate NFQUEUE binding was lost during TLS probe")
	}

	o.strategyPathExercised = o.tlsHandshakeComplete &&
		o.outboundQueuePackets > 0 && o.inboundQueuePackets > 0
	if !o.strategyPathExercised {
		return errors.New("TLS strategy path was not proven")
	}
	return nil
}

func validateBenchTLSStrategySmokeRequest(request benchTLSStrategySmokeRequest) error {
	if request.ProfileIndex < 0 {
		return errors.New("profile_index must be non-negative")
	}
	if _, err := normalizeBenchServerName(request.ServerName); err != nil {
		return err
	}
	if request.Confirm != benchTLSStrategySmokeConfirm {
		return errors.New("confirm must equal " + benchTLSStrategySmokeConfirm)
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(request.ExpectedConfigSHA256)) {
		return errors.New("expected_config_sha256 must be SHA256")
	}
	return nil
}

func handleBenchTLSStrategySmoke(w http.ResponseWriter, r *http.Request) {
	var request benchTLSStrategySmokeRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid TLS strategy smoke request"})
		return
	}
	if err := validateBenchTLSStrategySmokeRequest(request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !atomic.CompareAndSwapInt32(&benchSmokeActive, 0, 1) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "another bench session is active"})
		return
	}
	defer atomic.StoreInt32(&benchSmokeActive, 0)

	status := readStatus()
	if !strings.EqualFold(status.ConfigSHA256, strings.TrimSpace(request.ExpectedConfigSHA256)) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":          "production config changed before TLS strategy smoke",
			"current_sha256": status.ConfigSHA256,
		})
		return
	}

	capabilities := readBenchCapabilities()
	if !capabilities.CandidateSpawnCapable || !capabilities.QueueInventoryComplete ||
		!capabilities.CleanupBaselineProven || capabilities.RecommendedQueue == 0 ||
		capabilities.IPTablesSavePath == "" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "TLS strategy smoke capability gates are not proven"})
		return
	}

	inventory := readBenchStrategyInventory()
	profile, err := findBenchStrategyProfile(inventory, request.ProfileIndex)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	serverName, err := normalizeBenchServerName(request.ServerName)
	if err != nil || !profileAllowsBenchServerName(profile, serverName) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "server_name is not covered by the selected live profile"})
		return
	}

	resolveCtx, resolveCancel := context.WithTimeout(r.Context(), 4*time.Second)
	destinationIPv4, err := resolveBenchServerIPv4(resolveCtx, serverName)
	resolveCancel()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "resolve server_name: " + err.Error()})
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
		DestinationIPv4: destinationIPv4,
		LocalPort:       localPort,
		Queue:           capabilities.RecommendedQueue,
	}
	if err := validateBenchTransactionSpec(spec); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	candidateArgs, err := buildBenchStrategyCandidateArgs(spec, inventory, profile)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}

	baselineProcesses, _ := readBenchProcesses()
	systemOps := &benchSystemOps{
		iptablesPath:      capabilities.IPTablesPath,
		iptablesSavePath:  capabilities.IPTablesSavePath,
		candidateBinary:   capabilities.CandidateBinary,
		baselineConfigSHA: status.ConfigSHA256,
		baselineProcesses: baselineProcesses,
	}
	ops := &benchTLSStrategyOps{
		benchSystemOps: systemOps,
		candidateArgs:  candidateArgs,
		serverName:     serverName,
	}

	ctx, cancel := context.WithTimeout(r.Context(), benchTLSStrategySmokeTimeout)
	result := runBenchTransaction(ctx, ops, spec)
	cancel()

	after := readBenchCapabilities()
	ok := result.Error == "" && result.CleanupAttempted && result.CleanupProven &&
		result.State == benchLifecycleStateClean && after.CleanupBaselineProven &&
		ops.tlsHandshakeComplete && ops.strategyPathExercised &&
		ops.outboundQueuePackets > 0 && ops.inboundQueuePackets > 0

	response := benchTLSStrategySmokeResponse{
		OK:                    ok,
		SessionID:             spec.SessionID,
		ProfileIndex:          profile.Index,
		ServerName:            serverName,
		DestinationIPv4:       spec.DestinationIPv4,
		LocalPort:             spec.LocalPort,
		Queue:                 spec.Queue,
		CandidateArgCount:     len(candidateArgs),
		TLSHandshakeComplete:  ops.tlsHandshakeComplete,
		TLSVersion:            ops.tlsVersion,
		CipherSuite:           ops.cipherSuite,
		OutboundQueuePackets:  ops.outboundQueuePackets,
		InboundQueuePackets:   ops.inboundQueuePackets,
		StrategyPathExercised: ops.strategyPathExercised,
		Result:                result,
		CleanupBaselineAfter:  after.CleanupBaselineProven,
		BenchEnabled:          false,
		SafeToBench:           false,
	}
	if !ok {
		writeJSON(w, http.StatusBadGateway, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}
