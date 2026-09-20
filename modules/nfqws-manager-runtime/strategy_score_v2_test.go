package main

import "testing"

func TestStrategyScoreUsesEvidenceAndReadiness(t *testing.T) {
	entry := v2StrategyRegistryEntry{
		Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
		Evidence: v2StrategyRegistryEvidence{
			Targets: 2, VerifiedCount: 3, SuccessRate: 1,
			Confidence: v2MemoryConfidenceTrusted, ReuseEligible: 2,
		},
	}
	score := v2StrategyScoreForEntry(entry)
	if score.Total != 75 {
		t.Fatalf("total=%d breakdown=%+v", score.Total, score.Breakdown)
	}
}

func TestStrategyScorePenalizesFailures(t *testing.T) {
	entry := v2StrategyRegistryEntry{
		Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
		Evidence: v2StrategyRegistryEvidence{
			Targets: 1, VerifiedCount: 2, SuccessRate: 0.5,
			Confidence:   v2MemoryConfidenceProbation,
			FailureCount: 2, UnstableCount: 1,
		},
	}
	score := v2StrategyScoreForEntry(entry)
	if score.Total != 27 {
		t.Fatalf("total=%d breakdown=%+v", score.Total, score.Breakdown)
	}
}

func TestStrategyScoresSortDeterministically(t *testing.T) {
	registry := v2StrategyRegistryResponse{
		Entries: []v2StrategyRegistryEntry{
			{ID: "b", Fingerprint: "bbbb", Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true}},
			{ID: "a", Fingerprint: "aaaa", Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true}},
		},
	}
	out := v2BuildStrategyScores(registry)
	if len(out.Entries) != 2 || out.Entries[0].ID != "a" || out.Entries[1].ID != "b" {
		t.Fatalf("order=%+v", out.Entries)
	}
	if !out.ReadOnly {
		t.Fatal("score endpoint must be read-only")
	}
}
