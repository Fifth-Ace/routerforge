package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func nf3b3FakeParent(id string, metrics v2HTTPMetrics, ok bool, fake string) v2CandidateResult {
	c := v2CandidateResult{
		CandidateID: id, CandidateName: id,
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", fake, "--lua-desync=multisplit:pos=1,midsld"},
		Attempts: []v2BenchAttempt{{
			OK: ok, InfrastructureOK: true, CleanupProven: true, StrategyPathExercised: true, Metrics: metrics,
		}},
	}
	v2FinalizeCandidate(&c)
	return c
}

func nf3b3PartialFake(id string) v2CandidateResult {
	return nf3b3FakeParent(id, v2HTTPMetrics{Cutoff16KSuspected: true, TLSComplete: true, Bytes: 16384}, false, "--lua-desync=fake:blob=tls_clienthello:tcp_ack=-66000")
}

func TestNF3B3LiveFakeAxesAndProvenance(t *testing.T) {
	parent := nf3b3PartialFake("partial")
	if parent.ResultClass != "PARTIAL" {
		t.Fatalf("fixture result=%s", parent.ResultClass)
	}
	plans := v2RefinePlans([]v2CandidateResult{parent}, "example.org")
	if len(plans) != 4 {
		t.Fatalf("expected four bounded upstream fake variants, got %d", len(plans))
	}
	seen := map[string]bool{}
	for _, p := range plans {
		if p.ParentID != "partial" || p.CandidateID == "" || p.ChangedParameter == "" || p.Profile.Index != -1 {
			t.Fatalf("broken lineage: %+v", p)
		}
		fp := v2CandidateTechniqueFingerprint(p.Profile.Args)
		if fp == "" || seen[fp] {
			t.Fatalf("duplicate fingerprint: %q", fp)
		}
		seen[fp] = true
		if p.Profile.Args[len(p.Profile.Args)-1] != parent.Args[len(parent.Args)-1] {
			t.Fatal("non-fake Lua desync was modified")
		}
	}
	for i, want := range []string{"fake.fool=badsum", "fake.fool=tcp_md5", "fake.repeats=2", "fake.repeats=4"} {
		if plans[i].ChangedParameter != want {
			t.Fatalf("axis %d = %q, want %q", i, plans[i].ChangedParameter, want)
		}
	}
}

func TestNF3B3NoBlindMutationOrUntrustedShortcuts(t *testing.T) {
	for _, fake := range []string{
		"--lua-desync=multisplit:pos=1,midsld", // not a fake
		"--lua-desync=fake:repeats=2",          // no blob
		"--lua-desync=fake:blob=x:blob=y",      // ambiguous
		"--lua-desync=fake:blob=x:repeats=2:repeats=4",
	} {
		c := nf3b3FakeParent("invalid", v2HTTPMetrics{Cutoff16KSuspected: true}, false, fake)
		if got := v2RefinePlans([]v2CandidateResult{c}, "example.org"); len(got) != 0 {
			t.Fatalf("invalid fake %q generated %d plans", fake, len(got))
		}
	}
	failed := nf3b3FakeParent("no-progress", v2HTTPMetrics{}, false, "--lua-desync=fake:blob=tls_clienthello")
	if got := v2RefinePlans([]v2CandidateResult{failed}, "example.org"); len(got) != 0 {
		t.Fatal("no live progress produced mutation")
	}
	unproven := nf3b3PartialFake("not-tested")
	unproven.Attempts[0].StrategyPathExercised = false
	if got := v2RefinePlans([]v2CandidateResult{unproven}, "example.org"); len(got) != 0 {
		t.Fatal("unproven strategy path produced mutation")
	}
}

func TestNF3B3FoolAxisReplacesOnlyFoolAndRepeatAxisPreservesIt(t *testing.T) {
	original := "--lua-desync=fake:blob=tls_clienthello:tcp_ack=-66000:repeats=2"
	fool, ok := v2RefineFakeArg(original, v2RefineAxis{"fool", "badsum"})
	if !ok || strings.Contains(fool, "tcp_ack") || !strings.Contains(fool, ":badsum") || !strings.Contains(fool, ":repeats=2") {
		t.Fatalf("incorrect fake fool substitution: %s", fool)
	}
	repeats, ok := v2RefineFakeArg(original, v2RefineAxis{"repeats", "4"})
	if !ok || !strings.Contains(repeats, ":tcp_ack=-66000") || !strings.Contains(repeats, ":repeats=4") {
		t.Fatalf("incorrect repeats substitution: %s", repeats)
	}
	if _, ok = v2RefineFakeArg(original, v2RefineAxis{"repeats", "2"}); ok {
		t.Fatal("unchanged axis was emitted")
	}
	if _, ok = v2RefineFakeArg(original, v2RefineAxis{"fool", "unsafe"}); ok {
		t.Fatal("unapproved Lua argument accepted")
	}
}

func TestNF3B3MeasuredParentSelection(t *testing.T) {
	weaker := nf3b3FakeParent("weaker", v2HTTPMetrics{TLSComplete: true, Bytes: 10}, false, "--lua-desync=fake:blob=tls_clienthello")
	stronger := nf3b3PartialFake("stronger")
	plans := v2RefinePlans([]v2CandidateResult{weaker, stronger}, "example.org")
	if len(plans) == 0 || plans[0].ParentID != "stronger" {
		t.Fatalf("wrong measured parent selected: %+v", plans)
	}
	alreadyTwo := nf3b3FakeParent("confirmed", v2HTTPMetrics{ResponseComplete: true}, true, "--lua-desync=fake:blob=tls_clienthello")
	alreadyTwo.Attempts = append(alreadyTwo.Attempts, alreadyTwo.Attempts[0])
	v2FinalizeCandidate(&alreadyTwo)
	if got := v2RefinePlans([]v2CandidateResult{stronger, alreadyTwo}, "example.org"); len(got) != 0 {
		t.Fatal("twice-confirmed NF3B2 winner did not stop refinement")
	}
}

func TestNF3B3DuplicatesAndCandidateCap(t *testing.T) {
	parent := nf3b3PartialFake("partial")
	other := parent
	other.Args = append([]string(nil), parent.Args...)
	other.Args[3] = "--lua-desync=fake:blob=tls_clienthello:badsum:repeats=4"
	plans := v2RefinePlans([]v2CandidateResult{parent, other}, "example.org")
	for _, p := range plans {
		if v2CandidateTechniqueFingerprint(p.Profile.Args) == v2CandidateTechniqueFingerprint(other.Args) {
			t.Fatal("already-tested technique was regenerated")
		}
	}
	large := make([]v2CandidateResult, v2DirectSelectorMaxCandidates)
	for i := range large {
		large[i] = parent
	}
	if got := v2RefinePlans(large, "example.org"); len(got) != 0 {
		t.Fatal("selector cap exceeded")
	}
	if got := v2RefinePlans(large[:len(large)-1], "example.org"); len(got) != 1 {
		t.Fatalf("expected exactly one free slot, got %d", len(got))
	}
}

func TestNF3B3RefinedWinnerNeedsTwoRealSuccesses(t *testing.T) {
	plan := v2RefinePlans([]v2CandidateResult{nf3b3PartialFake("partial")}, "example.org")[:1]
	calls := 0
	result, err := v2RunRefinePlans(context.Background(), plan, func(_ context.Context, _ benchStrategyProfile) (v2BenchAttempt, error) {
		calls++
		return v2BenchAttempt{OK: true, InfrastructureOK: true, CleanupProven: true, StrategyPathExercised: true,
			Metrics: v2HTTPMetrics{ResponseComplete: true}}, nil
	})
	if err != nil || calls != 2 || len(result) != 1 || result[0].Successes != 2 || result[0].LiveResult != "WORKING" || result[0].DerivedFrom != "partial" {
		t.Fatalf("confirmation failure: calls=%d, result=%+v, err=%v", calls, result, err)
	}
	_, winner, _, _ := v2ChooseConfirmedRecommendation(nf3b3FakeParent("baseline", v2HTTPMetrics{}, false, "--lua-desync=fake:blob=x"), result)
	if winner == nil || winner.CandidateID != plan[0].CandidateID {
		t.Fatal("twice-proven refinement is not eligible")
	}
}

func TestNF3B3SingleFailureNeverRecommends(t *testing.T) {
	plan := v2RefinePlans([]v2CandidateResult{nf3b3PartialFake("partial")}, "example.org")[:1]
	calls := 0
	result, err := v2RunRefinePlans(context.Background(), plan, func(_ context.Context, _ benchStrategyProfile) (v2BenchAttempt, error) {
		calls++
		return v2BenchAttempt{InfrastructureOK: true, CleanupProven: true, StrategyPathExercised: true}, nil
	})
	if err != nil || calls != 1 || len(result) != 1 || result[0].ResultClass == "WORKING" {
		t.Fatalf("failed candidate retried: calls=%d result=%+v err=%v", calls, result, err)
	}
}

func TestNF3B3FailClosedOnCleanupCancelAndBudget(t *testing.T) {
	plans := v2RefinePlans([]v2CandidateResult{nf3b3PartialFake("partial")}, "example.org")
	calls := 0
	got, err := v2RunRefinePlans(context.Background(), plans, func(_ context.Context, _ benchStrategyProfile) (v2BenchAttempt, error) {
		calls++
		return v2BenchAttempt{OK: true, InfrastructureOK: true, CleanupProven: false}, nil
	})
	if err == nil || calls != 1 || len(got) != 1 {
		t.Fatalf("unclean worker did not stop: calls=%d err=%v", calls, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	_, err = v2RunRefinePlans(ctx, plans, func(_ context.Context, _ benchStrategyProfile) (v2BenchAttempt, error) {
		calls++
		return v2BenchAttempt{}, nil
	})
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("cancel not respected: calls=%d err=%v", calls, err)
	}
	if _, err := v2RunRefinePlans(context.Background(), append(plans, plans[0]), nil); err == nil {
		t.Fatal("too many plans accepted")
	}
}
