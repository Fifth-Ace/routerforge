package main

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const benchAutoTuneConfirm = "ROUTERFORGE_AUTOTUNE_TLS"

type benchAutoTuneRequest struct {
	Mode                 string `json:"mode"`
	ServerName           string `json:"server_name"`
	ExpectedConfigSHA256 string `json:"expected_config_sha256"`
	Confirm              string `json:"confirm"`
}

type benchAutoTuneMode struct {
	Name          string `json:"name"`
	Attempts      int    `json:"attempts"`
	MaxCandidates int    `json:"max_candidates"`
	TimeoutSec    int    `json:"timeout_sec"`
}

type benchAutoTuneAttempt struct {
	OK                   bool   `json:"ok"`
	SessionID            string `json:"session_id"`
	DestinationIPv4      string `json:"destination_ipv4"`
	LocalPort            int    `json:"local_port"`
	Queue                int    `json:"queue"`
	ProbeDurationMS      int64  `json:"probe_duration_ms"`
	OutboundQueuePackets uint64 `json:"outbound_queue_packets"`
	InboundQueuePackets  uint64 `json:"inbound_queue_packets"`
	CleanupProven        bool   `json:"cleanup_proven"`
	Error                string `json:"error,omitempty"`
}

type benchAutoTuneCandidate struct {
	Baseline           bool                   `json:"baseline"`
	SourceProfileIndex int                    `json:"source_profile_index"`
	StrategyTags       []int                  `json:"strategy_tags,omitempty"`
	Attempts           []benchAutoTuneAttempt `json:"attempts"`
	Successes          int                    `json:"successes"`
	Failures           int                    `json:"failures"`
	SuccessRate        float64                `json:"success_rate"`
	MedianProbeMS      int64                  `json:"median_probe_ms,omitempty"`
	CleanupProven      bool                   `json:"cleanup_proven"`
}

type benchAutoTuneResponse struct {
	OK                      bool                     `json:"ok"`
	Mode                    benchAutoTuneMode        `json:"mode"`
	ServerName              string                   `json:"server_name"`
	DestinationIPv4         string                   `json:"destination_ipv4"`
	MetricScope             string                   `json:"metric_scope"`
	Baseline                benchAutoTuneCandidate   `json:"baseline"`
	Candidates              []benchAutoTuneCandidate `json:"candidates"`
	RecommendationAvailable bool                     `json:"recommendation_available"`
	RecommendedProfileIndex int                      `json:"recommended_profile_index"`
	StrategyNeeded          bool                     `json:"strategy_needed"`
	RecommendationReason    string                   `json:"recommendation_reason"`
	CleanupBaselineAfter    bool                     `json:"cleanup_baseline_after"`
	BenchEnabled            bool                     `json:"bench_enabled"`
	SafeToBench             bool                     `json:"safe_to_bench"`
	ApplyEnabled            bool                     `json:"apply_enabled"`
	ApplyGateEligible       bool                     `json:"apply_gate_eligible"`
	ApplyGateToken          string                   `json:"apply_gate_token,omitempty"`
	ApplyGateExpiresAt      string                   `json:"apply_gate_expires_at,omitempty"`
	ApplyGateReason         string                   `json:"apply_gate_reason"`
}

func registerBenchAutoTuneRoute(mux *http.ServeMux) {
	mux.HandleFunc("/v1/bench-autotune", mutationOnly(handleBenchAutoTune))
	registerBenchAutoTuneApplyGateRoute(mux)
}

func parseBenchAutoTuneMode(raw string) (benchAutoTuneMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fast":
		return benchAutoTuneMode{Name: "fast", Attempts: 1, MaxCandidates: 3, TimeoutSec: 45}, nil
	case "normal":
		return benchAutoTuneMode{Name: "normal", Attempts: 2, MaxCandidates: 8, TimeoutSec: 90}, nil
	case "thorough":
		return benchAutoTuneMode{Name: "thorough", Attempts: 3, MaxCandidates: 16, TimeoutSec: 180}, nil
	default:
		return benchAutoTuneMode{}, errors.New("mode must be fast, normal or thorough")
	}
}

func retargetBenchStrategyProfile(profile benchStrategyProfile, serverName string) (benchStrategyProfile, error) {
	serverName, err := normalizeBenchServerName(serverName)
	if err != nil {
		return benchStrategyProfile{}, err
	}
	if !profile.CandidateEligible {
		return benchStrategyProfile{}, errors.New("profile is not eligible")
	}

	out := profile
	out.Args = append([]string{}, profile.Args...)
	out.HostlistDomains = []string{serverName}

	replaced := 0
	for i, arg := range out.Args {
		if strings.HasPrefix(arg, "--hostlist-domains=") {
			out.Args[i] = "--hostlist-domains=" + serverName
			replaced++
		}
	}
	if replaced != 1 {
		return benchStrategyProfile{}, errors.New("eligible strategy template must contain exactly one hostlist-domains argument")
	}
	return out, nil
}

func medianInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]int64{}, values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	mid := len(copyValues) / 2
	if len(copyValues)%2 == 1 {
		return copyValues[mid]
	}
	return (copyValues[mid-1] + copyValues[mid]) / 2
}

func finalizeBenchAutoTuneCandidate(candidate *benchAutoTuneCandidate) {
	candidate.Successes = 0
	candidate.Failures = 0
	candidate.CleanupProven = true
	var durations []int64
	for _, attempt := range candidate.Attempts {
		if attempt.OK {
			candidate.Successes++
			if attempt.ProbeDurationMS > 0 {
				durations = append(durations, attempt.ProbeDurationMS)
			}
		} else {
			candidate.Failures++
		}
		if !attempt.CleanupProven {
			candidate.CleanupProven = false
		}
	}
	if len(candidate.Attempts) > 0 {
		candidate.SuccessRate = float64(candidate.Successes) / float64(len(candidate.Attempts))
	}
	candidate.MedianProbeMS = medianInt64(durations)
}

func candidateBetterThan(a, b benchAutoTuneCandidate) bool {
	if a.SuccessRate != b.SuccessRate {
		return a.SuccessRate > b.SuccessRate
	}
	if a.Successes != b.Successes {
		return a.Successes > b.Successes
	}
	if a.MedianProbeMS == 0 {
		return false
	}
	if b.MedianProbeMS == 0 {
		return true
	}
	if a.MedianProbeMS != b.MedianProbeMS {
		return a.MedianProbeMS < b.MedianProbeMS
	}
	return a.SourceProfileIndex < b.SourceProfileIndex
}

func chooseBenchAutoTuneRecommendation(baseline benchAutoTuneCandidate, candidates []benchAutoTuneCandidate) (bool, int, bool, string) {
	var best *benchAutoTuneCandidate
	for i := range candidates {
		candidate := &candidates[i]
		if !candidate.CleanupProven || candidate.Successes == 0 {
			continue
		}
		if best == nil || candidateBetterThan(*candidate, *best) {
			best = candidate
		}
	}
	if best == nil {
		return false, 0, false, "no strategy candidate completed a successful TLS probe"
	}
	if best.SuccessRate <= baseline.SuccessRate {
		return false, 0, false, "baseline TLS reachability is not worse than the best strategy candidate"
	}
	return true, best.SourceProfileIndex, true, "strategy candidate improved TLS reachability over queue-only baseline"
}

func runBenchAutoTuneAttempt(ctx context.Context, capabilities benchCapabilities, configSHA, serverName, destinationIPv4 string, inventory benchStrategyInventory, profile *benchStrategyProfile) benchAutoTuneAttempt {
	localPort, err := allocateBenchLocalPort()
	if err != nil {
		return benchAutoTuneAttempt{Error: "allocate local port: " + err.Error()}
	}
	sessionID, err := newBenchSessionID()
	if err != nil {
		return benchAutoTuneAttempt{Error: "create session: " + err.Error()}
	}
	spec := benchTransactionSpec{
		SessionID:       sessionID,
		DestinationIPv4: destinationIPv4,
		LocalPort:       localPort,
		Queue:           capabilities.RecommendedQueue,
	}

	var candidateArgs []string
	if profile == nil {
		candidateArgs = benchCandidateArgs(spec)
	} else {
		candidateArgs, err = buildBenchStrategyCandidateArgs(spec, inventory, *profile)
		if err != nil {
			return benchAutoTuneAttempt{SessionID: sessionID, DestinationIPv4: destinationIPv4, LocalPort: localPort, Queue: spec.Queue, Error: err.Error()}
		}
	}

	baselineProcesses, _ := readBenchProcesses()
	systemOps := &benchSystemOps{
		iptablesPath:      capabilities.IPTablesPath,
		iptablesSavePath:  capabilities.IPTablesSavePath,
		candidateBinary:   capabilities.CandidateBinary,
		baselineConfigSHA: configSHA,
		baselineProcesses: baselineProcesses,
	}
	ops := &benchTLSStrategyOps{
		benchSystemOps: systemOps,
		candidateArgs:  candidateArgs,
		serverName:     serverName,
	}

	result := runBenchTransaction(ctx, ops, spec)
	after := readBenchCapabilities()
	ok := result.Error == "" && result.State == benchLifecycleStateClean &&
		result.CleanupAttempted && result.CleanupProven && after.CleanupBaselineProven &&
		ops.tlsHandshakeComplete && ops.strategyPathExercised

	attempt := benchAutoTuneAttempt{
		OK:                   ok,
		SessionID:            sessionID,
		DestinationIPv4:      destinationIPv4,
		LocalPort:            localPort,
		Queue:                spec.Queue,
		ProbeDurationMS:      ops.probeDurationMS,
		OutboundQueuePackets: ops.outboundQueuePackets,
		InboundQueuePackets:  ops.inboundQueuePackets,
		CleanupProven:        result.CleanupProven && after.CleanupBaselineProven,
	}
	if result.Error != "" {
		attempt.Error = result.Error
	}
	return attempt
}

func validateBenchAutoTuneRequest(request benchAutoTuneRequest) error {
	if _, err := parseBenchAutoTuneMode(request.Mode); err != nil {
		return err
	}
	if _, err := normalizeBenchServerName(request.ServerName); err != nil {
		return err
	}
	if request.Confirm != benchAutoTuneConfirm {
		return errors.New("confirm must equal " + benchAutoTuneConfirm)
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(request.ExpectedConfigSHA256)) {
		return errors.New("expected_config_sha256 must be SHA256")
	}
	return nil
}

func handleBenchAutoTune(w http.ResponseWriter, r *http.Request) {
	var request benchAutoTuneRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid AutoTune request"})
		return
	}
	if err := validateBenchAutoTuneRequest(request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !atomic.CompareAndSwapInt32(&benchSmokeActive, 0, 1) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "another bench session is active"})
		return
	}
	defer atomic.StoreInt32(&benchSmokeActive, 0)

	mode, _ := parseBenchAutoTuneMode(request.Mode)
	serverName, _ := normalizeBenchServerName(request.ServerName)

	status := readStatus()
	if !strings.EqualFold(status.ConfigSHA256, strings.TrimSpace(request.ExpectedConfigSHA256)) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":          "production config changed before AutoTune",
			"current_sha256": status.ConfigSHA256,
		})
		return
	}

	capabilities := readBenchCapabilities()
	if !capabilities.CandidateSpawnCapable || !capabilities.QueueInventoryComplete ||
		!capabilities.CleanupBaselineProven || capabilities.RecommendedQueue == 0 ||
		capabilities.IPTablesSavePath == "" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "AutoTune capability gates are not proven"})
		return
	}

	inventory := readBenchStrategyInventory()
	if !inventory.BaseDependenciesProven {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "strategy base dependencies are not proven"})
		return
	}

	var templates []benchStrategyProfile
	for _, profile := range inventory.Profiles {
		if !profile.CandidateEligible {
			continue
		}
		retargeted, err := retargetBenchStrategyProfile(profile, serverName)
		if err == nil {
			templates = append(templates, retargeted)
		}
		if len(templates) >= mode.MaxCandidates {
			break
		}
	}
	if len(templates) == 0 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "no retargetable strategy candidates"})
		return
	}

	resolveCtx, resolveCancel := context.WithTimeout(r.Context(), 4*time.Second)
	destinationIPv4, err := resolveBenchServerIPv4(resolveCtx, serverName)
	resolveCancel()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "resolve server_name: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(mode.TimeoutSec)*time.Second)
	defer cancel()

	baseline := benchAutoTuneCandidate{Baseline: true, CleanupProven: true}
	for i := 0; i < mode.Attempts; i++ {
		attempt := runBenchAutoTuneAttempt(ctx, capabilities, status.ConfigSHA256, serverName, destinationIPv4, inventory, nil)
		baseline.Attempts = append(baseline.Attempts, attempt)
		if !attempt.CleanupProven {
			finalizeBenchAutoTuneCandidate(&baseline)
			writeJSON(w, http.StatusBadGateway, benchAutoTuneResponse{
				OK: false, Mode: mode, ServerName: serverName, DestinationIPv4: destinationIPv4,
				MetricScope: "tls-reachability", Baseline: baseline,
				RecommendationReason: "baseline cleanup proof failed; AutoTune stopped fail-closed",
				CleanupBaselineAfter: false, BenchEnabled: false, SafeToBench: false, ApplyEnabled: false,
			})
			return
		}
	}
	finalizeBenchAutoTuneCandidate(&baseline)

	candidates := make([]benchAutoTuneCandidate, 0, len(templates))
	for _, profile := range templates {
		candidate := benchAutoTuneCandidate{
			SourceProfileIndex: profile.Index,
			StrategyTags:       append([]int{}, profile.StrategyTags...),
			CleanupProven:      true,
		}
		for i := 0; i < mode.Attempts; i++ {
			attempt := runBenchAutoTuneAttempt(ctx, capabilities, status.ConfigSHA256, serverName, destinationIPv4, inventory, &profile)
			candidate.Attempts = append(candidate.Attempts, attempt)
			if !attempt.CleanupProven {
				finalizeBenchAutoTuneCandidate(&candidate)
				candidates = append(candidates, candidate)
				writeJSON(w, http.StatusBadGateway, benchAutoTuneResponse{
					OK: false, Mode: mode, ServerName: serverName, DestinationIPv4: destinationIPv4,
					MetricScope: "tls-reachability", Baseline: baseline, Candidates: candidates,
					RecommendationReason: "candidate cleanup proof failed; AutoTune stopped fail-closed",
					CleanupBaselineAfter: false, BenchEnabled: false, SafeToBench: false, ApplyEnabled: false,
				})
				return
			}
		}
		finalizeBenchAutoTuneCandidate(&candidate)
		candidates = append(candidates, candidate)
	}

	recommendationAvailable, profileIndex, strategyNeeded, reason := chooseBenchAutoTuneRecommendation(baseline, candidates)
	after := readBenchCapabilities()
	ok := after.CleanupBaselineProven

	applyGateEligible := false
	applyGateToken := ""
	applyGateExpiresAt := ""
	applyGateReason := "no AutoTune strategy recommendation is available"
	clearBenchAutoTuneApplyPlan()
	if ok && recommendationAvailable && strategyNeeded {
		for _, profile := range templates {
			if profile.Index != profileIndex {
				continue
			}
			plan, planErr := storeBenchAutoTuneApplyPlan(status.ConfigSHA256, serverName, destinationIPv4, profile)
			if planErr != nil {
				ok = false
				applyGateReason = "create AutoTune apply gate: " + planErr.Error()
				break
			}
			applyGateEligible = true
			applyGateToken = plan.Token
			applyGateExpiresAt = plan.ExpiresAt.Format(time.RFC3339)
			applyGateReason = "fresh server-side recommendation is eligible for preview"
			break
		}
		if !applyGateEligible && ok {
			ok = false
			applyGateReason = "recommended profile is not present in the tested template set"
		}
	}

	response := benchAutoTuneResponse{
		OK:                      ok,
		Mode:                    mode,
		ServerName:              serverName,
		DestinationIPv4:         destinationIPv4,
		MetricScope:             "tls-reachability",
		Baseline:                baseline,
		Candidates:              candidates,
		RecommendationAvailable: recommendationAvailable,
		RecommendedProfileIndex: profileIndex,
		StrategyNeeded:          strategyNeeded,
		RecommendationReason:    reason,
		CleanupBaselineAfter:    after.CleanupBaselineProven,
		BenchEnabled:            false,
		SafeToBench:             false,
		ApplyEnabled:            false,
		ApplyGateEligible:       applyGateEligible,
		ApplyGateToken:          applyGateToken,
		ApplyGateExpiresAt:      applyGateExpiresAt,
		ApplyGateReason:         applyGateReason,
	}
	if !ok {
		writeJSON(w, http.StatusBadGateway, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}
