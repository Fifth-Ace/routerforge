package main

import (
	"net/http"
	"strconv"
	"strings"
)

const v2StrategyInsightsVersion = 1

type v2StrategyInsight struct {
	State   string   `json:"state"`
	Summary string   `json:"summary"`
	Signals []string `json:"signals"`
}

type v2StrategyInsightEntry struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Fingerprint string            `json:"fingerprint"`
	Score       v2StrategyScore   `json:"score"`
	Insight     v2StrategyInsight `json:"insight"`
}

type v2StrategyInsightsResponse struct {
	OK       bool                     `json:"ok"`
	Version  int                      `json:"version"`
	ReadOnly bool                     `json:"read_only"`
	Count    int                      `json:"count"`
	Entries  []v2StrategyInsightEntry `json:"entries"`
}

func registerStrategyInsightsV2Routes(mux *http.ServeMux) {
	registerPolicyAutomationGateV2Routes(mux)
	mux.HandleFunc("/v1/v2/strategy-insights", getOnly(handleV2StrategyInsights))
}

func v2StrategyInsightForEntry(entry v2StrategyScoreEntry) v2StrategyInsight {
	e := entry.Evidence
	signals := []string{
		"score=" + strconv.Itoa(entry.Score.Total),
		"verified=" + strconv.Itoa(e.VerifiedCount),
		"targets=" + strconv.Itoa(e.Targets),
	}
	if e.Confidence != "" {
		signals = append(signals, "confidence="+strings.ToLower(e.Confidence))
	}
	if e.FailureCount > 0 {
		signals = append(signals, "failures="+strconv.Itoa(e.FailureCount))
	}
	if e.UnstableCount > 0 {
		signals = append(signals, "unstable="+strconv.Itoa(e.UnstableCount))
	}

	switch {
	case !entry.Capabilities.CandidateReady:
		return v2StrategyInsight{
			State: "NOT_READY", Summary: "Стратегия не проходит текущий bench compiler contract.",
			Signals: signals,
		}
	case e.VerifiedCount == 0:
		return v2StrategyInsight{
			State: "UNVERIFIED", Summary: "Bench-capable, но verified evidence пока отсутствует.",
			Signals: signals,
		}
	case e.FailureCount > e.WorkingCount && e.FailureCount > 0:
		return v2StrategyInsight{
			State: "DEGRADED", Summary: "История содержит больше неудачных наблюдений, чем рабочих.",
			Signals: signals,
		}
	case e.VerifiedCount >= 3 && e.SuccessRate >= 0.90 &&
		(e.Confidence == v2MemoryConfidenceTrusted || e.Confidence == v2MemoryConfidenceStrong) &&
		e.FailureCount == 0:
		return v2StrategyInsight{
			State: "PROVEN", Summary: "Повторно подтверждённая стратегия с сильным evidence без зафиксированных failures.",
			Signals: signals,
		}
	case e.FailureCount > 0 || e.UnstableCount > 0:
		return v2StrategyInsight{
			State: "MIXED", Summary: "Рабочий evidence есть, но история содержит unstable/failure наблюдения.",
			Signals: signals,
		}
	case e.SuccessRate >= 0.70:
		return v2StrategyInsight{
			State: "PROMISING", Summary: "Есть положительный verified evidence, но данных ещё недостаточно для сильного вывода.",
			Signals: signals,
		}
	default:
		return v2StrategyInsight{
			State: "WEAK", Summary: "Evidence накоплен, но текущая успешность остаётся низкой.",
			Signals: signals,
		}
	}
}

func v2BuildStrategyInsights(scores v2StrategyScoreResponse) v2StrategyInsightsResponse {
	out := v2StrategyInsightsResponse{
		OK: true, Version: v2StrategyInsightsVersion, ReadOnly: true,
		Entries: []v2StrategyInsightEntry{},
	}
	for _, entry := range scores.Entries {
		out.Entries = append(out.Entries, v2StrategyInsightEntry{
			ID: entry.ID, Name: entry.Name, Fingerprint: entry.Fingerprint,
			Score: entry.Score, Insight: v2StrategyInsightForEntry(entry),
		})
	}
	out.Count = len(out.Entries)
	return out
}

func handleV2StrategyInsights(w http.ResponseWriter, _ *http.Request) {
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
	writeJSON(w, http.StatusOK, v2BuildStrategyInsights(scores))
}
