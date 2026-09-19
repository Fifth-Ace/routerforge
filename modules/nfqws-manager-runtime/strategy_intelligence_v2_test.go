package main

import "testing"

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
