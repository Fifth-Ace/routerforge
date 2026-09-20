package main

import "testing"

func TestStrategyInsightStates(t *testing.T) {
	base := v2StrategyScoreEntry{
		Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
	}
	cases := []struct {
		name  string
		entry v2StrategyScoreEntry
		want  string
	}{
		{
			name: "not ready",
			entry: v2StrategyScoreEntry{
				Capabilities: v2StrategyRegistryCapabilities{CandidateReady: false},
			},
			want: "NOT_READY",
		},
		{
			name:  "unverified",
			entry: base,
			want:  "UNVERIFIED",
		},
		{
			name: "proven",
			entry: v2StrategyScoreEntry{
				Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
				Evidence: v2StrategyRegistryEvidence{
					VerifiedCount: 3, WorkingCount: 3, SuccessRate: 1,
					Confidence: v2MemoryConfidenceTrusted,
				},
				Score: v2StrategyScore{Total: 84},
			},
			want: "PROVEN",
		},
		{
			name: "mixed",
			entry: v2StrategyScoreEntry{
				Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
				Evidence: v2StrategyRegistryEvidence{
					VerifiedCount: 3, WorkingCount: 2, FailureCount: 1,
					SuccessRate: 0.8, Confidence: v2MemoryConfidenceFresh,
				},
			},
			want: "MIXED",
		},
		{
			name: "degraded",
			entry: v2StrategyScoreEntry{
				Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
				Evidence: v2StrategyRegistryEvidence{
					VerifiedCount: 4, WorkingCount: 1, FailureCount: 3,
					SuccessRate: 0.25,
				},
			},
			want: "DEGRADED",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := v2StrategyInsightForEntry(tc.entry)
			if got.State != tc.want {
				t.Fatalf("state=%s want=%s insight=%+v", got.State, tc.want, got)
			}
		})
	}
}

func TestStrategyInsightsStayReadOnlyAndDeterministic(t *testing.T) {
	scores := v2StrategyScoreResponse{
		Entries: []v2StrategyScoreEntry{
			{
				ID: "a", Name: "A", Fingerprint: "aaaa",
				Capabilities: v2StrategyRegistryCapabilities{CandidateReady: true},
				Score:        v2StrategyScore{Total: 10},
			},
		},
	}
	out := v2BuildStrategyInsights(scores)
	if !out.ReadOnly || out.Version != v2StrategyInsightsVersion || out.Count != 1 {
		t.Fatalf("response=%+v", out)
	}
	if out.Entries[0].Fingerprint != "aaaa" || out.Entries[0].Insight.State != "UNVERIFIED" {
		t.Fatalf("entry=%+v", out.Entries[0])
	}
}
