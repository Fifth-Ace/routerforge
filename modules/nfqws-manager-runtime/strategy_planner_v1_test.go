package main

import "testing"

func TestRecommendationPlannerPromotesTLSFamily(t *testing.T) {
	resp := v2CandidatePoolResponse{
		RecommendationHints: 2,
		Candidates: []v2CandidatePoolItem{
			{ID: "a", Source: "builtin", Family: "fake", Stage: "full", Fingerprint: "a"},
			{ID: "b", Source: "curated", Family: "split", Stage: "quick", Fingerprint: "b"},
			{ID: "c", Source: "curated", Family: "disorder", Stage: "quick", Fingerprint: "c"},
		},
	}
	v2ApplyRecommendationPlanner(&resp, v2PlannerHint{
		DiagnosticCode: "TLS_HANDSHAKE_TIMEOUT", FaultDomain: "tls_path", StrategyRelevant: true,
	})
	if resp.PlannerVersion != 1 || !resp.PlannerReadOnly || !resp.PlannerStrategyRelevant {
		t.Fatalf("planner metadata=%+v", resp)
	}
	if resp.Candidates[0].ID == "a" {
		t.Fatalf("TLS planner failed to promote split/disorder family: %+v", resp.Candidates)
	}
	if resp.PlannerPromotedCount == 0 {
		t.Fatalf("expected promoted count")
	}
}

func TestRecommendationPlannerDoesNotUseDiagnosticWhenIrrelevant(t *testing.T) {
	resp := v2CandidatePoolResponse{
		Candidates: []v2CandidatePoolItem{
			{ID: "a", Source: "builtin", Family: "fake", Stage: "quick", Fingerprint: "a"},
			{ID: "b", Source: "curated", Family: "split", Stage: "quick", Fingerprint: "b"},
		},
	}
	v2ApplyRecommendationPlanner(&resp, v2PlannerHint{
		DiagnosticCode: "CLEAR_END_TO_END", FaultDomain: "none", StrategyRelevant: false,
	})
	if resp.Candidates[0].ID != "a" {
		t.Fatalf("irrelevant diagnostic changed normal order: %+v", resp.Candidates)
	}
	if resp.Candidates[0].PlannerDiagnosticHint || resp.Candidates[1].PlannerDiagnosticHint {
		t.Fatalf("irrelevant diagnostic became a candidate hint")
	}
}

func TestRecommendationPlannerHistoricalEvidenceIsHintOnly(t *testing.T) {
	resp := v2CandidatePoolResponse{
		RecommendationHints: 1,
		RecommendationAdded: 1,
		Candidates: []v2CandidatePoolItem{
			{ID: "weak", Source: "builtin", Family: "fake", Stage: "quick", Fingerprint: "weak", HistoricalInsight: "DEGRADED"},
			{ID: "proven", Source: "memory", Family: "split", Stage: "memory", Fingerprint: "proven", HistoricalInsight: "PROVEN", HistoricalPrimary: true, HistoricalPromoted: true},
		},
	}
	v2ApplyRecommendationPlanner(&resp, v2PlannerHint{})
	if resp.Candidates[0].ID != "proven" {
		t.Fatalf("historical hint not promoted: %+v", resp.Candidates)
	}
	if !resp.PlannerReadOnly {
		t.Fatalf("planner must remain read-only")
	}
}

func TestMergeHistoricalRecommendationPreservesSourceIdentity(t *testing.T) {
	items := []v2CandidatePoolItem{
		{ID: "memory-id", Source: "memory", Family: "split", Fingerprint: "fp", Args: []string{"--filter-tcp=443"}},
	}
	hint := v2CandidatePoolItem{
		ID: "registry-id", Source: "curated", Family: "split", Fingerprint: "fp",
		HistoricalRank: 1, HistoricalScore: 91, HistoricalInsight: "PROVEN",
		HistoricalPrimary: true, HistoricalPromoted: true, HistoricalReason: "historical evidence",
	}
	if !v2MergeHistoricalRecommendation(items, hint) {
		t.Fatalf("existing candidate was not found")
	}
	if items[0].Source != "memory" || items[0].ID != "memory-id" {
		t.Fatalf("source identity changed: %+v", items[0])
	}
	if !items[0].PlannerAlreadyInPool || items[0].HistoricalRank != 1 || items[0].HistoricalInsight != "PROVEN" {
		t.Fatalf("historical metadata not merged: %+v", items[0])
	}
}
