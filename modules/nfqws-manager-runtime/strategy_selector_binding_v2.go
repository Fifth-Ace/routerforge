package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const v2SelectorBindConfirm = "ROUTERFORGE_V2_SELECTOR_BIND"

type v2SelectorPendingBinding struct {
	ConfigSHA256    string
	ServerName      string
	DestinationIPv4 string
	SessionID       string
	Candidates      []v2CandidateResult
	ExpiresAt       time.Time
}

var v2SelectorPendingBindingState = struct {
	sync.Mutex
	item *v2SelectorPendingBinding
}{}

type v2SelectorBindRequest struct {
	SessionID            string `json:"session_id"`
	ExpectedConfigSHA256 string `json:"expected_config_sha256"`
	CandidateID          string `json:"candidate_id"`
	Confirm              string `json:"confirm"`
}

func clearV2SelectorPendingBinding() {
	v2SelectorPendingBindingState.Lock()
	v2SelectorPendingBindingState.item = nil
	v2SelectorPendingBindingState.Unlock()
}

func copyV2SelectorCandidate(in v2CandidateResult) v2CandidateResult {
	out := in
	out.Args = append([]string{}, in.Args...)
	out.Attempts = append([]v2BenchAttempt{}, in.Attempts...)
	out.StrategyTags = append([]int{}, in.StrategyTags...)
	return out
}

func storeV2SelectorPendingBinding(
	configSHA, serverName, destinationIPv4, sessionID string,
	candidates []v2CandidateResult,
) {
	if !v2SelectorSessionValid(sessionID) {
		return
	}
	working := make([]v2CandidateResult, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ResultClass != "WORKING" || candidate.Successes < 1 || !candidate.CleanupProven || !candidate.InfrastructureOK {
			continue
		}
		working = append(working, copyV2SelectorCandidate(candidate))
	}
	if len(working) == 0 {
		return
	}
	v2SelectorPendingBindingState.Lock()
	v2SelectorPendingBindingState.item = &v2SelectorPendingBinding{
		ConfigSHA256:    strings.ToLower(strings.TrimSpace(configSHA)),
		ServerName:      serverName,
		DestinationIPv4: destinationIPv4,
		SessionID:       sessionID,
		Candidates:      working,
		ExpiresAt:       time.Now().UTC().Add(benchAutoTuneApplyGateTTL),
	}
	v2SelectorPendingBindingState.Unlock()
}

func currentV2SelectorPendingBinding(sessionID, expectedConfigSHA string) (*v2SelectorPendingBinding, error) {
	v2SelectorPendingBindingState.Lock()
	item := v2SelectorPendingBindingState.item
	if item == nil {
		v2SelectorPendingBindingState.Unlock()
		return nil, errors.New("no selector result is waiting for apply preparation")
	}
	copyItem := *item
	copyItem.Candidates = make([]v2CandidateResult, len(item.Candidates))
	for i := range item.Candidates {
		copyItem.Candidates[i] = copyV2SelectorCandidate(item.Candidates[i])
	}
	v2SelectorPendingBindingState.Unlock()

	if time.Now().UTC().After(copyItem.ExpiresAt) {
		clearV2SelectorPendingBinding()
		return nil, errors.New("selector apply preparation window expired")
	}
	if strings.TrimSpace(sessionID) != copyItem.SessionID {
		return nil, errors.New("selector binding session mismatch")
	}
	if !strings.EqualFold(strings.TrimSpace(expectedConfigSHA), copyItem.ConfigSHA256) {
		return nil, errors.New("selector binding config identity mismatch")
	}
	if !strings.EqualFold(readStatus().ConfigSHA256, copyItem.ConfigSHA256) {
		clearV2SelectorPendingBinding()
		return nil, errors.New("active config changed since selector verification")
	}
	return &copyItem, nil
}

func v2SelectorPendingCandidate(item *v2SelectorPendingBinding, candidateID string) (*v2CandidateResult, error) {
	candidateID = strings.TrimSpace(candidateID)
	if candidateID == "" {
		return nil, errors.New("candidate_id is required")
	}
	for i := range item.Candidates {
		if item.Candidates[i].CandidateID == candidateID {
			candidate := copyV2SelectorCandidate(item.Candidates[i])
			return &candidate, nil
		}
	}
	return nil, errors.New("selected candidate is not one of the working selector results")
}

func v2ReverifySelectorCandidate(ctx context.Context, item *v2SelectorPendingBinding, candidate *v2CandidateResult) error {
	if err := v2GenericWinnerApplyEligible(candidate); err == nil {
		return nil
	}
	if candidate.ResultClass != "WORKING" || candidate.Successes < 1 || !candidate.CleanupProven || !candidate.InfrastructureOK {
		return errors.New("selected candidate is not eligible for focused verification")
	}

	capabilities := readBenchCapabilities()
	if !benchExecutionReady(capabilities) {
		return errors.New("selector verification capability gates are not proven")
	}
	inventory := readBenchStrategyInventory()
	profile, err := v2CustomProfile(v2PortableCandidateArgs(candidate.Args), item.ServerName)
	if err != nil {
		return errors.New("compile selected candidate for verification: " + err.Error())
	}
	queues := v2FreeBenchQueues(capabilities.OccupiedQueues, 1)
	if len(queues) != 1 {
		return errors.New("no free reserved NFQUEUE for selected candidate verification")
	}

	sandbox := newV2DirectSandbox(capabilities, inventory, 0, queues[0], item.ServerName, item.DestinationIPv4)
	if err := sandbox.RulesUp(true); err != nil {
		return errors.New("selected candidate verification rules: " + err.Error())
	}
	defer sandbox.RulesDown()

	verifyCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	candidate.Attempts = append(candidate.Attempts, sandbox.RunCandidate(verifyCtx, profile))
	v2FinalizeCandidate(candidate)
	if err := v2GenericWinnerApplyEligible(candidate); err != nil {
		return err
	}
	return nil
}

func handleV2SelectorBind(w http.ResponseWriter, r *http.Request) {
	var req v2SelectorBindRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid selector bind request"})
		return
	}
	if req.Confirm != v2SelectorBindConfirm {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal " + v2SelectorBindConfirm})
		return
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(req.ExpectedConfigSHA256)) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "expected_config_sha256 must be SHA256"})
		return
	}

	item, err := currentV2SelectorPendingBinding(req.SessionID, req.ExpectedConfigSHA256)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	candidate, err := v2SelectorPendingCandidate(item, req.CandidateID)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}

	if !atomic.CompareAndSwapInt32(&benchSmokeActive, 0, 1) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "another bench session is active"})
		return
	}
	defer atomic.StoreInt32(&benchSmokeActive, 0)

	if err := v2ReverifySelectorCandidate(r.Context(), item, candidate); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "selected strategy verification: " + err.Error()})
		return
	}
	plan, err := v2StoreGenericCandidateAppendPlan(
		item.ConfigSHA256, item.ServerName, item.DestinationIPv4,
		benchTransportHTTPS, item.SessionID, candidate,
	)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "prepare new selector profile: " + err.Error()})
		return
	}

	clearV2SelectorPendingBinding()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                     true,
		"apply_gate_eligible":    true,
		"apply_gate_token":       plan.Token,
		"apply_gate_expires_at":  plan.ExpiresAt.Format(time.RFC3339),
		"apply_gate_reason":      "selected strategy was re-verified and will be appended as a new NFQWS_ARGS_CUSTOM profile",
		"created_custom_profile": true,
		"candidate_id":           candidate.CandidateID,
		"verification_attempts":  len(candidate.Attempts),
		"verification_successes": candidate.Successes,
	})
}
