package main

import (
	"strings"
	"testing"
)

func v2MutationTestCandidate(id, family string, args ...string) v2CandidatePoolItem {
	return v2CandidatePoolItem{
		ID: id, Name: id, Source: "synthesized", Family: family,
		Protocol: benchTransportHTTPS, Args: append([]string{}, args...),
	}
}

func TestV2MutationBudgetIsBounded(t *testing.T) {
	cases := map[string]int{"fast": 2, "normal": 4, "thorough": 8}
	for name, want := range cases {
		mode, err := v2SelectorMode(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := v2MutationBudget(mode, mode.MaxCandidates); got != want {
			t.Fatalf("mode=%s budget=%d want=%d", name, got, want)
		}
		if got := v2MutationBudget(mode, 1); got != 1 {
			t.Fatalf("mode=%s remaining clamp=%d want=1", name, got)
		}
	}
}

func TestV2MutationOutcomeClassification(t *testing.T) {
	working := v2CandidateResult{
		ResultClass: "WORKING", Successes: 2, SuccessRate: 1,
		CleanupProven: true, InfrastructureOK: true,
		Attempts: []v2BenchAttempt{{OK: true}, {OK: true}},
	}
	if got := v2MutationOutcome(working); got != v2MutationOutcomeVerified {
		t.Fatalf("working outcome=%q", got)
	}
	partial := working
	partial.ResultClass = "PARTIAL"
	partial.SuccessRate = 0
	partial.Successes = 0
	if got := v2MutationOutcome(partial); got != v2MutationOutcomePartial {
		t.Fatalf("partial outcome=%q", got)
	}
	unstable := working
	unstable.ResultClass = "UNSTABLE"
	unstable.SuccessRate = 0.5
	unstable.Successes = 1
	if got := v2MutationOutcome(unstable); got != v2MutationOutcomeUnstable {
		t.Fatalf("unstable outcome=%q", got)
	}
	dead := working
	dead.ResultClass = "FAILED"
	dead.SuccessRate = 0
	dead.Successes = 0
	if got := v2MutationOutcome(dead); got != v2MutationOutcomeDead {
		t.Fatalf("dead outcome=%q", got)
	}
	inconclusive := working
	inconclusive.CleanupProven = false
	if got := v2MutationOutcome(inconclusive); got != v2MutationOutcomeInconclusive {
		t.Fatalf("inconclusive outcome=%q", got)
	}
}

func TestV2MutationSeedEligibilityIsConservative(t *testing.T) {
	partial := v2CandidateResult{
		ResultClass: "PARTIAL", CleanupProven: true, InfrastructureOK: true,
		Attempts: []v2BenchAttempt{{CleanupProven: true, InfrastructureOK: true}},
	}
	if !v2MutationSeedEligible(partial) {
		t.Fatal("PARTIAL candidate should be eligible for neighbor search")
	}
	dead := partial
	dead.ResultClass = "FAILED"
	if v2MutationSeedEligible(dead) {
		t.Fatal("DEAD candidate must not consume mutation budget")
	}
	inconclusive := partial
	inconclusive.InfrastructureOK = false
	if v2MutationSeedEligible(inconclusive) {
		t.Fatal("INCONCLUSIVE candidate must not consume mutation budget")
	}
}

func TestV2MutationHTTPSNeighborsAreUniqueAndCompilable(t *testing.T) {
	mode, _ := v2SelectorMode("thorough")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	seed := v2MutationTestCandidate(
		"seed", "fake+split",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=fake:blob=tls_clienthello:repeats=2",
		"--lua-desync=multisplit:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello",
	)
	items, meta := v2MutateCandidateNeighbors("example.com", transport, mode, seed, 24)
	if len(items) < 10 {
		t.Fatalf("neighbor pool unexpectedly small: %d meta=%+v", len(items), meta)
	}
	if meta.Version != v2StrategyMutationVersion || meta.Admitted != len(items) {
		t.Fatalf("bad mutation meta: %+v", meta)
	}
	seen := map[string]bool{}
	joined := ""
	for _, item := range items {
		if item.Source != "mutated" {
			t.Fatalf("source=%q want mutated", item.Source)
		}
		if _, err := v2CustomProfileForTransport(item.Args, "example.com", transport); err != nil {
			t.Fatalf("candidate %s does not compile: %v", item.ID, err)
		}
		fp := v2CandidateTechniqueFingerprint(item.Args)
		if fp == "" || seen[fp] {
			t.Fatalf("duplicate/empty fingerprint: %q", fp)
		}
		seen[fp] = true
		joined += strings.Join(item.Args, " ") + "\n"
	}
	for _, want := range []string{"multidisorder", "seqovl=336", "repeats=6", "ip_ttl=5"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("neighbor set missing axis %q:\n%s", want, joined)
		}
	}
}

func TestV2MutationQUICMutatesRepeatsAndTTL(t *testing.T) {
	mode, _ := v2SelectorMode("thorough")
	transport, _ := normalizeBenchTransport(benchTransportQUIC)
	seed := v2CandidatePoolItem{
		ID: "quic-seed", Name: "QUIC seed", Source: "synthesized", Family: "fake",
		Protocol: benchTransportQUIC,
		Args: []string{
			"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial",
			"--lua-desync=fake:blob=fake_default_quic:repeats=2",
		},
	}
	items, meta := v2MutateCandidateNeighbors("example.com", transport, mode, seed, 12)
	joined := ""
	for _, item := range items {
		joined += strings.Join(item.Args, " ") + "\n"
	}
	if !strings.Contains(joined, "repeats=11") {
		t.Fatalf("QUIC mutation missing repeats axis: %s", joined)
	}
	if !strings.Contains(joined, "ip_ttl=5") {
		t.Fatalf("QUIC mutation missing TTL axis: %s", joined)
	}
	if len(meta.Axes) < 2 {
		t.Fatalf("expected multiple mutation axes: %+v", meta)
	}
}

func TestV2MutationNeverReturnsSeedFingerprint(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	seed := v2MutationTestCandidate(
		"seed", "split",
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	)
	seedFP := v2CandidateTechniqueFingerprint(seed.Args)
	items, _ := v2MutateCandidateNeighbors("example.com", transport, mode, seed, 12)
	for _, item := range items {
		if got := v2CandidateTechniqueFingerprint(item.Args); got == seedFP {
			t.Fatalf("mutation returned unchanged seed: %s", item.ID)
		}
	}
}
