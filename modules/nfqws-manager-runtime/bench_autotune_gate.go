package main

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

const benchAutoTuneApplyGateTTL = 5 * time.Minute

type benchAutoTuneApplyPlan struct {
	Token                 string
	CreatedAt             time.Time
	ExpiresAt             time.Time
	ConfigSHA256          string
	ServerName            string
	DestinationIPv4       string
	SourceProfileIndex    int
	AppendProfile         bool
	SourceStrategyArgs    []string
	CandidateStrategyArgs []string
	CandidateSource       string
	CandidateFingerprint  string
	Transport             string
	SessionID             string
	CandidateID           string
	CandidateName         string
	LiveResultClass       string
	LiveSuccessRate       float64
	LiveCompleteRate      float64
	LiveCleanupProven     bool
	LiveInfrastructureOK  bool
	// StrategyArgs is retained as a compatibility alias for older internal
	// callers/tests. It always mirrors CandidateStrategyArgs.
	StrategyArgs []string
}

type benchAutoTuneApplyGateStatus struct {
	Eligible           bool   `json:"eligible"`
	ServerName         string `json:"server_name,omitempty"`
	SourceProfileIndex int    `json:"source_profile_index"`
	ConfigSHA256       string `json:"config_sha256,omitempty"`
	ExpiresAt          string `json:"expires_at,omitempty"`
	Reason             string `json:"reason"`
}

var benchAutoTuneApplyGateState = struct {
	sync.Mutex
	plan *benchAutoTuneApplyPlan
}{}

func registerBenchAutoTuneApplyGateRoute(mux *http.ServeMux) {
	mux.HandleFunc("/v1/bench-autotune/apply-gate", getOnly(handleBenchAutoTuneApplyGateStatus))
	registerBenchAutoTuneApplyPreviewRoute(mux)
}

func clearBenchAutoTuneApplyPlan() {
	benchAutoTuneApplyGateState.Lock()
	benchAutoTuneApplyGateState.plan = nil
	benchAutoTuneApplyGateState.Unlock()
	clearBenchAutoTuneApplyReceipt()
}

func storeBenchAutoTuneApplyPlan(configSHA, serverName, destinationIPv4 string, profile benchStrategyProfile) (*benchAutoTuneApplyPlan, error) {
	inventory := readBenchStrategyInventory()
	for _, sourceProfile := range inventory.Profiles {
		if sourceProfile.Index != profile.Index {
			continue
		}
		return storeBenchAutoTuneApplyPlanForCandidate(
			configSHA, serverName, destinationIPv4, sourceProfile, profile.Args,
			"production", v2CandidateTechniqueFingerprint(profile.Args),
		)
	}
	return nil, errors.New("source production profile is no longer present")
}

func storeBenchAutoTuneApplyPlanForCandidate(
	configSHA, serverName, destinationIPv4 string,
	sourceProfile benchStrategyProfile,
	candidateArgs []string,
	candidateSource, candidateFingerprint string,
) (*benchAutoTuneApplyPlan, error) {
	if sourceProfile.Index < 0 || len(sourceProfile.Args) == 0 {
		return nil, errors.New("source production profile is required")
	}
	if len(candidateArgs) == 0 {
		return nil, errors.New("candidate strategy args are required")
	}
	if err := validatePreviewArgs(candidateArgs); err != nil {
		return nil, err
	}
	if strings.TrimSpace(candidateFingerprint) == "" {
		candidateFingerprint = v2CandidateTechniqueFingerprint(candidateArgs)
	}
	if candidateFingerprint == "" {
		return nil, errors.New("candidate technique fingerprint is required")
	}
	token, err := newBenchSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	candidateArgsCopy := append([]string{}, candidateArgs...)
	plan := &benchAutoTuneApplyPlan{
		Token:                 token,
		CreatedAt:             now,
		ExpiresAt:             now.Add(benchAutoTuneApplyGateTTL),
		ConfigSHA256:          strings.ToLower(strings.TrimSpace(configSHA)),
		ServerName:            serverName,
		DestinationIPv4:       destinationIPv4,
		SourceProfileIndex:    sourceProfile.Index,
		SourceStrategyArgs:    append([]string{}, sourceProfile.Args...),
		CandidateStrategyArgs: candidateArgsCopy,
		CandidateSource:       strings.ToLower(strings.TrimSpace(candidateSource)),
		CandidateFingerprint:  candidateFingerprint,
		StrategyArgs:          append([]string{}, candidateArgsCopy...),
	}
	benchAutoTuneApplyGateState.Lock()
	benchAutoTuneApplyGateState.plan = plan
	benchAutoTuneApplyGateState.Unlock()
	return plan, nil
}

func storeBenchAutoTuneAppendPlanForCandidate(
	configSHA, serverName, destinationIPv4 string,
	candidateArgs []string,
	candidateSource, candidateFingerprint string,
) (*benchAutoTuneApplyPlan, error) {
	if len(candidateArgs) == 0 {
		return nil, errors.New("candidate strategy args are required")
	}
	if err := validatePreviewArgs(candidateArgs); err != nil {
		return nil, err
	}
	if strings.TrimSpace(candidateFingerprint) == "" {
		candidateFingerprint = v2CandidateTechniqueFingerprint(candidateArgs)
	}
	if candidateFingerprint == "" {
		return nil, errors.New("candidate technique fingerprint is required")
	}
	token, err := newBenchSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	candidateArgsCopy := append([]string{}, candidateArgs...)
	plan := &benchAutoTuneApplyPlan{
		Token:                 token,
		CreatedAt:             now,
		ExpiresAt:             now.Add(benchAutoTuneApplyGateTTL),
		ConfigSHA256:          strings.ToLower(strings.TrimSpace(configSHA)),
		ServerName:            serverName,
		DestinationIPv4:       destinationIPv4,
		SourceProfileIndex:    -1,
		AppendProfile:         true,
		CandidateStrategyArgs: candidateArgsCopy,
		CandidateSource:       strings.ToLower(strings.TrimSpace(candidateSource)),
		CandidateFingerprint:  candidateFingerprint,
		StrategyArgs:          append([]string{}, candidateArgsCopy...),
	}
	benchAutoTuneApplyGateState.Lock()
	benchAutoTuneApplyGateState.plan = plan
	benchAutoTuneApplyGateState.Unlock()
	return plan, nil
}

func currentBenchAutoTuneApplyPlan() (*benchAutoTuneApplyPlan, error) {
	benchAutoTuneApplyGateState.Lock()
	defer benchAutoTuneApplyGateState.Unlock()

	plan := benchAutoTuneApplyGateState.plan
	if plan == nil {
		return nil, errors.New("no active AutoTune recommendation")
	}
	if time.Now().UTC().After(plan.ExpiresAt) {
		benchAutoTuneApplyGateState.plan = nil
		return nil, errors.New("AutoTune recommendation expired")
	}
	status := readStatus()
	if !strings.EqualFold(status.ConfigSHA256, plan.ConfigSHA256) {
		benchAutoTuneApplyGateState.plan = nil
		return nil, errors.New("active config changed since AutoTune recommendation")
	}
	copyPlan := *plan
	copyPlan.SourceStrategyArgs = append([]string{}, plan.SourceStrategyArgs...)
	copyPlan.CandidateStrategyArgs = append([]string{}, plan.CandidateStrategyArgs...)
	copyPlan.StrategyArgs = append([]string{}, plan.StrategyArgs...)
	return &copyPlan, nil
}

func handleBenchAutoTuneApplyGateStatus(w http.ResponseWriter, _ *http.Request) {
	plan, err := currentBenchAutoTuneApplyPlan()
	if err != nil {
		writeJSON(w, http.StatusOK, benchAutoTuneApplyGateStatus{
			Eligible: false,
			Reason:   err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, benchAutoTuneApplyGateStatus{
		Eligible:           true,
		ServerName:         plan.ServerName,
		SourceProfileIndex: plan.SourceProfileIndex,
		ConfigSHA256:       plan.ConfigSHA256,
		ExpiresAt:          plan.ExpiresAt.Format(time.RFC3339),
		Reason:             "fresh server-side AutoTune recommendation is available",
	})
}
