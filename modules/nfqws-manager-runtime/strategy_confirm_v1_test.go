package main

import "testing"

func nf3b2Candidate(id string, ok, clean, infra bool) v2CandidateResult {
	c := v2CandidateResult{CandidateID: id, Attempts: []v2BenchAttempt{{
		OK: ok, CleanupProven: clean, InfrastructureOK: infra,
		StrategyPathExercised: infra,
		Metrics:               v2HTTPMetrics{ResponseComplete: ok},
	}}}
	v2FinalizeCandidate(&c)
	return c
}

func TestNF3B2SelectsOnlySoundSinglePassSuccesses(t *testing.T) {
	rows := []v2CandidateResult{
		nf3b2Candidate("bad", false, true, true),
		nf3b2Candidate("good1", true, true, true),
		nf3b2Candidate("unclean", true, false, true),
		nf3b2Candidate("good2", true, true, true),
		nf3b2Candidate("good3", true, true, true),
	}
	indices := v2ConfirmationShortlist(rows, v2ConfirmMaxCandidates)
	if len(indices) != 2 || indices[0] != 1 || indices[1] != 3 {
		t.Fatalf("shortlist=%v", indices)
	}
}

func TestNF3B2SinglePassCannotProduceRecommendationOrReusableMemory(t *testing.T) {
	baseline := nf3b2Candidate("baseline", false, true, true)
	one := nf3b2Candidate("one", true, true, true)
	ok, best, needed, _ := v2ChooseConfirmedRecommendation(baseline, []v2CandidateResult{one})
	if ok || best != nil || needed {
		t.Fatalf("single pass recommendation: ok=%v best=%+v needed=%v", ok, best, needed)
	}
	if got := v2ConfirmedMemoryEvidence([]v2CandidateResult{one}); len(got) != 0 {
		t.Fatalf("unconfirmed candidate entered memory: %+v", got)
	}
}

func TestNF3B2TwoIndependentSuccessesAllowRecommendation(t *testing.T) {
	baseline := nf3b2Candidate("baseline", false, true, true)
	repeated := nf3b2Candidate("repeated", true, true, true)
	repeated.Attempts = append(repeated.Attempts, v2BenchAttempt{OK: true, InfrastructureOK: true,
		CleanupProven: true, StrategyPathExercised: true,
		Metrics: v2HTTPMetrics{ResponseComplete: true}})
	v2FinalizeCandidate(&repeated)
	ok, best, needed, _ := v2ChooseConfirmedRecommendation(baseline, []v2CandidateResult{repeated})
	if !ok || best == nil || !needed || best.CandidateID != "repeated" || best.Successes != 2 {
		t.Fatalf("confirmation missing: ok=%v best=%+v needed=%v", ok, best, needed)
	}
	if got := v2ConfirmedMemoryEvidence([]v2CandidateResult{repeated}); len(got) != 1 {
		t.Fatalf("confirmed candidate excluded from memory: %+v", got)
	}
}

func TestNF3B2FlakyOrUncleanConfirmationNeverRecommends(t *testing.T) {
	baseline := nf3b2Candidate("baseline", false, true, true)
	for _, tc := range []struct {
		name      string
		ok, clean bool
	}{
		{"flaky", false, true},
		{"unclean", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := nf3b2Candidate(tc.name, true, true, true)
			c.Attempts = append(c.Attempts, v2BenchAttempt{OK: tc.ok, InfrastructureOK: true,
				CleanupProven: tc.clean, StrategyPathExercised: true,
				Metrics: v2HTTPMetrics{ResponseComplete: tc.ok}})
			v2FinalizeCandidate(&c)
			ok, best, _, _ := v2ChooseConfirmedRecommendation(baseline, []v2CandidateResult{c})
			if ok || best != nil {
				t.Fatalf("unstable/unclean confirmation recommended: %+v", c)
			}
		})
	}
}

func TestNF3B2HealthyBaselineNeverNeedsBypass(t *testing.T) {
	baseline := nf3b2Candidate("baseline", true, true, true)
	candidate := nf3b2Candidate("candidate", true, true, true)
	candidate.Attempts = append(candidate.Attempts, candidate.Attempts[0])
	v2FinalizeCandidate(&candidate)
	ok, _, needed, _ := v2ChooseConfirmedRecommendation(baseline, []v2CandidateResult{candidate})
	if ok || needed {
		t.Fatal("working baseline must suppress bypass recommendation")
	}
}
