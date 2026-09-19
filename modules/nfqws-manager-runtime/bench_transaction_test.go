package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type benchFakeOps struct {
	failAt       string
	events       []string
	deleteErrors bool
	cleanupError bool
}

func (f *benchFakeOps) fail(name string) error {
	f.events = append(f.events, name)
	if f.failAt == name {
		return errors.New("injected failure")
	}
	return nil
}

func (f *benchFakeOps) StartCandidate(_ context.Context, _ benchTransactionSpec) error {
	return f.fail("start")
}
func (f *benchFakeOps) InstallRule(_ context.Context, rule benchRuleSpec) error {
	return f.fail("install:" + rule.Name)
}
func (f *benchFakeOps) VerifyInstalled(_ context.Context, _ benchTransactionSpec, _ []benchRuleSpec) error {
	return f.fail("verify-installed")
}
func (f *benchFakeOps) Probe(_ context.Context, _ benchTransactionSpec) error {
	return f.fail("probe")
}
func (f *benchFakeOps) DeleteRule(_ context.Context, rule benchRuleSpec) error {
	f.events = append(f.events, "delete:"+rule.Name)
	if f.deleteErrors && rule.Name == "out-mark" {
		return errors.New("delete failure")
	}
	return nil
}
func (f *benchFakeOps) StopCandidate(_ context.Context, _ benchTransactionSpec) error {
	return f.fail("stop")
}
func (f *benchFakeOps) VerifyCleanup(_ context.Context, _ benchTransactionSpec) error {
	f.events = append(f.events, "verify-cleanup")
	if f.cleanupError {
		return errors.New("cleanup proof failure")
	}
	return nil
}

func benchTestSpec() benchTransactionSpec {
	return benchTransactionSpec{
		SessionID: "session-1234", DestinationIPv4: "203.0.113.10",
		LocalPort: 43123, Queue: 30000,
	}
}

func TestBenchRulePlanHasSixExactRules(t *testing.T) {
	got, err := buildBenchRulePlan(benchTestSpec())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 {
		t.Fatalf("rules=%d want=6", len(got))
	}
	wantNames := []string{"out-mark", "out-queue", "out-clear", "in-mark", "in-queue", "in-clear"}
	for i, name := range wantNames {
		if got[i].Name != name {
			t.Fatalf("rule[%d]=%q want=%q", i, got[i].Name, name)
		}
		if !strings.Contains(got[i].Comment, "routerforge-bench:session-1234:") {
			t.Fatalf("missing exact session rule identity: %+v", got[i])
		}
	}
	if got[0].Relation != "before" || got[0].Anchor != "nfqws_post" ||
		got[2].Relation != "after" || got[2].Anchor != "nfqws_post" ||
		got[3].Relation != "before" || got[3].Anchor != "nfqws_pre" ||
		got[5].Relation != "after" || got[5].Anchor != "nfqws_pre" {
		t.Fatalf("unexpected bracketing plan=%+v", got)
	}
}

func TestBenchTransactionSuccessAlwaysCleansReverseOrder(t *testing.T) {
	ops := &benchFakeOps{}
	got := runBenchTransaction(context.Background(), ops, benchTestSpec())

	if got.Error != "" || got.State != benchLifecycleStateClean || !got.CleanupAttempted || !got.CleanupProven {
		t.Fatalf("result=%+v", got)
	}
	wantTail := []string{
		"delete:in-clear",
		"delete:in-queue",
		"delete:in-mark",
		"delete:out-clear",
		"delete:out-queue",
		"delete:out-mark",
		"stop",
		"verify-cleanup",
	}
	if len(ops.events) < len(wantTail) {
		t.Fatalf("events=%v", ops.events)
	}
	tail := ops.events[len(ops.events)-len(wantTail):]
	if !reflect.DeepEqual(tail, wantTail) {
		t.Fatalf("cleanup events=%v want=%v", tail, wantTail)
	}
}

func TestBenchTransactionInstallFailureCleansOnlyAppliedRules(t *testing.T) {
	ops := &benchFakeOps{failAt: "install:in-mark"}
	got := runBenchTransaction(context.Background(), ops, benchTestSpec())

	if got.Error == "" || got.State != benchLifecycleStateClean || !got.CleanupProven {
		t.Fatalf("result=%+v", got)
	}
	wantDeletes := []string{"delete:out-clear", "delete:out-queue", "delete:out-mark"}
	var deletes []string
	for _, event := range ops.events {
		if strings.HasPrefix(event, "delete:") {
			deletes = append(deletes, event)
		}
	}
	if !reflect.DeepEqual(deletes, wantDeletes) {
		t.Fatalf("deletes=%v want=%v", deletes, wantDeletes)
	}
}

func TestBenchTransactionProbeFailureStillCleans(t *testing.T) {
	ops := &benchFakeOps{failAt: "probe"}
	got := runBenchTransaction(context.Background(), ops, benchTestSpec())

	if !strings.Contains(got.Error, "probe:") || got.State != benchLifecycleStateClean || !got.CleanupProven {
		t.Fatalf("result=%+v", got)
	}
}

func TestBenchTransactionCleanupFailureFailsClosed(t *testing.T) {
	ops := &benchFakeOps{cleanupError: true}
	got := runBenchTransaction(context.Background(), ops, benchTestSpec())

	if got.State != benchLifecycleStateCleanupUnproven || got.CleanupProven {
		t.Fatalf("result=%+v", got)
	}
	if len(got.CleanupErrors) == 0 {
		t.Fatal("expected cleanup proof error")
	}
}

func TestBenchTransactionRejectsInvalidSpecBeforeMutation(t *testing.T) {
	ops := &benchFakeOps{}
	spec := benchTestSpec()
	spec.Queue = 300

	got := runBenchTransaction(context.Background(), ops, spec)
	if got.State != benchLifecycleStateFailed || got.CleanupAttempted {
		t.Fatalf("result=%+v", got)
	}
	if len(ops.events) != 0 {
		t.Fatalf("mutation happened before validation: %v", ops.events)
	}
}
func TestBenchTransactionRejectsPrivateDestinationBeforeMutation(t *testing.T) {
	ops := &benchFakeOps{}
	spec := benchTestSpec()
	spec.DestinationIPv4 = "192.168.1.1"

	got := runBenchTransaction(context.Background(), ops, spec)
	if got.State != benchLifecycleStateFailed || got.CleanupAttempted {
		t.Fatalf("result=%+v", got)
	}
	if len(ops.events) != 0 {
		t.Fatalf("mutation happened before destination validation: %v", ops.events)
	}
}

func TestBenchTransactionContractControlledSmokeOnly(t *testing.T) {
	got := buildBenchTransactionContract()
	if !got.MutationEnabled || !got.ControlledSmokeOnly || !got.SystemMutatorImplemented {
		t.Fatalf("contract=%+v", got)
	}
	if got.ProductionConfigMutation || got.ProductionRestart {
		t.Fatalf("controlled smoke must not mutate production config/runtime: %+v", got)
	}
}
