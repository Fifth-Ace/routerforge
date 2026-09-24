package main

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

const v2SelectorBindConfirm = "ROUTERFORGE_V2_SELECTOR_BIND"

type v2SelectorPendingBinding struct {
	ConfigSHA256           string
	ServerName             string
	DestinationIPv4        string
	SessionID              string
	Candidate              v2CandidateResult
	MatchingProfileIndexes []int
	ExpiresAt              time.Time
}

var v2SelectorPendingBindingState = struct {
	sync.Mutex
	item *v2SelectorPendingBinding
}{}

type v2SelectorBindRequest struct {
	SessionID            string `json:"session_id"`
	ExpectedConfigSHA256 string `json:"expected_config_sha256"`
	ProfileIndex         *int   `json:"profile_index,omitempty"`
	CreateCustom         bool   `json:"create_custom,omitempty"`
	Confirm              string `json:"confirm"`
}

func clearV2SelectorPendingBinding() {
	v2SelectorPendingBindingState.Lock()
	v2SelectorPendingBindingState.item = nil
	v2SelectorPendingBindingState.Unlock()
}

func storeV2SelectorPendingBinding(
	configSHA, serverName, destinationIPv4, sessionID string,
	best *v2CandidateResult, matches []benchStrategyProfile,
) {
	if best == nil || !v2SelectorSessionValid(sessionID) {
		return
	}
	copyCandidate := *best
	copyCandidate.Args = append([]string{}, best.Args...)
	copyCandidate.Attempts = append([]v2BenchAttempt{}, best.Attempts...)
	indexes := make([]int, 0, len(matches))
	for _, profile := range matches {
		indexes = append(indexes, profile.Index)
	}
	v2SelectorPendingBindingState.Lock()
	v2SelectorPendingBindingState.item = &v2SelectorPendingBinding{
		ConfigSHA256:           strings.ToLower(strings.TrimSpace(configSHA)),
		ServerName:             serverName,
		DestinationIPv4:        destinationIPv4,
		SessionID:              sessionID,
		Candidate:              copyCandidate,
		MatchingProfileIndexes: indexes,
		ExpiresAt:              time.Now().UTC().Add(benchAutoTuneApplyGateTTL),
	}
	v2SelectorPendingBindingState.Unlock()
}

func currentV2SelectorPendingBinding(sessionID, expectedConfigSHA string) (*v2SelectorPendingBinding, error) {
	v2SelectorPendingBindingState.Lock()
	item := v2SelectorPendingBindingState.item
	if item == nil {
		v2SelectorPendingBindingState.Unlock()
		return nil, errors.New("no selector winner is waiting for profile binding")
	}
	copyItem := *item
	copyItem.Candidate = item.Candidate
	copyItem.Candidate.Args = append([]string{}, item.Candidate.Args...)
	copyItem.Candidate.Attempts = append([]v2BenchAttempt{}, item.Candidate.Attempts...)
	copyItem.MatchingProfileIndexes = append([]int{}, item.MatchingProfileIndexes...)
	v2SelectorPendingBindingState.Unlock()

	if time.Now().UTC().After(copyItem.ExpiresAt) {
		clearV2SelectorPendingBinding()
		return nil, errors.New("selector winner binding window expired")
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

func v2SelectorBindingAllowsProfile(item *v2SelectorPendingBinding, index int) bool {
	for _, allowed := range item.MatchingProfileIndexes {
		if allowed == index {
			return true
		}
	}
	return false
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
	if req.CreateCustom == (req.ProfileIndex != nil) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "choose exactly one binding mode"})
		return
	}

	item, err := currentV2SelectorPendingBinding(req.SessionID, req.ExpectedConfigSHA256)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}

	var plan *benchAutoTuneApplyPlan
	reason := ""
	if req.CreateCustom {
		if len(item.MatchingProfileIndexes) != 0 {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "new custom profile is only available when no production profile matches the target"})
			return
		}
		plan, err = v2StoreGenericCandidateAppendPlan(
			item.ConfigSHA256, item.ServerName, item.DestinationIPv4,
			benchTransportHTTPS, item.SessionID, &item.Candidate,
		)
		reason = "live-verified candidate will be appended as a new custom profile and is eligible for deterministic preview"
	} else {
		index := *req.ProfileIndex
		if !v2SelectorBindingAllowsProfile(item, index) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "selected production profile is not one of the verified target matches"})
			return
		}
		inventory := readBenchStrategyInventory()
		var source *benchStrategyProfile
		for i := range inventory.Profiles {
			if inventory.Profiles[i].Index == index && v2ProductionSourceProfileEligible(inventory.Profiles[i]) {
				candidate := inventory.Profiles[i]
				source = &candidate
				break
			}
		}
		if source == nil {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "selected production profile is no longer eligible"})
			return
		}
		var boundArgs []string
		boundArgs, err = v2BindCandidateToSourceProfile(*source, item.Candidate.Args)
		if err == nil {
			plan, err = v2StoreGenericCandidateApplyPlan(
				item.ConfigSHA256, item.ServerName, item.DestinationIPv4,
				benchTransportHTTPS, item.SessionID, *source, boundArgs, &item.Candidate,
			)
		}
		reason = "live-verified candidate is bound to the selected production profile and eligible for deterministic preview"
	}
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "create selector apply binding: " + err.Error()})
		return
	}
	clearV2SelectorPendingBinding()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                     true,
		"apply_gate_eligible":    true,
		"apply_gate_token":       plan.Token,
		"apply_gate_expires_at":  plan.ExpiresAt.Format(time.RFC3339),
		"apply_gate_reason":      reason,
		"source_profile_index":   plan.SourceProfileIndex,
		"created_custom_profile": plan.AppendProfile,
	})
}
