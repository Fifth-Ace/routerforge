package main

import "testing"

func v2MutationPolicyTestResult(id, class string, successRate float64) v2CandidateResult {
	successes := 0
	if successRate > 0 {
		successes = 1
	}
	return v2CandidateResult{
		CandidateID: id, CandidateName: id, CandidateSource: "synthesized",
		Args: []string{
			"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
			"--lua-desync=multisplit:pos=1,midsld",
		},
		Attempts: []v2BenchAttempt{{
			OK: true, CleanupProven: true, InfrastructureOK: true,
		}},
		Successes: successes, SuccessRate: successRate, CompleteRate: successRate,
		ResultClass: class, CleanupProven: true, InfrastructureOK: true,
	}
}

func TestV2MutationStopPolicyStopsWhenBaselineWorking(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	baseline := v2MutationPolicyTestResult("baseline", "WORKING", 1)
	baseline.Baseline = true
	decision := v2MutationEvaluateStopPolicy(mode, transport, baseline, nil)
	if decision.Continue || decision.Code != v2MutationStopBaselineWorking {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestV2MutationStopPolicyContinuesWithCleanDeadExploratoryFallback(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	baseline := v2MutationPolicyTestResult("baseline", "FAILED", 0)
	baseline.Baseline = true
	dead := v2MutationPolicyTestResult("dead", "FAILED", 0)
	decision := v2MutationEvaluateStopPolicy(mode, transport, baseline, []v2CandidateResult{dead})
	if !decision.Continue || decision.Code != v2MutationStopContinue {
		t.Fatalf("decision=%+v", decision)
	}
	if decision.SelectedSeeds != 1 || decision.SeedPlan.Seeds[0].Outcome != v2MutationSeedOutcomeExploratory {
		t.Fatalf("exploratory fallback missing: %+v", decision)
	}
}

func TestV2MutationStopPolicyStillStopsWhenOnlyInconclusiveCandidateExists(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	baseline := v2MutationPolicyTestResult("baseline", "FAILED", 0)
	baseline.Baseline = true
	inconclusive := v2MutationPolicyTestResult("inconclusive", "INCONCLUSIVE", 0)
	inconclusive.InfrastructureOK = false
	decision := v2MutationEvaluateStopPolicy(mode, transport, baseline, []v2CandidateResult{inconclusive})
	if decision.Continue || decision.Code != v2MutationStopNoEligibleSeeds {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestV2MutationStopPolicyFastStopsOnVerified(t *testing.T) {
	mode, _ := v2SelectorMode("fast")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	baseline := v2MutationPolicyTestResult("baseline", "FAILED", 0)
	baseline.Baseline = true
	verified := v2MutationPolicyTestResult("verified", "WORKING", 1)
	decision := v2MutationEvaluateStopPolicy(mode, transport, baseline, []v2CandidateResult{verified})
	if decision.Continue || decision.Code != v2MutationStopFastVerified {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestV2MutationStopPolicyNormalContinuesForPartial(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	baseline := v2MutationPolicyTestResult("baseline", "FAILED", 0)
	baseline.Baseline = true
	verified := v2MutationPolicyTestResult("verified", "WORKING", 1)
	partial := v2MutationPolicyTestResult("partial", "PARTIAL", 0)
	partial.Args = append([]string{}, partial.Args...)
	partial.Args[len(partial.Args)-1] = "--lua-desync=multidisorder:pos=1,midsld"
	decision := v2MutationEvaluateStopPolicy(mode, transport, baseline, []v2CandidateResult{verified, partial})
	if !decision.Continue || decision.Code != v2MutationStopContinue {
		t.Fatalf("decision=%+v", decision)
	}
	if decision.VerifiedSeeds != 1 || decision.NonVerifiedEligible != 1 {
		t.Fatalf("decision counts=%+v", decision)
	}
}

func TestV2MutationStopPolicyNormalStopsWhenEligibleSetVerified(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	baseline := v2MutationPolicyTestResult("baseline", "FAILED", 0)
	baseline.Baseline = true
	verified := v2MutationPolicyTestResult("verified", "WORKING", 1)
	decision := v2MutationEvaluateStopPolicy(mode, transport, baseline, []v2CandidateResult{verified})
	if decision.Continue || decision.Code != v2MutationStopVerifiedSetComplete {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestV2MutationStopPolicyThoroughRequiresTwoVerified(t *testing.T) {
	mode, _ := v2SelectorMode("thorough")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	baseline := v2MutationPolicyTestResult("baseline", "FAILED", 0)
	baseline.Baseline = true
	first := v2MutationPolicyTestResult("first", "WORKING", 1)
	second := v2MutationPolicyTestResult("second", "WORKING", 1)
	second.Args = append([]string{}, second.Args...)
	second.Args[len(second.Args)-1] = "--lua-desync=multidisorder:pos=1,midsld"

	one := v2MutationEvaluateStopPolicy(mode, transport, baseline, []v2CandidateResult{first})
	if !one.Continue {
		t.Fatalf("thorough stopped on one verified seed: %+v", one)
	}
	two := v2MutationEvaluateStopPolicy(mode, transport, baseline, []v2CandidateResult{first, second})
	if two.Continue || two.Code != v2MutationStopMultipleVerified {
		t.Fatalf("decision=%+v", two)
	}
}
