package main

import (
	"errors"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/transaction"
)

type fakeDNSPolicyActivationDriver struct {
	failures map[string]error
	calls    []string
}

func (f *fakeDNSPolicyActivationDriver) step(name string) error {
	f.calls = append(f.calls, name)
	if f.failures != nil {
		if err := f.failures[name]; err != nil {
			return err
		}
	}
	return nil
}

func failSteps(names ...string) map[string]error {
	out := make(map[string]error, len(names))
	for _, name := range names {
		out[name] = errors.New(name + " failed")
	}
	return out
}

func (f *fakeDNSPolicyActivationDriver) Precheck(_ []DNSPolicyRule) error {
	return f.step("precheck")
}

func (f *fakeDNSPolicyActivationDriver) Snapshot() (DNSPolicyRuntimeSnapshot, error) {
	if err := f.step("snapshot"); err != nil {
		return DNSPolicyRuntimeSnapshot{}, err
	}
	return DNSPolicyRuntimeSnapshot{Identity: "before-123", RuleCount: 0}, nil
}

func (f *fakeDNSPolicyActivationDriver) Validate(_ []DNSPolicyRule) error {
	return f.step("validate")
}

func (f *fakeDNSPolicyActivationDriver) Apply(_ []DNSPolicyRule) error {
	return f.step("apply")
}

func (f *fakeDNSPolicyActivationDriver) Verify(_ []DNSPolicyRule) error {
	return f.step("verify")
}

func (f *fakeDNSPolicyActivationDriver) Rollback(_ DNSPolicyRuntimeSnapshot) error {
	return f.step("rollback")
}

func (f *fakeDNSPolicyActivationDriver) VerifyRollback(_ DNSPolicyRuntimeSnapshot) error {
	return f.step("rollback-verify")
}

func testActivationRules() []DNSPolicyRule {
	return []DNSPolicyRule{
		{ID: "one", Priority: 10, Policy: "Policy1"},
	}
}

func requireTransactionState(t *testing.T, err error, want transaction.State) transaction.Manifest {
	t.Helper()
	manifest, ok := transaction.Extract(err)
	if !ok {
		t.Fatalf("transaction manifest missing from %v", err)
	}
	if manifest.State != want {
		t.Fatalf("state = %q, want %q; manifest=%#v", manifest.State, want, manifest)
	}
	return manifest
}

func TestDNSPolicyActivationEngineCommitsVerifiedApply(t *testing.T) {
	driver := &fakeDNSPolicyActivationDriver{}
	result, err := runDNSPolicyActivation("tx-success", testActivationRules(), driver)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Activated || result.Manifest.State != transaction.Committed {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []string{"precheck", "snapshot", "validate", "apply", "verify"}
	if len(driver.calls) != len(want) {
		t.Fatalf("calls = %#v", driver.calls)
	}
	for i := range want {
		if driver.calls[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q", i, driver.calls[i], want[i])
		}
	}
}

func TestDNSPolicyActivationEngineApplyFailureRollsBack(t *testing.T) {
	driver := &fakeDNSPolicyActivationDriver{failures: failSteps("apply")}
	result, err := runDNSPolicyActivation("tx-apply-fail", testActivationRules(), driver)
	if err == nil {
		t.Fatal("apply failure must return an error")
	}
	manifest := requireTransactionState(t, err, transaction.RolledBack)
	if result.Activated || result.Manifest.State != transaction.RolledBack {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(driver.calls) < 6 || driver.calls[len(driver.calls)-2] != "rollback" || driver.calls[len(driver.calls)-1] != "rollback-verify" {
		t.Fatalf("rollback sequence missing: %#v", driver.calls)
	}
	foundRecovery := false
	for _, evidence := range manifest.Evidence {
		if evidence.Status == transaction.EvidenceRecovered {
			foundRecovery = true
		}
	}
	if !foundRecovery {
		t.Fatalf("recovery evidence missing: %#v", manifest.Evidence)
	}
}

func TestDNSPolicyActivationEngineVerifyFailureRollsBack(t *testing.T) {
	driver := &fakeDNSPolicyActivationDriver{failures: failSteps("verify")}
	result, err := runDNSPolicyActivation("tx-verify-fail", testActivationRules(), driver)
	if err == nil {
		t.Fatal("verify failure must return an error")
	}
	requireTransactionState(t, err, transaction.RolledBack)
	if result.Activated || result.Manifest.State != transaction.RolledBack {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestDNSPolicyActivationEngineRollbackFailureIsAmbiguous(t *testing.T) {
	driver := &fakeDNSPolicyActivationDriver{failures: failSteps("verify", "rollback")}
	result, err := runDNSPolicyActivation("tx-rollback-fail", testActivationRules(), driver)
	if err == nil {
		t.Fatal("rollback failure must return an error")
	}
	requireTransactionState(t, err, transaction.Ambiguous)
	if result.Activated || result.Manifest.State != transaction.Ambiguous {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestDNSPolicyActivationEngineRollbackVerifyFailureIsAmbiguous(t *testing.T) {
	driver := &fakeDNSPolicyActivationDriver{failures: failSteps("verify", "rollback-verify")}
	result, err := runDNSPolicyActivation("tx-rollback-verify-fail", testActivationRules(), driver)
	if err == nil {
		t.Fatal("rollback verification failure must return an error")
	}
	requireTransactionState(t, err, transaction.Ambiguous)
	if result.Activated || result.Manifest.State != transaction.Ambiguous {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestDNSPolicyActivationEnginePrecheckFailureNeverApplies(t *testing.T) {
	driver := &fakeDNSPolicyActivationDriver{failures: failSteps("precheck")}
	result, err := runDNSPolicyActivation("tx-precheck-fail", testActivationRules(), driver)
	if err == nil {
		t.Fatal("precheck failure must return an error")
	}
	requireTransactionState(t, err, transaction.Failed)
	if result.Activated || result.Manifest.State != transaction.Failed {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(driver.calls) != 1 || driver.calls[0] != "precheck" {
		t.Fatalf("unexpected calls after precheck failure: %#v", driver.calls)
	}
}
