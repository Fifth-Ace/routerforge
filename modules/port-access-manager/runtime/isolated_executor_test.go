package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type recordedRunner struct {
	calls  [][]string
	failAt int
}

func (r *recordedRunner) Run(_ context.Context, cmd []string) error {
	r.calls = append(r.calls, append([]string(nil), cmd...))
	if r.failAt > 0 && len(r.calls) == r.failAt {
		return errors.New("simulated failure")
	}
	return nil
}
func exercisePlan(t *testing.T) KnockRules {
	t.Helper()
	p, err := buildKnockRules(KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestIsolatedExecutorRollbackSuccess(t *testing.T) {
	p := exercisePlan(t)
	r := &recordedRunner{}
	e := isolatedExecutor{runner: r, timeout: time.Second}
	calls := 0
	if err := e.exercise(context.Background(), p, func(context.Context) error { calls++; return nil }); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(r.calls) != len(p.Commands)+len(buildKnockRollback()) {
		t.Fatalf("calls=%d length=%d", calls, len(r.calls))
	}
	if r.calls[len(p.Commands)][3] != "-F" {
		t.Fatal("rollback not executed")
	}
}
func TestIsolatedExecutorRollbackOnStageError(t *testing.T) {
	p := exercisePlan(t)
	r := &recordedRunner{failAt: 2}
	e := isolatedExecutor{runner: r, timeout: time.Second}
	err := e.exercise(context.Background(), p, func(context.Context) error { t.Fatal("must not verify failed stage"); return nil })
	if err == nil || len(r.calls) != 4 {
		t.Fatalf("error=%v calls=%d", err, len(r.calls))
	}
}
func TestIsolatedExecutorRollbackOnVerificationError(t *testing.T) {
	p := exercisePlan(t)
	r := &recordedRunner{}
	e := isolatedExecutor{runner: r, timeout: time.Second}
	err := e.exercise(context.Background(), p, func(context.Context) error { return errors.New("verification rejected") })
	if err == nil || !strings.Contains(err.Error(), "verification rejected") || len(r.calls) != len(p.Commands)+len(buildKnockRollback()) {
		t.Fatalf("err=%v calls=%d", err, len(r.calls))
	}
}
func TestIsolatedExecutorRejectsExternalHooks(t *testing.T) {
	p := exercisePlan(t)
	p.Commands = append(p.Commands, []string{"iptables", "-t", "filter", "-I", "FORWARD", "1", "-j", "RF_PORT_KNOCK"})
	r := &recordedRunner{}
	err := (isolatedExecutor{runner: r, timeout: time.Second}).exercise(context.Background(), p, func(context.Context) error { return nil })
	if err == nil || len(r.calls) != 0 {
		t.Fatal("unsafe hook accepted")
	}
}
func TestIsolatedExecutorRejectsEnabledPlan(t *testing.T) {
	p := exercisePlan(t)
	p.DeploymentBlocked = false
	r := &recordedRunner{}
	err := (isolatedExecutor{runner: r, timeout: time.Second}).exercise(context.Background(), p, func(context.Context) error { return nil })
	if err == nil || len(r.calls) != 0 {
		t.Fatal("enabled plan accepted")
	}
}
