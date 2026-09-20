package main

import (
	"net/http"
	"sort"
)

const v2StrategyScoreVersion = 1

type v2StrategyScoreBreakdown struct {
	Success    int `json:"success"`
	Confidence int `json:"confidence"`
	Verified   int `json:"verified"`
	Coverage   int `json:"coverage"`
	Readiness  int `json:"readiness"`
	Penalty    int `json:"penalty"`
}

type v2StrategyScore struct {
	Total     int                      `json:"total"`
	Breakdown v2StrategyScoreBreakdown `json:"breakdown"`
}

type v2StrategyScoreEntry struct {
	ID           string                         `json:"id"`
	Name         string                         `json:"name"`
	Protocol     string                         `json:"protocol"`
	Family       string                         `json:"family"`
	Fingerprint  string                         `json:"fingerprint"`
	Sources      []string                       `json:"sources"`
	Capabilities v2StrategyRegistryCapabilities `json:"capabilities"`
	Evidence     v2StrategyRegistryEvidence     `json:"evidence"`
	Score        v2StrategyScore                `json:"score"`
}

type v2StrategyScoreResponse struct {
	OK       bool                   `json:"ok"`
	Version  int                    `json:"version"`
	ReadOnly bool                   `json:"read_only"`
	Count    int                    `json:"count"`
	Entries  []v2StrategyScoreEntry `json:"entries"`
}

func registerStrategyScoreV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/strategy-scores", getOnly(handleV2StrategyScores))
}

func v2ClampScore(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func v2StrategyScoreForEntry(entry v2StrategyRegistryEntry) v2StrategyScore {
	e := entry.Evidence
	b := v2StrategyScoreBreakdown{}

	if e.VerifiedCount > 0 {
		b.Success = v2ClampScore(int(e.SuccessRate*30.0+0.5), 0, 30)
		b.Verified = v2ClampScore(e.VerifiedCount*3, 0, 15)
	}
	switch e.Confidence {
	case v2MemoryConfidenceTrusted:
		b.Confidence = 20
	case v2MemoryConfidenceStrong:
		b.Confidence = 15
	case v2MemoryConfidenceFresh:
		b.Confidence = 10
	case v2MemoryConfidenceProbation:
		b.Confidence = 5
	}
	b.Coverage = v2ClampScore(e.Targets*3, 0, 15)
	if entry.Capabilities.CandidateReady {
		b.Readiness = 10
	}
	b.Penalty = v2ClampScore(e.FailureCount*5+e.UnstableCount*2, 0, 20)

	total := b.Success + b.Confidence + b.Verified + b.Coverage + b.Readiness - b.Penalty
	return v2StrategyScore{
		Total:     v2ClampScore(total, 0, 100),
		Breakdown: b,
	}
}

func v2BuildStrategyScores(registry v2StrategyRegistryResponse) v2StrategyScoreResponse {
	out := v2StrategyScoreResponse{
		OK: true, Version: v2StrategyScoreVersion, ReadOnly: true,
		Entries: []v2StrategyScoreEntry{},
	}
	for _, entry := range registry.Entries {
		out.Entries = append(out.Entries, v2StrategyScoreEntry{
			ID: entry.ID, Name: entry.Name, Protocol: entry.Protocol, Family: entry.Family,
			Fingerprint: entry.Fingerprint, Sources: append([]string{}, entry.Sources...),
			Capabilities: entry.Capabilities, Evidence: entry.Evidence,
			Score: v2StrategyScoreForEntry(entry),
		})
	}
	sort.SliceStable(out.Entries, func(i, j int) bool {
		if out.Entries[i].Score.Total != out.Entries[j].Score.Total {
			return out.Entries[i].Score.Total > out.Entries[j].Score.Total
		}
		if out.Entries[i].Evidence.VerifiedCount != out.Entries[j].Evidence.VerifiedCount {
			return out.Entries[i].Evidence.VerifiedCount > out.Entries[j].Evidence.VerifiedCount
		}
		return out.Entries[i].Fingerprint < out.Entries[j].Fingerprint
	})
	out.Count = len(out.Entries)
	return out
}

func handleV2StrategyScores(w http.ResponseWriter, _ *http.Request) {
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
	writeJSON(w, http.StatusOK, v2BuildStrategyScores(registry))
}
