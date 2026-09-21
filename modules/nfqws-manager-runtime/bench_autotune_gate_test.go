package main

import "testing"

func TestApplyGateStatusZeroProfileIdentity(t *testing.T) {
	status := benchAutoTuneApplyGateStatus{
		Eligible:           true,
		ServerName:         "www.googlevideo.com",
		SourceProfileIndex: 0,
		ConfigSHA256:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Reason:             "test",
	}
	if !status.Eligible || status.SourceProfileIndex != 0 {
		t.Fatalf("status=%+v", status)
	}
}

func TestStoreApplyPlanForGenericCandidateSeparatesSourceAndCandidate(t *testing.T) {
	clearBenchAutoTuneApplyPlan()
	defer clearBenchAutoTuneApplyPlan()
	source := analyzeBenchStrategyProfile(2, []string{
		"--hostlist=/opt/etc/nfqws2/lists/user.list",
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	})
	candidate, err := v2BindCandidateToSourceProfile(source, []string{
		"--hostlist-domains=example.com",
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=fakedsplit:pos=midsld",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := storeBenchAutoTuneApplyPlanForCandidate(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"example.com", "93.184.216.34", source, candidate, "builtin",
		v2CandidateTechniqueFingerprint(candidate),
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SourceProfileIndex != 2 || len(plan.SourceStrategyArgs) == 0 || len(plan.CandidateStrategyArgs) == 0 {
		t.Fatalf("plan identity incomplete: %+v", plan)
	}
	if stringSlicesEqual(plan.SourceStrategyArgs, plan.CandidateStrategyArgs) {
		t.Fatal("generic plan collapsed source and candidate identity")
	}
}
