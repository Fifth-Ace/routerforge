package main

import "testing"

func TestC3BBuiltinCatalogTransportAware(t *testing.T) {
	cases := map[string]int{
		benchTransportHTTPS: 8,
		benchTransportHTTP:  3,
		benchTransportQUIC:  4,
		benchTransportSTUN:  4,
	}
	for id, want := range cases {
		transport, err := normalizeBenchTransport(id)
		if err != nil {
			t.Fatal(err)
		}
		items := v2BuiltinCandidatesForTransport(transport)
		if len(items) != want {
			t.Fatalf("transport=%s builtins=%d want=%d", id, len(items), want)
		}
		seenID := map[string]bool{}
		seenFP := map[string]bool{}
		for _, item := range items {
			if item.ID == "" || seenID[item.ID] {
				t.Fatalf("transport=%s duplicate/empty id=%q", id, item.ID)
			}
			seenID[item.ID] = true
			if item.Protocol != id {
				t.Fatalf("transport=%s builtin=%s protocol=%s", id, item.ID, item.Protocol)
			}
			if _, err := v2CustomProfileForTransport(item.Args, "example.com", transport); err != nil {
				t.Fatalf("transport=%s builtin=%s compile: %v", id, item.ID, err)
			}
			fp := v2CandidateTechniqueFingerprint(item.Args)
			if fp == "" || seenFP[fp] {
				t.Fatalf("transport=%s duplicate/empty fingerprint for %s", id, item.ID)
			}
			seenFP[fp] = true
		}
	}
}

func TestC3BProgressiveBudgetsReserveEveryStage(t *testing.T) {
	for _, name := range []string{"fast", "normal", "thorough"} {
		mode, err := v2SelectorMode(name)
		if err != nil {
			t.Fatal(err)
		}
		budgets := v2ProgressiveStageBudgets(mode)
		total := 0
		for _, stage := range v2ProgressiveStageOrder {
			budget := budgets[stage]
			if budget < 1 {
				t.Fatalf("mode=%s stage=%s budget=%d", name, stage, budget)
			}
			total += budget
		}
		if total != mode.MaxCandidates {
			t.Fatalf("mode=%s budget total=%d max=%d", name, total, mode.MaxCandidates)
		}
	}
}

func TestC3BProgressiveAppendHonorsStageBudget(t *testing.T) {
	mode, _ := v2SelectorMode("fast")
	plan := v2ProgressivePlan{
		Stages: map[string][]v2ProgressiveTemplate{}, Budgets: v2ProgressiveStageBudgets(mode),
		Warnings: []string{}, ByFingerprint: map[string]v2ProgressiveTemplate{},
	}
	for _, stage := range v2ProgressiveStageOrder {
		plan.Stages[stage] = []v2ProgressiveTemplate{}
	}
	plan.Budgets[v2ProgressiveStageQuick] = 1
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	first, err := v2CustomProfileForTransport(v2BuiltinHTTPSCandidates[0].Args, "example.com", transport)
	if err != nil {
		t.Fatal(err)
	}
	second, err := v2CustomProfileForTransport(v2BuiltinHTTPSCandidates[1].Args, "example.com", transport)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	if !v2ProgressiveAppendTemplate(&plan, seen, mode.MaxCandidates, v2ProgressiveTemplate{
		Stage: v2ProgressiveStageQuick, Profile: first, ID: "first", Name: "first", Source: "builtin",
	}) {
		t.Fatal("first QUICK candidate should fit stage budget")
	}
	if v2ProgressiveAppendTemplate(&plan, seen, mode.MaxCandidates, v2ProgressiveTemplate{
		Stage: v2ProgressiveStageQuick, Profile: second, ID: "second", Name: "second", Source: "builtin",
	}) {
		t.Fatal("second QUICK candidate must be rejected after stage budget is full")
	}
	if len(plan.Stages[v2ProgressiveStageQuick]) != 1 {
		t.Fatalf("quick stage size=%d", len(plan.Stages[v2ProgressiveStageQuick]))
	}
}

func TestC3BProgressiveBuiltinsUseSharedCatalog(t *testing.T) {
	for _, id := range []string{benchTransportHTTPS, benchTransportHTTP, benchTransportQUIC, benchTransportSTUN} {
		transport, _ := normalizeBenchTransport(id)
		shared := v2BuiltinCandidatesForTransport(transport)
		progressive := v2ProgressiveBuiltins(transport)
		if len(shared) != len(progressive) {
			t.Fatalf("transport=%s shared=%d progressive=%d", id, len(shared), len(progressive))
		}
		for i := range shared {
			if shared[i].ID != progressive[i].ID || v2CandidateTechniqueFingerprint(shared[i].Args) != v2CandidateTechniqueFingerprint(progressive[i].Args) {
				t.Fatalf("transport=%s builtin[%d] diverged", id, i)
			}
		}
	}
}

func TestC3BMemoryFingerprintProtocolSeparation(t *testing.T) {
	const configSHA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if got, want := v2MemoryEnvironmentFingerprintForProtocol(configSHA, benchTransportHTTPS), v2MemoryEnvironmentFingerprint(configSHA); got != want {
		t.Fatalf("https fingerprint compatibility changed: got=%s want=%s", got, want)
	}
	seen := map[string]bool{}
	for _, protocol := range []string{benchTransportHTTPS, benchTransportHTTP, benchTransportQUIC, benchTransportSTUN} {
		fp := v2MemoryEnvironmentFingerprintForProtocol(configSHA, protocol)
		if fp == "" || seen[fp] {
			t.Fatalf("protocol=%s fingerprint is empty or collides", protocol)
		}
		seen[fp] = true
	}
}
