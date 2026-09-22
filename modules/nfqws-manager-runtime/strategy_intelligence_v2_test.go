package main

import (
	"net/http"
	"testing"
	"time"
)

func TestV2SelectorBenchTimeoutScalesForLowerConcurrency(t *testing.T) {
	mode := benchAutoTuneMode{Name: "normal", Attempts: 2, MaxCandidates: 16, TimeoutSec: 180}
	base := v2SelectorBenchTimeout(mode, v2DefaultConcurrency())
	if base != 180*time.Second {
		t.Fatalf("default concurrency timeout=%v want=180s", base)
	}
	if v2DefaultConcurrency() >= 2 {
		if got := v2SelectorBenchTimeout(mode, 1); got != 360*time.Second {
			t.Fatalf("single-worker timeout=%v want=360s", got)
		}
	}
}

func TestV2DomainMatches(t *testing.T) {
	cases := []struct {
		target, candidate string
		want              bool
	}{
		{"youtube.com", "youtube.com", true},
		{"www.youtube.com", "youtube.com", true},
		{"notyoutube.com", "youtube.com", false},
		{"www.youtube.com", "*.youtube.com", true},
	}
	for _, tc := range cases {
		if got := v2DomainMatches(tc.target, tc.candidate); got != tc.want {
			t.Fatalf("v2DomainMatches(%q,%q)=%v want %v", tc.target, tc.candidate, got, tc.want)
		}
	}
}

func TestV2ExtractDomains(t *testing.T) {
	got := v2ExtractDomains("# x\nyoutube.com\nwww.youtube.com\nbad\n||discord.com^\n", 64)
	if len(got) != 3 {
		t.Fatalf("got %v", got)
	}
	if got[0] != "youtube.com" || got[1] != "www.youtube.com" || got[2] != "discord.com" {
		t.Fatalf("unexpected domains %v", got)
	}
}

func TestV2ClassifyDetectTLS(t *testing.T) {
	stages := map[string]v2StageResult{
		"dns": {State: "pass"}, "tcp": {State: "pass"},
		"tls": {State: "fail"}, "http": {State: "skipped"},
	}
	class, _ := v2ClassifyDetect(stages, v2HTTPMetrics{})
	if class != "tls_failure" {
		t.Fatalf("class=%s", class)
	}
}

func TestV2CandidateRankingReliabilityFirst(t *testing.T) {
	fastFlaky := v2CandidateResult{SourceProfileIndex: 1, SuccessRate: .5, CompleteRate: .5, MedianTTFBMS: 10, MedianThroughput: 999999}
	slowStable := v2CandidateResult{SourceProfileIndex: 2, SuccessRate: 1, CompleteRate: 1, MedianTTFBMS: 200, MedianThroughput: 1000, CleanupProven: true}
	if !v2CandidateBetter(slowStable, fastFlaky) {
		t.Fatal("stable candidate must outrank faster flaky candidate")
	}
}

func TestV2ChooseRecommendationBaselineWorking(t *testing.T) {
	baseline := v2CandidateResult{ResultClass: "WORKING", SuccessRate: 1, CompleteRate: 1}
	candidates := []v2CandidateResult{{SourceProfileIndex: 2, Successes: 1, SuccessRate: 1, CompleteRate: 1, CleanupProven: true}}
	ok, _, needed, _ := v2ChooseRecommendation(baseline, candidates)
	if ok || needed {
		t.Fatal("working baseline must not create bypass recommendation")
	}
}

func TestV2FreeBenchQueues(t *testing.T) {
	got := v2FreeBenchQueues([]int{30000, 30002}, 3)
	if len(got) != 3 || got[0] != 30001 || got[1] != 30003 || got[2] != 30004 {
		t.Fatalf("unexpected queues %v", got)
	}
}

func TestV2RoutesUseModuleABIPath(t *testing.T) {
	mux := http.NewServeMux()
	registerStrategyIntelligenceV2Routes(mux)

	for _, path := range []string{
		"/v1/v2/inspect-target",
		"/v1/v2/detect",
		"/v1/v2/target-sources",
		"/v1/v2/targets/resolve",
		"/v1/v2/bench",
		"/v1/v2/selector",
		"/v1/v2/selector-progress",
		"/v1/v2/strategies",
		"/v1/v2/strategies/save",
		"/v1/v2/strategies/delete",
		"/v1/v2/candidates",
		"/v1/v2/memory",
		"/v1/v2/memory/clear",
		"/v1/v2/observed-targets",
		"/v1/v2/observed-targets/scan",
		"/v1/v2/observed-targets/ignore",
		"/v1/v2/strategy-registry",
		"/v1/v2/tcp16-memory",
		"/v1/v2/tcp16-probe",
		"/v1/v2/property-probe",
		"/v1/v2/selector-progressive",
		"/v1/v2/bench-profiles",
	} {
		req, err := http.NewRequest(http.MethodGet, "http://unix"+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, pattern := mux.Handler(req)
		if pattern != path {
			t.Fatalf("route %q pattern=%q", path, pattern)
		}
	}

	req, err := http.NewRequest(http.MethodGet, "http://unix/v2/target-sources", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, pattern := mux.Handler(req)
	if pattern != "" {
		t.Fatalf("legacy non-ABI route unexpectedly registered: %q", pattern)
	}
}

func TestV2FinalizeCandidateSeparatesInfrastructureFailure(t *testing.T) {
	c := v2CandidateResult{
		Attempts: []v2BenchAttempt{{
			OK: false, CleanupProven: true, InfrastructureOK: false,
			StrategyPathExercised: false,
		}},
	}
	v2FinalizeCandidate(&c)
	if c.InfrastructureOK {
		t.Fatal("infrastructure failure was hidden")
	}
	if c.ResultClass != "INCONCLUSIVE" {
		t.Fatalf("result_class=%q want INCONCLUSIVE", c.ResultClass)
	}
}

func TestV2FinalizeCandidateKeepsLegitimateCandidateFailure(t *testing.T) {
	c := v2CandidateResult{
		Attempts: []v2BenchAttempt{{
			OK: false, CleanupProven: true, InfrastructureOK: true,
			StrategyPathExercised: true,
		}},
	}
	v2FinalizeCandidate(&c)
	if !c.InfrastructureOK {
		t.Fatal("legitimate candidate failure was mislabeled as infrastructure failure")
	}
	if c.ResultClass != "FAILED" {
		t.Fatalf("result_class=%q want FAILED", c.ResultClass)
	}
}

func TestV2ChooseRecommendationReturnsCandidateIdentity(t *testing.T) {
	baseline := v2CandidateResult{
		ResultClass: "FAILED", SuccessRate: 0, CompleteRate: 0,
		CleanupProven: true, InfrastructureOK: true,
	}
	candidates := []v2CandidateResult{{
		CandidateID: "s-abc", CandidateName: "Saved", CandidateSource: "custom",
		SourceProfileIndex: -1, Successes: 1, SuccessRate: 1, CompleteRate: 1,
		ResultClass: "WORKING", CleanupProven: true, InfrastructureOK: true,
	}}
	ok, best, needed, _ := v2ChooseRecommendation(baseline, candidates)
	if !ok || !needed || best == nil {
		t.Fatalf("recommendation missing: ok=%v needed=%v best=%+v", ok, needed, best)
	}
	if best.CandidateID != "s-abc" || best.CandidateSource != "custom" {
		t.Fatalf("candidate identity lost: %+v", best)
	}
}

func TestV2SelectorRequestRejectsBadSession(t *testing.T) {
	req := v2SelectorRequest{
		Mode: "fast", ServerName: "example.com",
		ExpectedConfigSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SessionID:            "../bad", Confirm: v2SelectorConfirm,
	}
	if validateV2SelectorRequest(req) == nil {
		t.Fatal("bad selector session was accepted")
	}
}

func TestV2CustomProfileCompilesAwaySelectionFiles(t *testing.T) {
	args := []string{
		"--hostlist=/opt/etc/nfqws2/lists/youtube.list",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=2",
	}
	profile, err := v2CustomProfile(args, "youtube.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range profile.Args {
		if arg == "--hostlist=/opt/etc/nfqws2/lists/youtube.list" {
			t.Fatal("file-bound selector leaked into isolated candidate")
		}
	}
	found := false
	for _, arg := range profile.Args {
		if arg == "--hostlist-domains=youtube.com" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("target hostlist-domain missing: %+v", profile.Args)
	}
}
