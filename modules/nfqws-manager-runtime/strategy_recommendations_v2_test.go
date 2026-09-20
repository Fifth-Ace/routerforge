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
