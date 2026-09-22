package main

import (
	"strings"
	"testing"
)

func v2MutationSearchTestResult(id, class, technique string, successRate float64) v2CandidateResult {
	successes := 0
	if successRate > 0 {
		successes = 1
	}
	return v2CandidateResult{
		CandidateID: id, CandidateName: id, CandidateSource: "synthesized",
		Args: []string{
			"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", technique,
		},
		Attempts:  []v2BenchAttempt{{OK: successRate > 0, CleanupProven: true, InfrastructureOK: true}},
		Successes: successes, SuccessRate: successRate, CompleteRate: successRate,
		ResultClass: class, CleanupProven: true, InfrastructureOK: true,
	}
}

func TestV2MutationRoundBudgetsByMode(t *testing.T) {
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	seed := v2MutationSearchTestResult("seed", "WORKING", "--lua-desync=multisplit:pos=1,midsld", 1)
	want := map[string]int{"fast": 2, "normal": 4, "thorough": 8}
	for name, budget := range want {
		mode, _ := v2SelectorMode(name)
		plan := v2BuildMutationRound("example.com", transport, mode, []v2CandidateResult{seed})
		if plan.Trace.Budget != budget {
			t.Fatalf("mode=%s budget=%d want=%d", name, plan.Trace.Budget, budget)
		}
		if len(plan.Candidates) > budget {
			t.Fatalf("mode=%s candidates=%d budget=%d", name, len(plan.Candidates), budget)
		}
	}
}

func TestV2MutationRoundSkipsWhenNoEligibleSeed(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	dead := v2MutationSearchTestResult("dead", "FAILED", "--lua-desync=multisplit:pos=1,midsld", 0)
	plan := v2BuildMutationRound("example.com", transport, mode, []v2CandidateResult{dead})
	if plan.Trace.SeedPlan.Selected != 0 || plan.Trace.Admitted != 0 || len(plan.Candidates) != 0 {
		t.Fatalf("unexpected mutation round: %+v", plan)
	}
}

func TestV2MutationRoundDoesNotReturnParentFingerprint(t *testing.T) {
	mode, _ := v2SelectorMode("thorough")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	seed := v2MutationSearchTestResult("seed", "WORKING", "--lua-desync=multisplit:pos=1,midsld", 1)
	parentFP := v2CandidateTechniqueFingerprint(seed.Args)
	plan := v2BuildMutationRound("example.com", transport, mode, []v2CandidateResult{seed})
	if plan.Trace.Admitted == 0 {
		t.Fatal("expected mutated candidates")
	}
	for _, item := range plan.Candidates {
		if item.Source != "mutated" {
			t.Fatalf("source=%q want mutated", item.Source)
		}
		if fp := v2CandidateTechniqueFingerprint(item.Args); fp == parentFP {
			t.Fatalf("parent fingerprint returned as mutation: %s", item.ID)
		}
	}
}

func TestV2MutationRoundSharesBudgetAcrossSeeds(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	first := v2MutationSearchTestResult("first", "WORKING", "--lua-desync=multisplit:pos=1,midsld", 1)
	second := v2MutationSearchTestResult("second", "PARTIAL", "--lua-desync=multidisorder:pos=1,midsld", 0)
	plan := v2BuildMutationRound("example.com", transport, mode, []v2CandidateResult{first, second})
	if plan.Trace.SeedPlan.Selected != 2 {
		t.Fatalf("selected seeds=%d want=2", plan.Trace.SeedPlan.Selected)
	}
	if plan.Trace.Admitted != 4 {
		t.Fatalf("admitted=%d want=4 trace=%+v", plan.Trace.Admitted, plan.Trace)
	}
	seenFirst, seenSecond := false, false
	for _, item := range plan.Candidates {
		if strings.Contains(item.ID, "-s01-") {
			seenFirst = true
		}
		if strings.Contains(item.ID, "-s02-") {
			seenSecond = true
		}
	}
	if !seenFirst || !seenSecond {
		t.Fatalf("budget was not shared across seeds: %+v", plan.Candidates)
	}
}

func TestV2CompileMutationRoundUsesExistingCandidateCompiler(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	seed := v2MutationSearchTestResult("seed", "WORKING", "--lua-desync=multisplit:pos=1,midsld", 1)
	plan := v2BuildMutationRound("example.com", transport, mode, []v2CandidateResult{seed})
	compiled, err := v2CompileMutationRound("example.com", transport, plan.Candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(compiled) != len(plan.Candidates) {
		t.Fatalf("compiled=%d candidates=%d", len(compiled), len(plan.Candidates))
	}
	for _, item := range compiled {
		if item.Profile.Index != -1 {
			t.Fatalf("mutation profile index=%d want=-1", item.Profile.Index)
		}
	}
}
