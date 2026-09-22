package main

import (
	"strings"
	"testing"
)

func v2MutationSeedTestResult(id, class string, successRate, completeRate float64, args ...string) v2CandidateResult {
	successes := 0
	if successRate > 0 {
		successes = 1
	}
	return v2CandidateResult{
		CandidateID: id, CandidateName: id, CandidateSource: "synthesized",
		Args:      append([]string{}, args...),
		Attempts:  []v2BenchAttempt{{OK: successRate > 0, CleanupProven: true, InfrastructureOK: true}},
		Successes: successes, SuccessRate: successRate, CompleteRate: completeRate,
		ResultClass: class, CleanupProven: true, InfrastructureOK: true,
	}
}

func TestV2MutationSeedBudgets(t *testing.T) {
	want := map[string]int{"fast": 1, "normal": 2, "thorough": 4}
	for name, expected := range want {
		mode, err := v2SelectorMode(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := v2MutationSeedBudget(mode); got != expected {
			t.Fatalf("mode=%s budget=%d want=%d", name, got, expected)
		}
	}
}

func TestV2MutationSeedSelectionUsesCleanDeadOnlyAsExploratoryFallback(t *testing.T) {
	mode, _ := v2SelectorMode("thorough")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	args := []string{
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	}
	dead := v2MutationSeedTestResult("dead", "FAILED", 0, 0, args...)
	inconclusive := v2MutationSeedTestResult("inconclusive", "INCONCLUSIVE", 0, 0, args...)
	inconclusive.InfrastructureOK = false
	baseline := v2MutationSeedTestResult("baseline", "PARTIAL", 0, 0, args...)
	baseline.Baseline = true
	plan := v2MutationSelectSeeds(mode, transport, []v2CandidateResult{dead, inconclusive, baseline})
	if plan.Selected != 1 || plan.Eligible != 1 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Seeds[0].Outcome != v2MutationSeedOutcomeExploratory {
		t.Fatalf("outcome=%q want EXPLORATORY", plan.Seeds[0].Outcome)
	}
}

func TestV2MutationSeedSelectionRejectsInconclusiveFallback(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	inconclusive := v2MutationSeedTestResult(
		"inconclusive", "INCONCLUSIVE", 0, 0,
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	)
	plan := v2MutationSelectSeeds(mode, transport, []v2CandidateResult{inconclusive})
	if plan.Selected != 0 || plan.Eligible != 0 {
		t.Fatalf("inconclusive candidate became mutation seed: %+v", plan)
	}
}

func TestV2MutationSeedSelectionRejectsInfraFailedDeadFallback(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	dead := v2MutationSeedTestResult(
		"dead-infra", "FAILED", 0, 0,
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	)
	dead.InfrastructureOK = false
	dead.Attempts[0].InfrastructureOK = false
	plan := v2MutationSelectSeeds(mode, transport, []v2CandidateResult{dead})
	if plan.Selected != 0 || plan.Eligible != 0 {
		t.Fatalf("infra-failed dead candidate became exploratory seed: %+v", plan)
	}
}

func TestV2MutationSeedSelectionNeverUsesDeadFallbackWhenNormalSeedExists(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	base := []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello"}
	dead := v2MutationSeedTestResult("dead", "FAILED", 0, 0, append(base, "--lua-desync=multisplit:pos=1,midsld")...)
	partial := v2MutationSeedTestResult("partial", "PARTIAL", 0, 0, append(base, "--lua-desync=multidisorder:pos=1,midsld")...)
	plan := v2MutationSelectSeeds(mode, transport, []v2CandidateResult{dead, partial})
	if plan.Selected != 1 || plan.Seeds[0].Outcome != v2MutationOutcomePartial {
		t.Fatalf("dead fallback competed with normal seed: %+v", plan)
	}
}

func TestV2MutationSeedSelectionRejectsDirtyDeadFallback(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	dead := v2MutationSeedTestResult(
		"dead", "FAILED", 0, 0,
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	)
	dead.CleanupProven = false
	plan := v2MutationSelectSeeds(mode, transport, []v2CandidateResult{dead})
	if plan.Selected != 0 || plan.Eligible != 0 {
		t.Fatalf("dirty dead candidate became exploratory seed: %+v", plan)
	}
}

func TestV2MutationSeedSelectionPrioritizesVerifiedThenPartialThenUnstable(t *testing.T) {
	mode, _ := v2SelectorMode("thorough")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	base := []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello"}
	verified := v2MutationSeedTestResult("verified", "WORKING", 1, 1, append(base, "--lua-desync=multisplit:pos=1,midsld")...)
	partial := v2MutationSeedTestResult("partial", "PARTIAL", 0, 0, append(base, "--lua-desync=multidisorder:pos=1,midsld")...)
	unstable := v2MutationSeedTestResult("unstable", "UNSTABLE", 0.5, 0.5, append(base, "--lua-desync=fakedsplit:pos=midsld")...)
	plan := v2MutationSelectSeeds(mode, transport, []v2CandidateResult{unstable, partial, verified})
	if plan.Selected != 3 {
		t.Fatalf("selected=%d want=3 plan=%+v", plan.Selected, plan)
	}
	if plan.Seeds[0].Outcome != v2MutationOutcomeVerified {
		t.Fatalf("first outcome=%s want VERIFIED", plan.Seeds[0].Outcome)
	}
	if plan.Seeds[1].Outcome != v2MutationOutcomePartial {
		t.Fatalf("second outcome=%s want PARTIAL", plan.Seeds[1].Outcome)
	}
	if plan.Seeds[2].Outcome != v2MutationOutcomeUnstable {
		t.Fatalf("third outcome=%s want UNSTABLE", plan.Seeds[2].Outcome)
	}
}

func TestV2MutationSeedMetricsCannotCrossOutcomeClass(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	base := []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello"}

	partial := v2MutationSeedTestResult(
		"partial", "PARTIAL", 0, 0,
		append(base, "--lua-desync=multisplit:pos=1,midsld")...,
	)
	unstable := v2MutationSeedTestResult(
		"unstable", "UNSTABLE", 0.99, 1,
		append(base, "--lua-desync=multidisorder:pos=1,midsld")...,
	)
	unstable.Successes = 5
	unstable.MedianTTFBMS = 10
	unstable.MedianThroughput = 10 * 1024 * 1024

	plan := v2MutationSelectSeeds(mode, transport, []v2CandidateResult{unstable, partial})
	if plan.Selected != 2 {
		t.Fatalf("selected=%d want=2 plan=%+v", plan.Selected, plan)
	}
	if plan.Seeds[0].Outcome != v2MutationOutcomePartial {
		t.Fatalf("metrics crossed outcome class: first=%s want PARTIAL", plan.Seeds[0].Outcome)
	}
	if plan.Seeds[1].Outcome != v2MutationOutcomeUnstable {
		t.Fatalf("second=%s want UNSTABLE", plan.Seeds[1].Outcome)
	}
}

func TestV2MutationSeedSelectionHonorsModeBudget(t *testing.T) {
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	base := []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello"}
	results := []v2CandidateResult{
		v2MutationSeedTestResult("a", "WORKING", 1, 1, append(base, "--lua-desync=multisplit:pos=1,midsld")...),
		v2MutationSeedTestResult("b", "WORKING", 1, 1, append(base, "--lua-desync=multidisorder:pos=1,midsld")...),
		v2MutationSeedTestResult("c", "UNSTABLE", 0.5, 0.5, append(base, "--lua-desync=fakedsplit:pos=midsld")...),
	}
	fast, _ := v2SelectorMode("fast")
	if got := v2MutationSelectSeeds(fast, transport, results).Selected; got != 1 {
		t.Fatalf("fast selected=%d want=1", got)
	}
	normal, _ := v2SelectorMode("normal")
	if got := v2MutationSelectSeeds(normal, transport, results).Selected; got != 2 {
		t.Fatalf("normal selected=%d want=2", got)
	}
}

func TestV2MutationSeedSelectionDeduplicatesTechniqueFingerprint(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	args := []string{
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	}
	weak := v2MutationSeedTestResult("weak", "UNSTABLE", 0.5, 0.5, args...)
	strong := v2MutationSeedTestResult("strong", "WORKING", 1, 1, args...)
	plan := v2MutationSelectSeeds(mode, transport, []v2CandidateResult{weak, strong})
	if plan.Eligible != 1 || plan.Selected != 1 || plan.Duplicates != 1 {
		t.Fatalf("unexpected dedupe plan: %+v", plan)
	}
	if plan.Seeds[0].Result.CandidateID != "strong" {
		t.Fatalf("kept seed=%s want strong", plan.Seeds[0].Result.CandidateID)
	}
}

func TestV2MutationSeedPlanCarriesExplainableReasonAndFamily(t *testing.T) {
	mode, _ := v2SelectorMode("fast")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	result := v2MutationSeedTestResult(
		"partial", "PARTIAL", 0, 0,
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=fakedsplit:pos=midsld",
	)
	plan := v2MutationSelectSeeds(mode, transport, []v2CandidateResult{result})
	if plan.Selected != 1 {
		t.Fatalf("selected=%d", plan.Selected)
	}
	if plan.Seeds[0].Seed.Family != "fake-split" {
		t.Fatalf("family=%q want fake-split", plan.Seeds[0].Seed.Family)
	}
	if !strings.Contains(plan.Seeds[0].Reason, "partial") || !strings.Contains(plan.Seeds[0].Reason, "adaptive score=") {
		t.Fatalf("reason=%q", plan.Seeds[0].Reason)
	}
}
