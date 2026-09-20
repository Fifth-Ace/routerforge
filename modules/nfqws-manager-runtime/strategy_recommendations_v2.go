package main

import (
	"net/http"
	"sort"
)

const v2StrategyRecommendationsVersion = 1

type v2StrategyRecommendation struct {
	Rank            int     `json:"rank"`
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Protocol        string  `json:"protocol"`
	Family          string  `json:"family"`
	Fingerprint     string  `json:"fingerprint"`
	Score           int     `json:"score"`
	Insight         string  `json:"insight"`
	VerifiedCount   int     `json:"verified_count"`
	SuccessRate     float64 `json:"success_rate"`
	AutomationReady bool    `json:"automation_ready"`
	Primary         bool    `json:"primary"`
	Reason          string  `json:"reason"`
}

type v2StrategyRecommendationsResponse struct {
	OK       bool                       `json:"ok"`
	Version  int                        `json:"version"`
	ReadOnly bool                       `json:"read_only"`
	Count    int                        `json:"count"`
	Primary  int                        `json:"primary_count"`
	Items    []v2StrategyRecommendation `json:"items"`
}

func registerStrategyRecommendationsV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/strategy-recommendations", getOnly(handleV2StrategyRecommendations))
}

func v2RecommendationInsightRank(state string) int {
	switch state {
	case "PROVEN":
		return 4
	case "PROMISING":
		return 3
	case "MIXED":
		return 2
	case "WEAK":
		return 1
	default:
		return 0
	}
}

func v2BuildStrategyRecommendations(
	scores v2StrategyScoreResponse,
	insights v2StrategyInsightsResponse,
	gate v2PolicyAutomationGateResponse,
) v2StrategyRecommendationsResponse {
	insightByFingerprint := map[string]v2StrategyInsight{}
	for _, item := range insights.Entries {
		insightByFingerprint[item.Fingerprint] = item.Insight
	}
	automationByFingerprint := map[string]bool{}
	for _, item := range gate.Candidates {
		automationByFingerprint[item.Fingerprint] = item.Eligible
	}

	items := make([]v2StrategyRecommendation, 0, len(scores.Entries))
	for _, entry := range scores.Entries {
		if !entry.Capabilities.CandidateReady || entry.Evidence.VerifiedCount <= 0 {
			continue
		}
		insight := insightByFingerprint[entry.Fingerprint]
		items = append(items, v2StrategyRecommendation{
			ID: entry.ID, Name: entry.Name, Protocol: entry.Protocol, Family: entry.Family,
			Fingerprint: entry.Fingerprint, Score: entry.Score.Total, Insight: insight.State,
			VerifiedCount: entry.Evidence.VerifiedCount, SuccessRate: entry.Evidence.SuccessRate,
			AutomationReady: automationByFingerprint[entry.Fingerprint],
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Protocol != items[j].Protocol {
			return items[i].Protocol < items[j].Protocol
		}
		ir, jr := v2RecommendationInsightRank(items[i].Insight), v2RecommendationInsightRank(items[j].Insight)
		if ir != jr {
			return ir > jr
		}
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		if items[i].VerifiedCount != items[j].VerifiedCount {
			return items[i].VerifiedCount > items[j].VerifiedCount
		}
		return items[i].Fingerprint < items[j].Fingerprint
	})

	rankByProtocol := map[string]int{}
	primarySeen := map[string]bool{}
	primaryCount := 0
	for i := range items {
		rankByProtocol[items[i].Protocol]++
		items[i].Rank = rankByProtocol[items[i].Protocol]
		switch items[i].Insight {
		case "PROVEN":
			items[i].Reason = "historical evidence is proven; live verification is still required before apply"
		case "PROMISING":
			items[i].Reason = "historical evidence is promising; live verification is required"
		case "MIXED":
			items[i].Reason = "historical evidence is mixed; treat as a bench hint only"
		default:
			items[i].Reason = "historical evidence is weak; keep as a low-priority bench hint"
		}
		if !primarySeen[items[i].Protocol] && (items[i].Insight == "PROVEN" || items[i].Insight == "PROMISING") {
			items[i].Primary = true
			primarySeen[items[i].Protocol] = true
			primaryCount++
		}
	}

	return v2StrategyRecommendationsResponse{
		OK: true, Version: v2StrategyRecommendationsVersion, ReadOnly: true,
		Count: len(items), Primary: primaryCount, Items: items,
	}
}

func handleV2StrategyRecommendations(w http.ResponseWriter, _ *http.Request) {
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
	gate := v2BuildPolicyAutomationGate(scores, insights)
	writeJSON(w, http.StatusOK, v2BuildStrategyRecommendations(scores, insights, gate))
}
