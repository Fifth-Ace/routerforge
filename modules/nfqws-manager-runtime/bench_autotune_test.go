package main

import (
	"strings"
	"testing"
)

func TestParseBenchAutoTuneMode(t *testing.T) {
	fast, err := parseBenchAutoTuneMode("FAST")
	if err != nil || fast.Attempts != 1 || fast.MaxCandidates != 3 {
		t.Fatalf("fast=%+v err=%v", fast, err)
	}
	normal, err := parseBenchAutoTuneMode("normal")
	if err != nil || normal.Attempts != 2 || normal.MaxCandidates != 8 {
		t.Fatalf("normal=%+v err=%v", normal, err)
	}
	thorough, err := parseBenchAutoTuneMode("thorough")
	if err != nil || thorough.Attempts != 3 || thorough.MaxCandidates != 16 {
		t.Fatalf("thorough=%+v err=%v", thorough, err)
	}
	if _, err := parseBenchAutoTuneMode("turbo"); err == nil {
		t.Fatal("unknown mode accepted")
	}
}

func TestRetargetBenchStrategyProfileOnlyChangesDomain(t *testing.T) {
	profile := benchStrategyProfile{
		Index:             2,
		CandidateEligible: true,
		Args: []string{
			"--hostlist-domains=mobatek.net",
			"--filter-tcp=443",
			"--filter-l7=tls",
			"--payload=tls_client_hello",
			"--lua-desync=multisplit:pos=2",
		},
		HostlistDomains: []string{"mobatek.net"},
		StrategyTags:    []int{7},
	}
	got, err := retargetBenchStrategyProfile(profile, "www.googlevideo.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.Args[0] != "--hostlist-domains=www.googlevideo.com" {
		t.Fatalf("retarget=%q", got.Args[0])
	}
	if got.Args[1] != profile.Args[1] || got.Args[4] != profile.Args[4] {
		t.Fatalf("strategy args changed: %+v", got.Args)
	}
	if got.HostlistDomains[0] != "www.googlevideo.com" {
		t.Fatalf("domains=%v", got.HostlistDomains)
	}
}

func TestAutoTuneRecommendationRequiresImprovementOverBaseline(t *testing.T) {
	baseline := benchAutoTuneCandidate{Successes: 1, SuccessRate: 1, CleanupProven: true, MedianProbeMS: 100}
	candidates := []benchAutoTuneCandidate{
		{SourceProfileIndex: 0, Successes: 1, SuccessRate: 1, CleanupProven: true, MedianProbeMS: 50},
	}
	ok, _, needed, _ := chooseBenchAutoTuneRecommendation(baseline, candidates)
	if ok || needed {
		t.Fatal("strategy should not be recommended when baseline reachability is equally successful")
	}

	baseline.Successes = 0
	baseline.SuccessRate = 0
	ok, index, needed, _ := chooseBenchAutoTuneRecommendation(baseline, candidates)
	if !ok || !needed || index != 0 {
		t.Fatalf("recommendation ok=%v index=%d needed=%v", ok, index, needed)
	}
}

func TestFinalizeAutoTuneCandidate(t *testing.T) {
	candidate := benchAutoTuneCandidate{
		Attempts: []benchAutoTuneAttempt{
			{OK: true, ProbeDurationMS: 100, CleanupProven: true},
			{OK: true, ProbeDurationMS: 60, CleanupProven: true},
			{OK: false, CleanupProven: true},
		},
	}
	finalizeBenchAutoTuneCandidate(&candidate)
	if candidate.Successes != 2 || candidate.Failures != 1 {
		t.Fatalf("candidate=%+v", candidate)
	}
	if candidate.MedianProbeMS != 80 {
		t.Fatalf("median=%d", candidate.MedianProbeMS)
	}
}

func TestValidateBenchAutoTuneRequest(t *testing.T) {
	good := benchAutoTuneRequest{
		Mode:                 "fast",
		ServerName:           "www.googlevideo.com",
		ExpectedConfigSHA256: strings.Repeat("a", 64),
		Confirm:              benchAutoTuneConfirm,
	}
	if err := validateBenchAutoTuneRequest(good); err != nil {
		t.Fatalf("good request rejected: %v", err)
	}
	bad := good
	bad.Confirm = "WRONG"
	if err := validateBenchAutoTuneRequest(bad); err == nil {
		t.Fatal("wrong confirmation accepted")
	}
}
