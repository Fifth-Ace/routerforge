package main

import "testing"

func TestProgressiveVerifyAttempts(t *testing.T) {
	cases := map[string]int{"fast": 2, "normal": 2, "thorough": 3}
	for name, want := range cases {
		mode, err := v2SelectorMode(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := v2ProgressiveVerifyAttempts(mode); got != want {
			t.Fatalf("mode=%s verify=%d want=%d", name, got, want)
		}
	}
}

func TestProgressiveHTTPSMemoryEnvironmentIsBackwardCompatible(t *testing.T) {
	const configSHA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	legacy := v2MemoryEnvironmentFingerprint(configSHA)
	progressive := v2ProgressiveMemoryEnvironmentFingerprint(configSHA, benchTransportHTTPS)
	if progressive != legacy {
		t.Fatalf("https environment fingerprint changed: got=%s want=%s", progressive, legacy)
	}
	if stun := v2ProgressiveMemoryEnvironmentFingerprint(configSHA, benchTransportSTUN); stun == legacy {
		t.Fatal("STUN environment fingerprint must differ from HTTPS")
	}
}

func TestProgressiveBuiltinsCompileForTheirTransport(t *testing.T) {
	for _, id := range []string{benchTransportHTTPS, benchTransportHTTP, benchTransportQUIC, benchTransportSTUN} {
		transport, err := normalizeBenchTransport(id)
		if err != nil {
			t.Fatal(err)
		}
		builtins := v2ProgressiveBuiltins(transport)
		if len(builtins) == 0 {
			t.Fatalf("transport=%s has no progressive builtins", id)
		}
		for _, item := range builtins {
			profile, err := v2CustomProfileForTransport(item.Args, "example.com", transport)
			if err != nil {
				t.Fatalf("transport=%s builtin=%s: %v", id, item.ID, err)
			}
			if !profile.CandidateEligible {
				t.Fatalf("transport=%s builtin=%s is not eligible: %v", id, item.ID, profile.Reasons)
			}
			if id == benchTransportSTUN {
				for _, arg := range profile.Args {
					if len(arg) >= len("--hostlist") && arg[:len("--hostlist")] == "--hostlist" {
						t.Fatalf("STUN builtin retained domain selection arg %q", arg)
					}
				}
			}
		}
	}
}

func TestProgressiveStableRequiresRepeatedSuccess(t *testing.T) {
	working := v2BenchAttempt{
		OK: true, InfrastructureOK: true, CleanupProven: true,
		Transport: benchTransportHTTPS,
		Metrics:   v2HTTPMetrics{TLSComplete: true, ProgressProven: true, ResponseComplete: true},
	}
	candidate := v2CandidateResult{Attempts: []v2BenchAttempt{working}}
	v2FinalizeCandidate(&candidate)
	if v2ProgressiveStable(candidate, 2) {
		t.Fatal("single success must not satisfy repeated stability")
	}
	candidate.Attempts = append(candidate.Attempts, working)
	v2FinalizeCandidate(&candidate)
	if !v2ProgressiveStable(candidate, 2) {
		t.Fatalf("two clean successes should be stable: %+v", candidate)
	}
}

func TestProgressivePlanDeduplicatesAcrossStages(t *testing.T) {
	plan := v2ProgressivePlan{
		Stages:   map[string][]v2ProgressiveTemplate{},
		Warnings: []string{}, ByFingerprint: map[string]v2ProgressiveTemplate{},
	}
	for _, stage := range v2ProgressiveStageOrder {
		plan.Stages[stage] = []v2ProgressiveTemplate{}
	}
	seen := map[string]bool{}
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	profile, err := v2CustomProfileForTransport(v2BuiltinHTTPSCandidates[0].Args, "example.com", transport)
	if err != nil {
		t.Fatal(err)
	}
	first := v2ProgressiveTemplate{Stage: v2ProgressiveStageMemory, Profile: profile, ID: "m1", Name: "memory", Source: "memory"}
	second := v2ProgressiveTemplate{Stage: v2ProgressiveStageQuick, Profile: profile, ID: "b1", Name: "builtin", Source: "builtin"}
	if !v2ProgressiveAppendTemplate(&plan, seen, 8, first) {
		t.Fatal("first candidate should be added")
	}
	if v2ProgressiveAppendTemplate(&plan, seen, 8, second) {
		t.Fatal("same technique must be deduplicated across stages")
	}
	if len(plan.Stages[v2ProgressiveStageMemory]) != 1 || len(plan.Stages[v2ProgressiveStageQuick]) != 0 {
		t.Fatalf("unexpected plan stage counts: %+v", plan.Stages)
	}
}

func TestProgressiveRecommendationRejectsUnverifiedSingleSuccess(t *testing.T) {
	transport, _ := normalizeBenchTransport(benchTransportSTUN)
	baseline := v2CandidateResult{Attempts: []v2BenchAttempt{{
		OK: false, InfrastructureOK: true, CleanupProven: true,
		Transport: benchTransportSTUN,
	}}}
	v2FinalizeCandidate(&baseline)
	candidate := v2CandidateResult{CandidateID: "one-shot", Attempts: []v2BenchAttempt{{
		OK: true, InfrastructureOK: true, CleanupProven: true,
		Transport: benchTransportSTUN,
		Metrics:   v2HTTPMetrics{STUNResponseProven: true, STUNTransactionMatched: true, ResponseComplete: true, ProgressProven: true},
	}}}
	v2FinalizeCandidate(&candidate)
	recommend, best, needed, _ := v2ProgressiveChooseRecommendation(baseline, []v2CandidateResult{candidate}, 2, transport)
	if recommend || best != nil || needed {
		t.Fatal("single successful attempt must not produce a progressive recommendation")
	}
}
