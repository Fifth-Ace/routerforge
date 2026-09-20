package main

import "testing"

func v2PolicyTestEntry(score int, failures, unstable, reuse int) v2StrategyScoreEntry {
	return v2StrategyScoreEntry{
		ID: "candidate", Name: "Candidate", Fingerprint: "fingerprint",
		Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
		Evidence: v2StrategyRegistryEvidence{
			VerifiedCount: 4, WorkingCount: 4, FailureCount: failures, UnstableCount: unstable,
			SuccessRate: 1, Confidence: v2MemoryConfidenceTrusted, ReuseEligible: reuse,
		},
		Score: v2StrategyScore{Total: score},
	}
}

func TestPolicyAutomationEligibilityRequiresMatureEvidence(t *testing.T) {
	entry := v2PolicyTestEntry(90, 0, 0, 1)
	ok, reasons := v2PolicyAutomationEligibility(entry, v2StrategyInsight{State: "PROVEN"})
	if !ok || len(reasons) != 0 {
		t.Fatalf("eligible=%v reasons=%v", ok, reasons)
	}
}

func TestPolicyAutomationEligibilityRejectsMixedHistory(t *testing.T) {
	entry := v2PolicyTestEntry(90, 1, 1, 1)
	ok, reasons := v2PolicyAutomationEligibility(entry, v2StrategyInsight{State: "MIXED"})
	if ok {
		t.Fatal("mixed history passed automation gate")
	}
	want := map[string]bool{
		"insight_not_proven":          true,
		"historical_failures_present": true,
		"historical_unstable_present": true,
	}
	for _, reason := range reasons {
		delete(want, reason)
	}
	if len(want) != 0 {
		t.Fatalf("missing reasons=%v got=%v", want, reasons)
	}
}

func TestPolicyAutomationGateNeverEnablesAutomation(t *testing.T) {
	entry := v2PolicyTestEntry(90, 0, 0, 1)
	scores := v2StrategyScoreResponse{Entries: []v2StrategyScoreEntry{entry}}
	insights := v2StrategyInsightsResponse{Entries: []v2StrategyInsightEntry{{
		Fingerprint: entry.Fingerprint, Insight: v2StrategyInsight{State: "PROVEN"},
	}}}
	out := v2BuildPolicyAutomationGate(scores, insights)
	if out.AutomationEnabled || !out.ReadOnly {
		t.Fatalf("unsafe gate state=%+v", out)
	}
	if out.ReadyCount != 1 || len(out.Candidates) != 1 || !out.Candidates[0].Eligible {
		t.Fatalf("gate=%+v", out)
	}
}
