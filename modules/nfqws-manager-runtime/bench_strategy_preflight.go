package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	benchStrategyPreflightConfirm = "ROUTERFORGE_STRATEGY_PREFLIGHT"
	benchStrategyPreflightTimeout = 12 * time.Second
)

type benchStrategyPreflightRequest struct {
	ProfileIndex         int    `json:"profile_index"`
	ExpectedConfigSHA256 string `json:"expected_config_sha256"`
	Confirm              string `json:"confirm"`
}

type benchStrategyPreflightResponse struct {
	OK                    bool     `json:"ok"`
	SessionID             string   `json:"session_id"`
	ProfileIndex          int      `json:"profile_index"`
	Domains               []string `json:"domains"`
	Queue                 int      `json:"queue"`
	CandidateArgCount     int      `json:"candidate_arg_count"`
	ProcessStarted        bool     `json:"process_started"`
	QueueBound            bool     `json:"queue_bound"`
	ProcessIdentityProven bool     `json:"process_identity_proven"`
	CleanupAttempted      bool     `json:"cleanup_attempted"`
	CleanupProven         bool     `json:"cleanup_proven"`
	CleanupBaselineAfter  bool     `json:"cleanup_baseline_after"`
	Error                 string   `json:"error,omitempty"`
	BenchEnabled          bool     `json:"bench_enabled"`
	SafeToBench           bool     `json:"safe_to_bench"`
}

func registerBenchStrategyPreflightRoute(mux *http.ServeMux) {
	mux.HandleFunc("/v1/bench-strategy-preflight", mutationOnly(handleBenchStrategyPreflight))
}

func findBenchStrategyProfile(inventory benchStrategyInventory, index int) (benchStrategyProfile, error) {
	for _, profile := range inventory.Profiles {
		if profile.Index == index {
			if !profile.CandidateEligible {
				return benchStrategyProfile{}, errors.New("selected profile is not eligible for first-scope TLS bench")
			}
			return profile, nil
		}
	}
	return benchStrategyProfile{}, errors.New("strategy profile index not found")
}

func buildBenchStrategyCandidateArgs(spec benchTransactionSpec, inventory benchStrategyInventory, profile benchStrategyProfile) ([]string, error) {
	if !inventory.BaseDependenciesProven {
		return nil, errors.New("live strategy base dependencies are not proven")
	}
	if !profile.CandidateEligible {
		return nil, errors.New("strategy profile is not eligible")
	}

	args := append([]string{}, benchCandidateArgs(spec)...)
	args = append(args, inventory.BaseArgs...)
	args = append(args, profile.Args...)
	return args, nil
}

func candidateProcessMatchesArgv(pid int, spec benchTransactionSpec, expectedArgs []string) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return false
	}
	parts := strings.Split(string(data), "\x00")
	actual := make(map[string]int, len(parts))
	for _, part := range parts {
		if part != "" {
			actual[part]++
		}
	}
	if !benchCandidateProcessMatches(pid, spec) {
		return false
	}
	for _, arg := range expectedArgs {
		if actual[arg] == 0 {
			return false
		}
		actual[arg]--
	}
	return true
}

func startBenchCandidateWithArgs(ctx context.Context, ops *benchSystemOps, spec benchTransactionSpec, args []string) error {
	pidFile := benchCandidatePIDFile(spec.SessionID)
	if _, err := os.Stat(pidFile); err == nil {
		return errors.New("bench candidate pidfile already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	startCtx, cancel := context.WithTimeout(ctx, benchCandidateStartTimeout)
	output, err := runBenchCandidateStartCommand(startCtx, ops.candidateBinary, args...)
	ctxErr := startCtx.Err()
	cancel()
	if err != nil || ctxErr != nil {
		_ = ops.StopCandidate(context.Background(), spec)
		if ctxErr != nil {
			return errors.New("candidate start timed out")
		}
		return errors.New("candidate start failed: " + strings.TrimSpace(string(output)))
	}

	deadline := time.Now().Add(benchCandidateStartTimeout)
	for time.Now().Before(deadline) {
		pid, pidErr := readBenchCandidatePID(spec.SessionID)
		bound, queueErr := benchQueueIsBound(spec.Queue)
		if pidErr == nil && queueErr == nil && bound && candidateProcessMatchesArgv(pid, spec, args) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	_ = ops.StopCandidate(context.Background(), spec)
	return errors.New("candidate did not prove exact argv identity and NFQUEUE binding")
}

func validateBenchStrategyPreflightRequest(request benchStrategyPreflightRequest) error {
	if request.ProfileIndex < 0 {
		return errors.New("profile_index must be non-negative")
	}
	if request.Confirm != benchStrategyPreflightConfirm {
		return errors.New("confirm must equal " + benchStrategyPreflightConfirm)
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(request.ExpectedConfigSHA256)) {
		return errors.New("expected_config_sha256 must be SHA256")
	}
	return nil
}

func handleBenchStrategyPreflight(w http.ResponseWriter, r *http.Request) {
	var request benchStrategyPreflightRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid strategy preflight request"})
		return
	}
	if err := validateBenchStrategyPreflightRequest(request); err != nil {
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
			"error":          "production config changed before strategy preflight",
			"current_sha256": status.ConfigSHA256,
		})
		return
	}

	capabilities := readBenchCapabilities()
	if !capabilities.CandidateSpawnCapable || !capabilities.QueueInventoryComplete ||
		!capabilities.CleanupBaselineProven || capabilities.RecommendedQueue == 0 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "strategy preflight capability gates are not proven"})
		return
	}

	inventory := readBenchStrategyInventory()
	profile, err := findBenchStrategyProfile(inventory, request.ProfileIndex)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	sessionID, err := newBenchSessionID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create bench session id: " + err.Error()})
		return
	}

	spec := benchTransactionSpec{
		SessionID:       sessionID,
		DestinationIPv4: "1.1.1.1",
		LocalPort:       44300,
		Queue:           capabilities.RecommendedQueue,
	}
	args, err := buildBenchStrategyCandidateArgs(spec, inventory, profile)
	if err != nil {
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

	response := benchStrategyPreflightResponse{
		SessionID:         sessionID,
		ProfileIndex:      profile.Index,
		Domains:           append([]string{}, profile.HostlistDomains...),
		Queue:             spec.Queue,
		CandidateArgCount: len(args),
		BenchEnabled:      false,
		SafeToBench:       false,
	}

	ctx, cancel := context.WithTimeout(r.Context(), benchStrategyPreflightTimeout)
	startErr := startBenchCandidateWithArgs(ctx, ops, spec, args)
	cancel()
	if startErr == nil {
		response.ProcessStarted = true
		pid, pidErr := readBenchCandidatePID(spec.SessionID)
		bound, queueErr := benchQueueIsBound(spec.Queue)
		response.QueueBound = queueErr == nil && bound
		response.ProcessIdentityProven = pidErr == nil && candidateProcessMatchesArgv(pid, spec, args)
	}

	response.CleanupAttempted = true
	stopErr := ops.StopCandidate(context.Background(), spec)
	cleanupErr := ops.VerifyCleanup(context.Background(), spec)
	after := readBenchCapabilities()
	response.CleanupBaselineAfter = after.CleanupBaselineProven
	response.BenchEnabled = after.BenchEnabled
	response.SafeToBench = after.SafeToBench
	response.CleanupProven = stopErr == nil && cleanupErr == nil && after.CleanupBaselineProven

	var failures []string
	if startErr != nil {
		failures = append(failures, "start: "+startErr.Error())
	}
	if startErr == nil && (!response.QueueBound || !response.ProcessIdentityProven) {
		failures = append(failures, "candidate start proof incomplete")
	}
	if stopErr != nil {
		failures = append(failures, "stop: "+stopErr.Error())
	}
	if cleanupErr != nil {
		failures = append(failures, "cleanup: "+cleanupErr.Error())
	}
	processesAfter, _ := readBenchProcesses()
	if !reflect.DeepEqual(baselineProcesses, processesAfter) {
		failures = append(failures, "production process snapshot changed")
	}

	response.OK = len(failures) == 0 && response.ProcessStarted &&
		response.QueueBound && response.ProcessIdentityProven && response.CleanupProven
	if len(failures) > 0 {
		response.Error = strings.Join(failures, "; ")
	}
	if !response.OK {
		writeJSON(w, http.StatusBadGateway, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}
