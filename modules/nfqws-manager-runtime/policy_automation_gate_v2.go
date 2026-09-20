package main

import "net/http"

const (
	v2PolicyAutomationGateVersion = 1
	v2PolicyAutomationMinScore    = 80
	v2PolicyAutomationMinVerified = 3
	v2PolicyAutomationMinSuccess  = 0.90
)

type v2PolicyAutomationCandidate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Fingerprint string   `json:"fingerprint"`
	Eligible    bool     `json:"eligible"`
	Reasons     []string `json:"reasons"`
	Score       int      `json:"score"`
	Insight     string   `json:"insight"`
}

type v2PolicyAutomationGateResponse struct {
	OK                bool                          `json:"ok"`
	Version           int                           `json:"version"`
	ReadOnly          bool                          `json:"read_only"`
	AutomationEnabled bool                          `json:"automation_enabled"`
	ReadyCount        int                           `json:"ready_count"`
	Count             int                           `json:"count"`
	Policy            map[string]any                `json:"policy"`
	Candidates        []v2PolicyAutomationCandidate `json:"candidates"`
}

func registerPolicyAutomationGateV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/policy-automation-gate", getOnly(handleV2PolicyAutomationGate))
}

func v2PolicyAutomationEligibility(entry v2StrategyScoreEntry, insight v2StrategyInsight) (bool, []string) {
	reasons := []string{}
	e := entry.Evidence

	if !entry.Capabilities.CandidateReady {
		reasons = append(reasons, "candidate_not_ready")
	}
	if insight.State != "PROVEN" {
		reasons = append(reasons, "insight_not_proven")
	}
	if entry.Score.Total < v2PolicyAutomationMinScore {
		reasons = append(reasons, "score_below_threshold")
	}
	if e.VerifiedCount < v2PolicyAutomationMinVerified {
		reasons = append(reasons, "insufficient_verified_evidence")
	}
	if e.SuccessRate < v2PolicyAutomationMinSuccess {
		reasons = append(reasons, "success_rate_below_threshold")
	}
	if e.Confidence != v2MemoryConfidenceTrusted && e.Confidence != v2MemoryConfidenceStrong {
		reasons = append(reasons, "confidence_below_threshold")
	}
	if e.FailureCount > 0 {
		reasons = append(reasons, "historical_failures_present")
	}
	if e.UnstableCount > 0 {
		reasons = append(reasons, "historical_unstable_present")
	}
	if e.ReuseEligible <= 0 {
		reasons = append(reasons, "no_reuse_eligible_target")
	}
	return len(reasons) == 0, reasons
}

func v2BuildPolicyAutomationGate(scores v2StrategyScoreResponse, insights v2StrategyInsightsResponse) v2PolicyAutomationGateResponse {
	insightByFingerprint := map[string]v2StrategyInsight{}
	for _, item := range insights.Entries {
		insightByFingerprint[item.Fingerprint] = item.Insight
	}

	out := v2PolicyAutomationGateResponse{
		OK:                true,
		Version:           v2PolicyAutomationGateVersion,
		ReadOnly:          true,
		AutomationEnabled: false,
		Policy: map[string]any{
			"minimum_score":          v2PolicyAutomationMinScore,
			"minimum_verified":       v2PolicyAutomationMinVerified,
			"minimum_success_rate":   v2PolicyAutomationMinSuccess,
			"required_insight":       "PROVEN",
			"accepted_confidence":    []string{v2MemoryConfidenceTrusted, v2MemoryConfidenceStrong},
			"require_zero_failures":  true,
			"require_zero_unstable":  true,
			"require_reuse_eligible": true,
			"action":                 "none",
		},
		Candidates: []v2PolicyAutomationCandidate{},
	}

	for _, entry := range scores.Entries {
		insight := insightByFingerprint[entry.Fingerprint]
		eligible, reasons := v2PolicyAutomationEligibility(entry, insight)
		if eligible {
			out.ReadyCount++
		}
		out.Candidates = append(out.Candidates, v2PolicyAutomationCandidate{
			ID: entry.ID, Name: entry.Name, Fingerprint: entry.Fingerprint,
			Eligible: eligible, Reasons: reasons, Score: entry.Score.Total, Insight: insight.State,
		})
	}
	out.Count = len(out.Candidates)
	return out
}

func handleV2PolicyAutomationGate(w http.ResponseWriter, _ *http.Request) {
	library, err := readV2StrategyLibrary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read strategy library: " + err.Error()})
		return
	}
	memory, err := readV2TargetMemory()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read target memory: " + err.Error()})
		return
	}
	registry := v2BuildStrategyRegistry(library, memory, v2TargetMemoryNow().UTC())
	scores := v2BuildStrategyScores(registry)
	insights := v2BuildStrategyInsights(scores)
	writeJSON(w, http.StatusOK, v2BuildPolicyAutomationGate(scores, insights))
}
