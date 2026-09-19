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
	Token              string
	CreatedAt          time.Time
	ExpiresAt          time.Time
	ConfigSHA256       string
	ServerName         string
	DestinationIPv4    string
	SourceProfileIndex int
	StrategyArgs       []string
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
}

func clearBenchAutoTuneApplyPlan() {
	benchAutoTuneApplyGateState.Lock()
	defer benchAutoTuneApplyGateState.Unlock()
	benchAutoTuneApplyGateState.plan = nil
}

func storeBenchAutoTuneApplyPlan(configSHA, serverName, destinationIPv4 string, profile benchStrategyProfile) (*benchAutoTuneApplyPlan, error) {
	token, err := newBenchSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	plan := &benchAutoTuneApplyPlan{
		Token:              token,
		CreatedAt:          now,
		ExpiresAt:          now.Add(benchAutoTuneApplyGateTTL),
		ConfigSHA256:       strings.ToLower(strings.TrimSpace(configSHA)),
		ServerName:         serverName,
		DestinationIPv4:    destinationIPv4,
		SourceProfileIndex: profile.Index,
		StrategyArgs:       append([]string{}, profile.Args...),
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
