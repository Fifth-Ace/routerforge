package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const v2ProgressiveSelectorConfirm = "ROUTERFORGE_V2_PROGRESSIVE_SELECTOR"

const (
	v2ProgressiveStageBaseline   = "BASELINE"
	v2ProgressiveStageMemory     = "MEMORY"
	v2ProgressiveStageProduction = "PRODUCTION"
	v2ProgressiveStageQuick      = "QUICK"
	v2ProgressiveStageFull       = "FULL"
	v2ProgressiveStageLibrary    = "LIBRARY"
	v2ProgressiveStageVerifyTop  = "VERIFY_TOP"
	v2ProgressiveStageRank       = "RANK"
	v2ProgressiveStageDone       = "DONE"
)

var v2ProgressiveStageOrder = []string{
	v2ProgressiveStageMemory,
	v2ProgressiveStageProduction,
	v2ProgressiveStageQuick,
	v2ProgressiveStageFull,
	v2ProgressiveStageLibrary,
}

type v2ProgressiveSelectorRequest struct {
	Mode                 string                     `json:"mode"`
	ServerName           string                     `json:"server_name"`
	Transport            string                     `json:"transport,omitempty"`
	ExpectedConfigSHA256 string                     `json:"expected_config_sha256"`
	Concurrency          int                        `json:"concurrency,omitempty"`
	SessionID            string                     `json:"session_id,omitempty"`
	IncludeProduction    *bool                      `json:"include_production,omitempty"`
	Candidates           []v2SelectorCandidateInput `json:"candidates,omitempty"`
	PropertyVector       *v2DPIPropertyVector       `json:"property_vector,omitempty"`
	Confirm              string                     `json:"confirm"`
}

type v2ProgressiveStageSummary struct {
	Stage      string `json:"stage"`
	Planned    int    `json:"planned"`
	Completed  int    `json:"completed"`
	Working    int    `json:"working"`
	Verified   bool   `json:"verified"`
	Skipped    bool   `json:"skipped"`
	StopReason string `json:"stop_reason,omitempty"`
}

type v2ProgressiveSelectorResponse struct {
	OK                         bool                        `json:"ok"`
	SessionID                  string                      `json:"session_id"`
	Mode                       benchAutoTuneMode           `json:"mode"`
	ServerName                 string                      `json:"server_name"`
	DestinationIPv4            string                      `json:"destination_ipv4"`
	Transport                  string                      `json:"transport"`
	Network                    string                      `json:"network"`
	RemotePort                 int                         `json:"remote_port"`
	MetricScope                string                      `json:"metric_scope"`
	Baseline                   v2CandidateResult           `json:"baseline"`
	Candidates                 []v2CandidateResult         `json:"candidates"`
	Stages                     []v2ProgressiveStageSummary `json:"stages"`
	EarlyStop                  bool                        `json:"early_stop"`
	StopStage                  string                      `json:"stop_stage,omitempty"`
	RecommendationAvailable    bool                        `json:"recommendation_available"`
	RecommendedProfileIndex    int                         `json:"recommended_profile_index"`
	RecommendedCandidateID     string                      `json:"recommended_candidate_id,omitempty"`
	RecommendedCandidateName   string                      `json:"recommended_candidate_name,omitempty"`
	RecommendedCandidateSource string                      `json:"recommended_candidate_source,omitempty"`
	StrategyNeeded             bool                        `json:"strategy_needed"`
	RecommendationReason       string                      `json:"recommendation_reason"`
	CleanupBaselineAfter       bool                        `json:"cleanup_baseline_after"`
	BenchEnabled               bool                        `json:"bench_enabled"`
	SafeToBench                bool                        `json:"safe_to_bench"`
	ApplyEnabled               bool                        `json:"apply_enabled"`
	ApplyGateEligible          bool                        `json:"apply_gate_eligible"`
	ApplyGateReason            string                      `json:"apply_gate_reason"`
	Concurrency                int                         `json:"concurrency"`
	VerifyAttempts             int                         `json:"verify_attempts"`
	Warnings                   []string                    `json:"warnings"`
	PropertyVector             *v2DPIPropertyVector        `json:"property_vector,omitempty"`
	MemoryUpdated              bool                        `json:"memory_updated"`
	MemoryWarning              string                      `json:"memory_warning,omitempty"`
}

type v2ProgressiveTemplate struct {
	Stage       string
	Profile     benchStrategyProfile
	ID          string
	Name        string
	Source      string
	Production  bool
	Fingerprint string
}

type v2ProgressivePlan struct {
	Stages        map[string][]v2ProgressiveTemplate
	Budgets       map[string]int
	Warnings      []string
	ByFingerprint map[string]v2ProgressiveTemplate
}

func registerProgressiveSelectorV2Route(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/selector-progressive", mutationOnly(handleV2ProgressiveSelector))
}

func validateV2ProgressiveSelectorRequest(req v2ProgressiveSelectorRequest) error {
	if _, err := v2SelectorMode(req.Mode); err != nil {
		return err
	}
	target, err := v2NormalizeTarget(req.ServerName)
	if err != nil {
		return err
	}
	transport, err := normalizeBenchTransport(req.Transport)
	if err != nil {
		return err
	}
	if req.Confirm != v2ProgressiveSelectorConfirm {
		return errors.New("confirm must equal " + v2ProgressiveSelectorConfirm)
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(req.ExpectedConfigSHA256)) {
		return errors.New("expected_config_sha256 must be SHA256")
	}
	if req.SessionID != "" && !v2SelectorSessionValid(req.SessionID) {
		return errors.New("invalid selector session_id")
	}
	if req.Concurrency < 0 || req.Concurrency > v2MaxConcurrency {
		return fmt.Errorf("concurrency must be in range 0-%d", v2MaxConcurrency)
	}
	if _, err := v2NormalizeDPIPropertyVector(req.PropertyVector, transport.ID); err != nil {
		return err
	}
	if len(req.Candidates) > 32 {
		return errors.New("progressive selector candidates exceed limit")
	}
	for i, candidate := range req.Candidates {
		if len(candidate.Args) == 0 || len(candidate.Args) > v2StrategyArgsMax {
			return errors.New("progressive selector candidate args are empty or exceed limit")
		}
		if candidate.ID != "" && !v2StrategySafeID(candidate.ID) {
			return errors.New("progressive selector candidate id is invalid")
		}
		if candidate.Name != "" && !v2StrategySafeName(candidate.Name) {
			return errors.New("progressive selector candidate name is invalid")
		}
		if _, compileErr := v2CustomProfileForTransport(candidate.Args, target, transport); compileErr != nil {
			return fmt.Errorf("progressive selector candidate %d is not eligible for %s: %w", i, transport.ID, compileErr)
		}
	}
	return nil
}

func v2ProgressiveVerifyAttempts(mode benchAutoTuneMode) int {
	if mode.Name == "thorough" {
		return 3
	}
	return 2
}

func v2ProgressiveQuickLimit(mode benchAutoTuneMode) int {
	switch mode.Name {
	case "fast":
		return 2
	case "thorough":
		return 4
	default:
		return 3
	}
}

func v2ProgressiveStageBudgets(mode benchAutoTuneMode) map[string]int {
	budgets := map[string]int{
		v2ProgressiveStageMemory:     2,
		v2ProgressiveStageProduction: 3,
		v2ProgressiveStageQuick:      3,
		v2ProgressiveStageFull:       5,
		v2ProgressiveStageLibrary:    3,
	}
	switch mode.Name {
	case "fast":
		budgets[v2ProgressiveStageMemory] = 1
		budgets[v2ProgressiveStageProduction] = 1
		budgets[v2ProgressiveStageQuick] = 2
		budgets[v2ProgressiveStageFull] = 2
		budgets[v2ProgressiveStageLibrary] = 2
	case "thorough":
		budgets[v2ProgressiveStageMemory] = 4
		budgets[v2ProgressiveStageProduction] = 6
		budgets[v2ProgressiveStageQuick] = 4
		budgets[v2ProgressiveStageFull] = 8
		budgets[v2ProgressiveStageLibrary] = 10
	}
	return budgets
}

func v2ProgressiveMemoryEnvironmentFingerprint(configSHA, protocol string) string {
	return v2MemoryEnvironmentFingerprintForProtocol(configSHA, protocol)
}

func v2ProgressiveBuiltins(transport benchTransportProfile) []v2BuiltinCandidate {
	return v2BuiltinCandidatesForTransport(transport)
}

func v2ProgressiveAppendTemplate(plan *v2ProgressivePlan, seen map[string]bool, max int, item v2ProgressiveTemplate) bool {
	if plan == nil || len(item.Profile.Args) == 0 || item.Stage == "" {
		return false
	}
	fingerprint := v2CandidateTechniqueFingerprint(item.Profile.Args)
	if fingerprint == "" || seen[fingerprint] {
		return false
	}
	count := 0
	for _, stage := range v2ProgressiveStageOrder {
		count += len(plan.Stages[stage])
	}
	if count >= max {
		return false
	}
	if budget, ok := plan.Budgets[item.Stage]; ok && budget >= 0 && len(plan.Stages[item.Stage]) >= budget {
		return false
	}
	seen[fingerprint] = true
	item.Fingerprint = fingerprint
	plan.Stages[item.Stage] = append(plan.Stages[item.Stage], item)
	plan.ByFingerprint[fingerprint] = item
	return true
}

func v2ProgressiveMemoryTemplates(target, configSHA string, transport benchTransportProfile, limit int) ([]v2ProgressiveTemplate, error) {
	items, err := v2TargetMemoryCandidatesForTransport(target, configSHA, transport.ID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]v2ProgressiveTemplate, 0, len(items))
	for _, item := range items {
		profile, compileErr := v2CustomProfileForTransport(item.Args, target, transport)
		if compileErr != nil {
			continue
		}
		profile.Index = -1
		out = append(out, v2ProgressiveTemplate{
			Stage: v2ProgressiveStageMemory, Profile: profile,
			ID: item.ID, Name: item.Name, Source: "memory",
		})
	}
	return out, nil
}

func v2BuildProgressivePlan(req v2ProgressiveSelectorRequest, mode benchAutoTuneMode, transport benchTransportProfile, target, configSHA string, inventory benchStrategyInventory) v2ProgressivePlan {
	plan := v2ProgressivePlan{
		Stages: map[string][]v2ProgressiveTemplate{}, Budgets: v2ProgressiveStageBudgets(mode), Warnings: []string{},
		ByFingerprint: map[string]v2ProgressiveTemplate{},
	}
	for _, stage := range v2ProgressiveStageOrder {
		plan.Stages[stage] = []v2ProgressiveTemplate{}
	}
	seen := map[string]bool{}
	max := mode.MaxCandidates

	memory, err := v2ProgressiveMemoryTemplates(target, configSHA, transport, plan.Budgets[v2ProgressiveStageMemory])
	if err != nil {
		plan.Warnings = append(plan.Warnings, "memory: "+err.Error())
	} else {
		for _, item := range memory {
			v2ProgressiveAppendTemplate(&plan, seen, max, item)
		}
	}

	includeProduction := true
	if req.IncludeProduction != nil {
		includeProduction = *req.IncludeProduction
	}
	if includeProduction {
		for _, raw := range inventory.Profiles {
			profile := prepareBenchStrategyProfileForTransport(raw, transport)
			if !profile.CandidateEligible {
				continue
			}
			retargeted, retargetErr := retargetBenchStrategyProfileForTransport(profile, target, transport)
			if retargetErr != nil {
				continue
			}
			v2ProgressiveAppendTemplate(&plan, seen, max, v2ProgressiveTemplate{
				Stage: v2ProgressiveStageProduction, Profile: retargeted,
				ID:     fmt.Sprintf("production-%d", raw.Index),
				Name:   fmt.Sprintf("Production profile %d", raw.Index),
				Source: "production", Production: true,
			})
		}
	}

	// Dynamic synthesis owns QUICK/FULL first. Builtins remain a deterministic
	// fallback when synthesis cannot fill a stage or produces duplicates.
	synthesized, synthMeta := v2SynthesizeCandidates(target, transport, mode, v2PlannerHint{Properties: req.PropertyVector}, v2SynthesisBudget(mode, max))
	for _, item := range synthesized {
		profile, compileErr := v2CustomProfileForTransport(item.Args, target, transport)
		if compileErr != nil {
			plan.Warnings = append(plan.Warnings, item.ID+": "+compileErr.Error())
			continue
		}
		profile.Index = -1
		v2ProgressiveAppendTemplate(&plan, seen, max, v2ProgressiveTemplate{
			Stage: item.Stage, Profile: profile, ID: item.ID, Name: item.Name, Source: "synthesized",
		})
	}
	if synthMeta.Generated > 0 && len(synthesized) == 0 {
		plan.Warnings = append(plan.Warnings, "strategy synthesizer generated no compilable candidates")
	}

	builtins := v2ProgressiveBuiltins(transport)
	quickLimit := v2ProgressiveQuickLimit(mode)
	if quickLimit > len(builtins) {
		quickLimit = len(builtins)
	}
	for i, item := range builtins {
		profile, compileErr := v2CustomProfileForTransport(item.Args, target, transport)
		if compileErr != nil {
			plan.Warnings = append(plan.Warnings, item.ID+": "+compileErr.Error())
			continue
		}
		profile.Index = -1
		stage := v2ProgressiveStageFull
		if i < quickLimit {
			stage = v2ProgressiveStageQuick
		}
		v2ProgressiveAppendTemplate(&plan, seen, max, v2ProgressiveTemplate{
			Stage: stage, Profile: profile, ID: item.ID, Name: item.Name, Source: "builtin",
		})
	}

	for i, candidate := range req.Candidates {
		profile, compileErr := v2CustomProfileForTransport(candidate.Args, target, transport)
		if compileErr != nil {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("external candidate %d: %v", i, compileErr))
			continue
		}
		profile.Index = -1
		fp := v2CandidateTechniqueFingerprint(profile.Args)
		id := strings.TrimSpace(candidate.ID)
		if id == "" {
			id = "external-" + fp[:16]
		}
		name := strings.TrimSpace(candidate.Name)
		if name == "" {
			name = fmt.Sprintf("External candidate %d", i+1)
		}
		v2ProgressiveAppendTemplate(&plan, seen, max, v2ProgressiveTemplate{
			Stage: v2ProgressiveStageLibrary, Profile: profile,
			ID: id, Name: name, Source: v2StrategySource(candidate.Source),
		})
	}

	if doc, libraryErr := readV2StrategyLibrary(); libraryErr != nil {
		plan.Warnings = append(plan.Warnings, "library: "+libraryErr.Error())
	} else {
		for _, item := range doc.Strategies {
			profile, compileErr := v2CustomProfileForTransport(item.Args, target, transport)
			if compileErr != nil {
				continue
			}
			profile.Index = -1
			v2ProgressiveAppendTemplate(&plan, seen, max, v2ProgressiveTemplate{
				Stage: v2ProgressiveStageLibrary, Profile: profile,
				ID: item.ID, Name: item.Name, Source: item.Source,
			})
		}
	}
	return plan
}

func v2ProgressiveCandidateFromTemplate(item v2ProgressiveTemplate) v2CandidateResult {
	return v2CandidateResult{
		CandidateID: item.ID, CandidateName: item.Name, CandidateSource: item.Source,
		SourceProfileIndex: item.Profile.Index,
		StrategyTags:       append([]int{}, item.Profile.StrategyTags...),
		Args:               append([]string{}, item.Profile.Args...), CleanupProven: true,
	}
}

func v2ProgressiveRunStage(ctx context.Context, capabilities benchCapabilities, configSHA, target, ip string, inventory benchStrategyInventory, transport benchTransportProfile, items []v2ProgressiveTemplate, queues []int) []v2CandidateResult {
	if len(items) == 0 || len(queues) == 0 {
		return []v2CandidateResult{}
	}
	type job struct {
		index int
		item  v2ProgressiveTemplate
	}
	type jobResult struct {
		index  int
		result v2CandidateResult
	}
	jobs := make(chan job)
	results := make(chan jobResult, len(items))
	workers := len(queues)
	if workers > len(items) {
		workers = len(items)
	}
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		queue := queues[worker]
		wg.Add(1)
		go func(q int) {
			defer wg.Done()
			for j := range jobs {
				candidate := v2ProgressiveCandidateFromTemplate(j.item)
				attempt := v2RunTransportAttempt(ctx, capabilities, configSHA, target, ip, inventory, &j.item.Profile, q, transport)
				candidate.Attempts = append(candidate.Attempts, attempt)
				v2FinalizeCandidate(&candidate)
				results <- jobResult{index: j.index, result: candidate}
			}
		}(queue)
	}
	go func() {
		for i, item := range items {
			jobs <- job{index: i, item: item}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	out := make([]v2CandidateResult, len(items))
	for result := range results {
		out[result.index] = result.result
	}
	return out
}

func v2ProgressiveBestCandidate(candidates []v2CandidateResult) *v2CandidateResult {
	var best *v2CandidateResult
	for i := range candidates {
		candidate := &candidates[i]
		if !candidate.CleanupProven || !candidate.InfrastructureOK || candidate.Successes == 0 || candidate.ResultClass == "PARTIAL" {
			continue
		}
		if best == nil || v2CandidateBetter(*candidate, *best) {
			best = candidate
		}
	}
	return best
}

func v2ProgressiveStable(candidate v2CandidateResult, verifyAttempts int) bool {
	return candidate.CleanupProven && candidate.InfrastructureOK &&
		len(candidate.Attempts) >= verifyAttempts && candidate.ResultClass == "WORKING" && candidate.SuccessRate == 1
}

func v2ProgressiveVerifyTop(ctx context.Context, capabilities benchCapabilities, configSHA, target, ip string, inventory benchStrategyInventory, transport benchTransportProfile, candidate *v2CandidateResult, item v2ProgressiveTemplate, queue, verifyAttempts int) {
	for len(candidate.Attempts) < verifyAttempts {
		attempt := v2RunTransportAttempt(ctx, capabilities, configSHA, target, ip, inventory, &item.Profile, queue, transport)
		candidate.Attempts = append(candidate.Attempts, attempt)
		v2FinalizeCandidate(candidate)
		if !attempt.CleanupProven || !attempt.InfrastructureOK {
			break
		}
	}
	v2FinalizeCandidate(candidate)
}

func v2BuildProgressiveStageSummary(stage string, planned int, results []v2CandidateResult) v2ProgressiveStageSummary {
	summary := v2ProgressiveStageSummary{Stage: stage, Planned: planned, Completed: len(results)}
	for _, result := range results {
		if result.ResultClass == "WORKING" {
			summary.Working++
		}
	}
	if planned == 0 {
		summary.Skipped = true
	}
	return summary
}

func v2ProgressiveChooseRecommendation(baseline v2CandidateResult, candidates []v2CandidateResult, verifyAttempts int, transport benchTransportProfile) (bool, *v2CandidateResult, bool, string) {
	if v2ProgressiveStable(baseline, verifyAttempts) {
		return false, nil, false, "baseline is stable; bypass strategy is not required for " + transport.ID
	}
	var best *v2CandidateResult
	for i := range candidates {
		candidate := &candidates[i]
		if !v2ProgressiveStable(*candidate, verifyAttempts) {
			continue
		}
		if best == nil || v2CandidateBetter(*candidate, *best) {
			best = candidate
		}
	}
	if best == nil {
		return false, nil, false, "no candidate reached the progressive stability threshold for " + transport.ID
	}
	copyBest := *best
	return true, &copyBest, true, "candidate reached repeated protocol evidence and cleanup stability for " + transport.ID
}

func v2RecordProgressiveEvidence(target, configSHA string, transport benchTransportProfile, candidates []v2CandidateResult) error {
	doc, err := readV2TargetMemory()
	if err != nil {
		return err
	}
	target, err = v2NormalizeTarget(target)
	if err != nil {
		return err
	}
	now := v2TargetMemoryNow().UTC().Format(time.RFC3339)
	env := v2ProgressiveMemoryEnvironmentFingerprint(configSHA, transport.ID)
	index := map[string]int{}
	for i := range doc.Entries {
		index[v2MemoryKey(doc.Entries[i].Target, doc.Entries[i].Protocol, doc.Entries[i].Fingerprint)] = i
	}
	changed := false
	for _, candidate := range candidates {
		if !candidate.InfrastructureOK || !candidate.CleanupProven || len(candidate.Args) == 0 {
			continue
		}
		args := v2PortableCandidateArgs(candidate.Args)
		if len(args) == 0 {
			continue
		}
		fingerprint := v2CandidateTechniqueFingerprint(args)
		if fingerprint == "" {
			continue
		}
		key := v2MemoryKey(target, transport.ID, fingerprint)
		entry := v2TargetMemoryEntry{}
		if i, ok := index[key]; ok {
			entry = doc.Entries[i]
		}
		entry.Target = target
		entry.Protocol = transport.ID
		entry.IPFamily = "ipv4"
		entry.Fingerprint = fingerprint
		entry = v2MergeTargetMemoryObservation(entry, candidate, env, configSHA, now)
		if i, ok := index[key]; ok {
			doc.Entries[i] = entry
		} else {
			index[key] = len(doc.Entries)
			doc.Entries = append(doc.Entries, entry)
		}
		changed = true
	}
	if !changed {
		return nil
	}
	sort.SliceStable(doc.Entries, func(i, j int) bool {
		return doc.Entries[i].LastVerified > doc.Entries[j].LastVerified
	})
	if len(doc.Entries) > v2TargetMemoryMax {
		doc.Entries = doc.Entries[:v2TargetMemoryMax]
	}
	return writeV2TargetMemory(doc)
}

func handleV2ProgressiveSelector(w http.ResponseWriter, r *http.Request) {
	var req v2ProgressiveSelectorRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid progressive selector request"})
		return
	}
	if err := validateV2ProgressiveSelectorRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		generated, err := newBenchSessionID()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create progressive selector session: " + err.Error()})
			return
		}
		sessionID = generated
	}
	setV2SelectorProgress(sessionID, "PLAN", 0, 0, "building progressive candidate plan", false, false)

	if !atomic.CompareAndSwapInt32(&benchSmokeActive, 0, 1) {
		setV2SelectorProgress(sessionID, "FAILED", 0, 0, "another bench session is active", true, true)
		writeJSON(w, http.StatusConflict, map[string]any{"error": "another bench session is active", "session_id": sessionID})
		return
	}
	defer atomic.StoreInt32(&benchSmokeActive, 0)

	// Progressive selection is recommendation-only in C3A. Invalidate any
	// older legacy selector receipt so this route can never leave a stale
	// production Apply gate armed.
	clearBenchAutoTuneApplyPlan()

	mode, _ := v2SelectorMode(req.Mode)
	transport, _ := normalizeBenchTransport(req.Transport)
	target, _ := v2NormalizeTarget(req.ServerName)
	propertyVector, _ := v2NormalizeDPIPropertyVector(req.PropertyVector, transport.ID)
	req.PropertyVector = propertyVector
	verifyAttempts := v2ProgressiveVerifyAttempts(mode)
	status := readStatus()
	if !strings.EqualFold(status.ConfigSHA256, strings.TrimSpace(req.ExpectedConfigSHA256)) {
		setV2SelectorProgress(sessionID, "FAILED", 0, 0, "production config changed before progressive selector", true, true)
		writeJSON(w, http.StatusConflict, map[string]any{"error": "production config changed before progressive selector", "current_sha256": status.ConfigSHA256, "session_id": sessionID})
		return
	}
	capabilities := readBenchCapabilities()
	if !benchExecutionReady(capabilities) {
		setV2SelectorProgress(sessionID, "FAILED", 0, 0, "progressive selector capability gates are not proven", true, true)
		writeJSON(w, http.StatusConflict, map[string]any{"error": "progressive selector capability gates are not proven", "session_id": sessionID})
		return
	}
	inventory := readBenchStrategyInventory()
	if !inventory.BaseDependenciesProven {
		setV2SelectorProgress(sessionID, "FAILED", 0, 0, "strategy base dependencies are not proven", true, true)
		writeJSON(w, http.StatusConflict, map[string]any{"error": "strategy base dependencies are not proven", "session_id": sessionID})
		return
	}

	resolveCtx, cancelResolve := context.WithTimeout(r.Context(), 4*time.Second)
	ip, err := resolveBenchServerIPv4(resolveCtx, target)
	cancelResolve()
	if err != nil {
		setV2SelectorProgress(sessionID, "FAILED", 0, 0, "resolve target failed", true, true)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "resolve target: " + err.Error(), "session_id": sessionID})
		return
	}

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = v2DefaultConcurrency()
	}
	if concurrency > v2MaxConcurrency {
		concurrency = v2MaxConcurrency
	}
	queues := v2FreeBenchQueues(capabilities.OccupiedQueues, concurrency)
	if len(queues) < 1 {
		setV2SelectorProgress(sessionID, "FAILED", 0, 0, "no free reserved NFQUEUE", true, true)
		writeJSON(w, http.StatusConflict, map[string]any{"error": "no free reserved NFQUEUE for progressive selector", "session_id": sessionID})
		return
	}
	concurrency = len(queues)

	plan := v2BuildProgressivePlan(req, mode, transport, target, status.ConfigSHA256, inventory)
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(mode.TimeoutSec)*time.Second)
	defer cancel()

	stageHistory := []v2ProgressiveStageSummary{}
	allCandidates := []v2CandidateResult{}
	baseline := v2CandidateResult{
		Baseline: true, CandidateID: "baseline", CandidateName: "Baseline",
		CandidateSource: "baseline", SourceProfileIndex: -1, CleanupProven: true,
	}
	setV2SelectorProgress(sessionID, v2ProgressiveStageBaseline, 0, verifyAttempts, "proving baseline stability", false, false)
	for i := 0; i < verifyAttempts; i++ {
		attempt := v2RunTransportAttempt(ctx, capabilities, status.ConfigSHA256, target, ip, inventory, nil, queues[0], transport)
		baseline.Attempts = append(baseline.Attempts, attempt)
		v2FinalizeCandidate(&baseline)
		setV2SelectorProgress(sessionID, v2ProgressiveStageBaseline, i+1, verifyAttempts, "proving baseline stability", false, false)
		if !attempt.CleanupProven || !attempt.InfrastructureOK || !attempt.OK {
			break
		}
	}
	v2FinalizeCandidate(&baseline)
	baselineSummary := v2ProgressiveStageSummary{Stage: v2ProgressiveStageBaseline, Planned: verifyAttempts, Completed: len(baseline.Attempts)}
	if baseline.ResultClass == "WORKING" {
		baselineSummary.Working = 1
	}
	if !baseline.CleanupProven || !baseline.InfrastructureOK {
		baselineSummary.StopReason = "baseline infrastructure/cleanup proof failed"
		stageHistory = append(stageHistory, baselineSummary)
		setV2SelectorProgress(sessionID, "FAILED", len(baseline.Attempts), verifyAttempts, baselineSummary.StopReason, true, true)
		afterFailure := readBenchCapabilities()
		writeJSON(w, http.StatusBadGateway, v2ProgressiveSelectorResponse{
			OK: false, SessionID: sessionID, Mode: mode, ServerName: target, DestinationIPv4: ip,
			Transport: transport.ID, Network: transport.Network, RemotePort: transport.RemotePort, MetricScope: transport.MetricScope,
			Baseline: baseline, Candidates: allCandidates, Stages: stageHistory,
			RecommendationReason: baselineSummary.StopReason,
			CleanupBaselineAfter: afterFailure.CleanupBaselineProven, BenchEnabled: afterFailure.BenchEnabled, SafeToBench: afterFailure.SafeToBench,
			ApplyEnabled: false, ApplyGateEligible: false, ApplyGateReason: "C3A progressive selection is recommendation-only",
			Concurrency: concurrency, VerifyAttempts: verifyAttempts, Warnings: plan.Warnings,
		})
		return
	}
	if v2ProgressiveStable(baseline, verifyAttempts) {
		baselineSummary.Verified = true
		baselineSummary.StopReason = "baseline reached repeated protocol evidence; bypass strategy is not required"
		stageHistory = append(stageHistory, baselineSummary)
		after := readBenchCapabilities()
		ok := after.CleanupBaselineProven && strings.EqualFold(readStatus().ConfigSHA256, status.ConfigSHA256)
		response := v2ProgressiveSelectorResponse{
			OK: ok, SessionID: sessionID, Mode: mode, ServerName: target, DestinationIPv4: ip,
			Transport: transport.ID, Network: transport.Network, RemotePort: transport.RemotePort, MetricScope: transport.MetricScope,
			Baseline: baseline, Candidates: allCandidates, Stages: stageHistory,
			EarlyStop: true, StopStage: v2ProgressiveStageBaseline,
			RecommendationAvailable: false, RecommendedProfileIndex: -1, StrategyNeeded: false,
			RecommendationReason: baselineSummary.StopReason,
			CleanupBaselineAfter: after.CleanupBaselineProven, BenchEnabled: after.BenchEnabled, SafeToBench: after.SafeToBench,
			ApplyEnabled: false, ApplyGateEligible: false, ApplyGateReason: "baseline is stable; no apply plan is needed",
			Concurrency: concurrency, VerifyAttempts: verifyAttempts, Warnings: plan.Warnings,
		}
		if !ok {
			setV2SelectorProgress(sessionID, "FAILED", len(baseline.Attempts), verifyAttempts, "final cleanup/config proof failed", true, true)
			writeJSON(w, http.StatusBadGateway, response)
			return
		}
		setV2SelectorProgress(sessionID, v2ProgressiveStageDone, len(baseline.Attempts), verifyAttempts, baselineSummary.StopReason, true, false)
		writeJSON(w, http.StatusOK, response)
		return
	}
	stageHistory = append(stageHistory, baselineSummary)

	earlyStop := false
	stopStage := ""
	stopReason := ""
	for _, stage := range v2ProgressiveStageOrder {
		items := plan.Stages[stage]
		if len(items) == 0 {
			stageHistory = append(stageHistory, v2ProgressiveStageSummary{Stage: stage, Planned: 0, Completed: 0, Skipped: true})
			continue
		}
		setV2SelectorProgress(sessionID, stage, 0, len(items), "testing progressive stage "+strings.ToLower(stage), false, false)
		results := v2ProgressiveRunStage(ctx, capabilities, status.ConfigSHA256, target, ip, inventory, transport, items, queues)
		for i := range results {
			allCandidates = append(allCandidates, results[i])
			setV2SelectorProgress(sessionID, stage, i+1, len(items), "testing progressive stage "+strings.ToLower(stage), false, false)
		}
		summary := v2BuildProgressiveStageSummary(stage, len(items), results)
		for _, result := range results {
			if !result.CleanupProven || !result.InfrastructureOK {
				summary.StopReason = "candidate infrastructure/cleanup proof failed; progressive selector stopped fail-closed"
				stageHistory = append(stageHistory, summary)
				setV2SelectorProgress(sessionID, "FAILED", summary.Completed, summary.Planned, summary.StopReason, true, true)
				afterFailure := readBenchCapabilities()
				writeJSON(w, http.StatusBadGateway, v2ProgressiveSelectorResponse{
					OK: false, SessionID: sessionID, Mode: mode, ServerName: target, DestinationIPv4: ip,
					Transport: transport.ID, Network: transport.Network, RemotePort: transport.RemotePort, MetricScope: transport.MetricScope,
					Baseline: baseline, Candidates: allCandidates, Stages: stageHistory,
					RecommendationReason: summary.StopReason,
					CleanupBaselineAfter: afterFailure.CleanupBaselineProven, BenchEnabled: afterFailure.BenchEnabled, SafeToBench: afterFailure.SafeToBench,
					ApplyEnabled: false, ApplyGateEligible: false, ApplyGateReason: "C3A progressive selection is recommendation-only",
					Concurrency: concurrency, VerifyAttempts: verifyAttempts, Warnings: plan.Warnings,
				})
				return
			}
		}

		best := v2ProgressiveBestCandidate(allCandidates)
		if best != nil && best.ResultClass == "WORKING" && len(best.Attempts) < verifyAttempts {
			fingerprint := v2CandidateTechniqueFingerprint(best.Args)
			item, found := plan.ByFingerprint[fingerprint]
			if found {
				setV2SelectorProgress(sessionID, v2ProgressiveStageVerifyTop, len(best.Attempts), verifyAttempts, "repeating top candidate before early stop", false, false)
				v2ProgressiveVerifyTop(ctx, capabilities, status.ConfigSHA256, target, ip, inventory, transport, best, item, queues[0], verifyAttempts)
				setV2SelectorProgress(sessionID, v2ProgressiveStageVerifyTop, len(best.Attempts), verifyAttempts, "top candidate verification complete", false, false)
			}
		}
		if best != nil && v2ProgressiveStable(*best, verifyAttempts) {
			summary.Verified = true
			summary.StopReason = "top candidate reached repeated protocol evidence and cleanup stability"
			earlyStop = true
			stopStage = stage
			stopReason = summary.StopReason
			stageHistory = append(stageHistory, summary)
			break
		}
		stageHistory = append(stageHistory, summary)
	}

	setV2SelectorProgress(sessionID, v2ProgressiveStageRank, len(allCandidates), len(allCandidates), "ranking progressively verified candidates", false, false)
	recommend, best, needed, reason := v2ProgressiveChooseRecommendation(baseline, allCandidates, verifyAttempts, transport)
	if earlyStop && stopReason != "" {
		reason = stopReason
	}
	after := readBenchCapabilities()
	ok := after.CleanupBaselineProven && strings.EqualFold(readStatus().ConfigSHA256, status.ConfigSHA256)

	recommendedProfileIndex := -1
	recommendedID, recommendedName, recommendedSource := "", "", ""
	if best != nil {
		recommendedProfileIndex = best.SourceProfileIndex
		recommendedID = best.CandidateID
		recommendedName = best.CandidateName
		recommendedSource = best.CandidateSource
	}
	memoryUpdated := false
	memoryWarning := ""
	if ok {
		if memoryErr := v2RecordProgressiveEvidence(target, status.ConfigSHA256, transport, allCandidates); memoryErr != nil {
			memoryWarning = memoryErr.Error()
		} else if len(allCandidates) > 0 {
			memoryUpdated = true
		}
	}
	response := v2ProgressiveSelectorResponse{
		OK: ok, SessionID: sessionID, Mode: mode, ServerName: target, DestinationIPv4: ip,
		Transport: transport.ID, Network: transport.Network, RemotePort: transport.RemotePort, MetricScope: transport.MetricScope,
		Baseline: baseline, Candidates: allCandidates, Stages: stageHistory,
		EarlyStop: earlyStop, StopStage: stopStage,
		RecommendationAvailable: recommend, RecommendedProfileIndex: recommendedProfileIndex,
		RecommendedCandidateID: recommendedID, RecommendedCandidateName: recommendedName, RecommendedCandidateSource: recommendedSource,
		StrategyNeeded: needed, RecommendationReason: reason,
		CleanupBaselineAfter: after.CleanupBaselineProven, BenchEnabled: after.BenchEnabled, SafeToBench: after.SafeToBench,
		ApplyEnabled: false, ApplyGateEligible: false,
		ApplyGateReason: "C3A progressive selection is recommendation-only; existing Preview/Safe Apply gates remain separate",
		Concurrency:     concurrency, VerifyAttempts: verifyAttempts, Warnings: plan.Warnings,
		PropertyVector: req.PropertyVector,
		MemoryUpdated:  memoryUpdated, MemoryWarning: memoryWarning,
	}
	if !ok {
		setV2SelectorProgress(sessionID, "FAILED", len(allCandidates), len(allCandidates), "final cleanup/config proof failed", true, true)
		writeJSON(w, http.StatusBadGateway, response)
		return
	}
	setV2SelectorProgress(sessionID, v2ProgressiveStageDone, len(allCandidates), len(allCandidates), reason, true, false)
	writeJSON(w, http.StatusOK, response)
}
