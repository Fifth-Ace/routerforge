package main

import "testing"

func TestV2GenericWinnerApplyEligibleRejectsUnstable(t *testing.T) {
	best := &v2CandidateResult{
		ResultClass:      "UNSTABLE",
		SuccessRate:      0.5,
		CleanupProven:    true,
		InfrastructureOK: true,
		Args: []string{
			"--filter-tcp=443",
			"--filter-l7=tls",
			"--payload=tls_client_hello",
			"--lua-desync=multisplit:pos=1",
		},
	}
	if err := v2GenericWinnerApplyEligible(best); err == nil {
		t.Fatal("unstable live winner became Apply eligible")
	}
}

func TestV2GenericWinnerApplyEligibleRejectsSingleWorkingAttempt(t *testing.T) {
	attempt := v2BenchAttempt{OK: true, InfrastructureOK: true, CleanupProven: true}
	best := &v2CandidateResult{
		ResultClass:      "WORKING",
		SuccessRate:      1,
		Successes:        1,
		Attempts:         []v2BenchAttempt{attempt},
		CleanupProven:    true,
		InfrastructureOK: true,
		Args: []string{
			"--filter-tcp=443",
			"--filter-l7=tls",
			"--payload=tls_client_hello",
			"--lua-desync=multisplit:pos=1",
		},
	}
	if err := v2GenericWinnerApplyEligible(best); err == nil {
		t.Fatal("single successful live attempt became Apply eligible")
	}
}

func TestV2GenericWinnerApplyEligibleAcceptsRepeatedWorking(t *testing.T) {
	attempt := v2BenchAttempt{OK: true, InfrastructureOK: true, CleanupProven: true}
	best := &v2CandidateResult{
		ResultClass:      "WORKING",
		SuccessRate:      1,
		Successes:        2,
		Attempts:         []v2BenchAttempt{attempt, attempt},
		CleanupProven:    true,
		InfrastructureOK: true,
		Args: []string{
			"--filter-tcp=443",
			"--filter-l7=tls",
			"--payload=tls_client_hello",
			"--lua-desync=multisplit:pos=1",
		},
	}
	if err := v2GenericWinnerApplyEligible(best); err != nil {
		t.Fatalf("repeated working live winner rejected: %v", err)
	}
}

func TestV2CopyStringMapIsIndependent(t *testing.T) {
	src := map[string]string{"user.list": "abc"}
	got := v2CopyStringMap(src)
	got["user.list"] = "def"
	if src["user.list"] != "abc" {
		t.Fatal("receipt dependency map aliases source map")
	}
}
