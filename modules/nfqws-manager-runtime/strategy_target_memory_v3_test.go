package main

import (
	"strings"
	"testing"
	"time"
)

func v3MemoryCandidate(class string, success, complete float64) v2CandidateResult {
	return v2CandidateResult{
		CandidateID: "candidate", CandidateName: "Candidate", CandidateSource: "builtin",
		Args: []string{
			"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
			"--lua-desync=multisplit:pos=1,midsld",
		},
		ResultClass: class, SuccessRate: success, CompleteRate: complete,
		InfrastructureOK: true, CleanupProven: true,
	}
}

func TestTargetMemoryV3AccumulatesIndependentObservations(t *testing.T) {
	env := strings.Repeat("a", 64)
	entry := v2TargetMemoryEntry{}
	entry = v2MergeTargetMemoryObservation(entry, v3MemoryCandidate("WORKING", 1, 1), env, strings.Repeat("1", 64), "2026-09-21T00:00:00Z")
	entry = v2MergeTargetMemoryObservation(entry, v3MemoryCandidate("UNSTABLE", 0.5, 0.5), env, strings.Repeat("1", 64), "2026-09-21T01:00:00Z")

	if entry.VerifiedCount != 2 || entry.ObservationCount != 2 {
		t.Fatalf("counts=%d/%d entry=%+v", entry.VerifiedCount, entry.ObservationCount, entry)
	}
	if entry.WorkingObservations != 1 || entry.UnstableObservations != 1 || entry.FailureObservations != 0 {
		t.Fatalf("observation classes=%+v", entry)
	}
	if entry.SuccessRate != 0.75 || entry.CompleteRate != 0.75 {
		t.Fatalf("rolling rates success=%v complete=%v", entry.SuccessRate, entry.CompleteRate)
	}
}

func TestTargetMemoryV3EnvironmentChangeResetsTrust(t *testing.T) {
	now := time.Date(2026, 9, 21, 2, 0, 0, 0, time.UTC)
	oldEnv := strings.Repeat("a", 64)
	newEnv := strings.Repeat("b", 64)
	entry := v2TargetMemoryEntry{
		Environment: oldEnv, ResultClass: "WORKING", SuccessRate: 1, CompleteRate: 1,
		VerifiedCount: 3, ObservationCount: 3, WorkingObservations: 3,
		SuccessStreak: 3, LastVerified: now.Add(-time.Hour).Format(time.RFC3339),
		InfrastructureOK: true, CleanupProven: true,
	}
	entry = v2MergeTargetMemoryObservation(entry, v3MemoryCandidate("WORKING", 1, 1), newEnv, strings.Repeat("2", 64), now.Format(time.RFC3339))

	if entry.EnvironmentChanges != 1 || entry.VerifiedCount != 1 || entry.SuccessStreak != 1 {
		t.Fatalf("environment reset failed: %+v", entry)
	}
	decision := v2TargetMemoryReuseDecisionForEntry(entry, now)
	if decision.Confidence != v2MemoryConfidenceFresh || !decision.ReuseEligible {
		t.Fatalf("old trust leaked across environment change: %+v", decision)
	}
}

func TestTargetMemoryV3HydratesLegacyCounters(t *testing.T) {
	entry := v2TargetMemoryEntry{VerifiedCount: 3, ResultClass: "WORKING"}
	v2HydrateLegacyObservationCounters(&entry)
	if entry.ObservationCount != 3 || entry.WorkingObservations != 3 {
		t.Fatalf("legacy hydration failed: %+v", entry)
	}
}

func TestTargetMemoryV3RegistryUsesHistoricalClassCounts(t *testing.T) {
	args := v3MemoryCandidate("WORKING", 1, 1).Args
	fp := v2CandidateTechniqueFingerprint(args)
	memory := v2TargetMemoryDocument{Version: v2TargetMemoryVersion, Entries: []v2TargetMemoryEntry{{
		Target: "example.com", Protocol: "https", IPFamily: "ipv4",
		CandidateID: "candidate", CandidateName: "Candidate", CandidateSource: "builtin",
		Fingerprint: fp, Args: args, ResultClass: "WORKING",
		VerifiedCount: 4, ObservationCount: 4, WorkingObservations: 3, FailureObservations: 1,
		SuccessRate: 0.75, CompleteRate: 0.75, SuccessStreak: 1,
		LastVerified:     time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC).Format(time.RFC3339),
		InfrastructureOK: true, CleanupProven: true,
	}}}
	registry := v2BuildStrategyRegistry(v2StrategyLibraryDocument{Version: 1}, memory, time.Date(2026, 9, 21, 2, 0, 0, 0, time.UTC))
	for _, item := range registry.Entries {
		if item.Fingerprint == fp {
			if item.Evidence.WorkingCount != 3 || item.Evidence.FailureCount != 1 {
				t.Fatalf("registry evidence=%+v", item.Evidence)
			}
			return
		}
	}
	t.Fatal("memory strategy missing from registry")
}
