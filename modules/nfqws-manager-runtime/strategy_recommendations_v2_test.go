package main

import "testing"

func TestRecommendationEngineRanksWithinProtocol(t *testing.T) {
	scores := v2StrategyScoreResponse{Entries: []v2StrategyScoreEntry{
		{
			ID: "weak", Name: "Weak", Protocol: "https", Family: "split", Fingerprint: "bbbb",
			Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
			Evidence:     v2StrategyRegistryEvidence{VerifiedCount: 6, SuccessRate: 0.6},
			Score:        v2StrategyScore{Total: 70},
		},
		{
			ID: "proven", Name: "Proven", Protocol: "https", Family: "fake", Fingerprint: "aaaa",
			Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
			Evidence:     v2StrategyRegistryEvidence{VerifiedCount: 3, SuccessRate: 1},
			Score:        v2StrategyScore{Total: 85},
		},
	}}
	insights := v2StrategyInsightsResponse{Entries: []v2StrategyInsightEntry{
		{Fingerprint: "bbbb", Insight: v2StrategyInsight{State: "WEAK"}},
		{Fingerprint: "aaaa", Insight: v2StrategyInsight{State: "PROVEN"}},
	}}
	gate := v2PolicyAutomationGateResponse{Candidates: []v2PolicyAutomationCandidate{
		{Fingerprint: "aaaa", Eligible: true},
	}}
	out := v2BuildStrategyRecommendations(scores, insights, gate)
	if out.Count != 2 || out.Primary != 1 {
		t.Fatalf("response=%+v", out)
	}
	if out.Items[0].ID != "proven" || out.Items[0].Rank != 1 || !out.Items[0].Primary || !out.Items[0].AutomationReady {
		t.Fatalf("first=%+v", out.Items[0])
	}
	if out.Items[1].ID != "weak" || out.Items[1].Rank != 2 || out.Items[1].Primary {
		t.Fatalf("second=%+v", out.Items[1])
	}
}

func TestRecommendationEngineExcludesUnverifiedAndNotReady(t *testing.T) {
	scores := v2StrategyScoreResponse{Entries: []v2StrategyScoreEntry{
		{
			ID: "unverified", Protocol: "https", Fingerprint: "a",
			Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
		},
		{
			ID: "not-ready", Protocol: "https", Fingerprint: "b",
			Capabilities: v2StrategyRegistryCapabilities{CandidateReady: false},
			Evidence:     v2StrategyRegistryEvidence{VerifiedCount: 4, SuccessRate: 1},
		},
	}}
	out := v2BuildStrategyRecommendations(scores, v2StrategyInsightsResponse{}, v2PolicyAutomationGateResponse{})
	if out.Count != 0 || !out.ReadOnly {
		t.Fatalf("response=%+v", out)
	}
}

func TestRecommendationSemanticChainPreservesMatureStrategy(t *testing.T) {
	registry := v2StrategyRegistryResponse{
		OK: true, Version: v2StrategyRegistryVersion, ReadOnly: true,
		Entries: []v2StrategyRegistryEntry{{
			ID: "semantic-chain", Name: "Semantic Chain", Protocol: "https", Family: "split",
			Fingerprint: "semantic-chain-fingerprint",
			Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
			Evidence: v2StrategyRegistryEvidence{
				Targets: 5, VerifiedCount: 5, WorkingCount: 5,
				SuccessRate: 1, Confidence: v2MemoryConfidenceTrusted, ReuseEligible: 3,
			},
		}},
	}

	scores := v2BuildStrategyScores(registry)
	if !scores.ReadOnly || len(scores.Entries) != 1 {
		t.Fatalf("scores=%+v", scores)
	}
	score := scores.Entries[0]
	if score.Fingerprint != "semantic-chain-fingerprint" || score.Score.Total < v2PolicyAutomationMinScore {
		t.Fatalf("score identity/threshold lost: %+v", score)
	}

	insights := v2BuildStrategyInsights(scores)
	if !insights.ReadOnly || len(insights.Entries) != 1 {
		t.Fatalf("insights=%+v", insights)
	}
	if insights.Entries[0].Fingerprint != score.Fingerprint || insights.Entries[0].Insight.State != "PROVEN" {
		t.Fatalf("insight semantic mismatch: %+v", insights.Entries[0])
	}

	gate := v2BuildPolicyAutomationGate(scores, insights)
	if gate.AutomationEnabled || !gate.ReadOnly || gate.ReadyCount != 1 || len(gate.Candidates) != 1 {
		t.Fatalf("policy gate=%+v", gate)
	}
	if !gate.Candidates[0].Eligible || gate.Candidates[0].Fingerprint != score.Fingerprint {
		t.Fatalf("policy identity/eligibility mismatch: %+v", gate.Candidates[0])
	}

	recommendations := v2BuildStrategyRecommendations(scores, insights, gate)
	if !recommendations.ReadOnly || recommendations.Count != 1 || recommendations.Primary != 1 {
		t.Fatalf("recommendations=%+v", recommendations)
	}
	item := recommendations.Items[0]
	if item.Fingerprint != score.Fingerprint || item.Insight != "PROVEN" ||
		!item.AutomationReady || !item.Primary || item.Rank != 1 {
		t.Fatalf("recommendation semantic mismatch: %+v", item)
	}
}
func TestRecommendationEngineKeepsProtocolsIndependent(t *testing.T) {
	scores := v2StrategyScoreResponse{Entries: []v2StrategyScoreEntry{
		{
			ID: "https-a", Protocol: "https", Fingerprint: "a",
			Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
			Evidence:     v2StrategyRegistryEvidence{VerifiedCount: 3, SuccessRate: 1},
			Score:        v2StrategyScore{Total: 90},
		},
		{
			ID: "quic-a", Protocol: "quic", Fingerprint: "b",
			Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
			Evidence:     v2StrategyRegistryEvidence{VerifiedCount: 3, SuccessRate: 1},
			Score:        v2StrategyScore{Total: 88},
		},
	}}
	insights := v2StrategyInsightsResponse{Entries: []v2StrategyInsightEntry{
		{Fingerprint: "a", Insight: v2StrategyInsight{State: "PROVEN"}},
		{Fingerprint: "b", Insight: v2StrategyInsight{State: "PROMISING"}},
	}}
	out := v2BuildStrategyRecommendations(scores, insights, v2PolicyAutomationGateResponse{})
	if out.Primary != 2 {
		t.Fatalf("primary=%d items=%+v", out.Primary, out.Items)
	}
	for _, item := range out.Items {
		if item.Rank != 1 || !item.Primary {
			t.Fatalf("item=%+v", item)
		}
	}
}
