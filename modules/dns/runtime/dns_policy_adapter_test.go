package main

import (
	"errors"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/transaction"
)

type fakeDNSPolicyRuntimePrimitive struct {
	failures map[string]error
	calls    []string
}

func (f *fakeDNSPolicyRuntimePrimitive) step(name string) error {
	f.calls = append(f.calls, name)
	if f.failures != nil {
		if err := f.failures[name]; err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDNSPolicyRuntimePrimitive) SnapshotRuntime() (DNSPolicyRuntimeSnapshot, error) {
	if err := f.step("snapshot-runtime"); err != nil {
		return DNSPolicyRuntimeSnapshot{}, err
	}
	return DNSPolicyRuntimeSnapshot{Identity: "runtime-before", RuleCount: 0}, nil
}

func (f *fakeDNSPolicyRuntimePrimitive) ApplyCanonicalRules(_ []DNSPolicyRule) error {
	return f.step("apply-canonical")
}

func (f *fakeDNSPolicyRuntimePrimitive) VerifyCanonicalRules(_ []DNSPolicyRule) error {
	return f.step("verify-canonical")
}

func (f *fakeDNSPolicyRuntimePrimitive) RestoreRuntime(_ DNSPolicyRuntimeSnapshot) error {
	return f.step("restore-runtime")
}

func (f *fakeDNSPolicyRuntimePrimitive) VerifyRuntimeSnapshot(_ DNSPolicyRuntimeSnapshot) error {
	return f.step("verify-snapshot")
}

func (f *fakeDNSPolicyRuntimePrimitive) RuntimeHealth() error {
	return f.step("runtime-health")
}

func TestDNSPolicyRuntimeAdapterDiscoveryBlocksProductionDriver(t *testing.T) {
	discovery := discoverDNSPolicyRuntimeAdapter()
	if discovery.ProductionDriverReady {
		t.Fatal("production driver must remain blocked until final hardware acceptance is complete")
	}
	if len(discovery.KnownPrimitives) < 6 {
		t.Fatalf("known primitives incomplete: %#v", discovery.KnownPrimitives)
	}
	for _, primitive := range discovery.KnownPrimitives {
		if !primitive.Available || primitive.Name == "" || primitive.Evidence == "" {
			t.Fatalf("invalid discovered primitive: %#v", primitive)
		}
	}
	if len(discovery.BlockingUnknowns) != 2 {
		t.Fatalf("blocking unknowns = %#v", discovery.BlockingUnknowns)
	}
}

func TestDNSPolicyActivationAdapterMapsEngineToRuntimeContract(t *testing.T) {
	primitive := &fakeDNSPolicyRuntimePrimitive{}
	validateCalls := 0
	adapter := newDNSPolicyActivationAdapter(primitive, func(rules []DNSPolicyRule) error {
		validateCalls++
		if len(rules) != 1 {
			return errors.New("unexpected rule count")
		}
		return nil
	})

	result, err := runDNSPolicyActivation("tx-adapter-success", testActivationRules(), adapter)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Activated || result.Manifest.State != transaction.Committed {
		t.Fatalf("unexpected result: %#v", result)
	}
	if validateCalls != 2 {
		t.Fatalf("validator calls = %d, want 2", validateCalls)
	}
	want := []string{
		"snapshot-runtime",
		"runtime-health",
		"apply-canonical",
		"verify-canonical",
		"runtime-health",
	}
	if len(primitive.calls) != len(want) {
		t.Fatalf("calls = %#v", primitive.calls)
	}
	for i := range want {
		if primitive.calls[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q", i, primitive.calls[i], want[i])
		}
	}
}

func TestDNSPolicyActivationAdapterVerifyFailureRestoresSnapshot(t *testing.T) {
	primitive := &fakeDNSPolicyRuntimePrimitive{
		failures: map[string]error{"verify-canonical": errors.New("readback mismatch")},
	}
	adapter := newDNSPolicyActivationAdapter(primitive, func([]DNSPolicyRule) error { return nil })

	result, err := runDNSPolicyActivation("tx-adapter-rollback", testActivationRules(), adapter)
	if err == nil {
		t.Fatal("verify failure must return an error")
	}
	manifest, ok := transaction.Extract(err)
	if !ok || manifest.State != transaction.RolledBack {
		t.Fatalf("expected rolled-back transaction, got %#v / %v", manifest, err)
	}
	if result.Activated || result.Manifest.State != transaction.RolledBack {
		t.Fatalf("unexpected result: %#v", result)
	}
	wantTail := []string{"verify-canonical", "restore-runtime", "verify-snapshot", "runtime-health"}
	if len(primitive.calls) < len(wantTail) {
		t.Fatalf("calls = %#v", primitive.calls)
	}
	tail := primitive.calls[len(primitive.calls)-len(wantTail):]
	for i := range wantTail {
		if tail[i] != wantTail[i] {
			t.Fatalf("tail[%d] = %q, want %q; calls=%#v", i, tail[i], wantTail[i], primitive.calls)
		}
	}
}

func TestDNSPolicyActivationAdapterHealthFailureBeforeApplyNeverMutates(t *testing.T) {
	primitive := &fakeDNSPolicyRuntimePrimitive{
		failures: map[string]error{"runtime-health": errors.New("runtime unhealthy")},
	}
	adapter := newDNSPolicyActivationAdapter(primitive, func([]DNSPolicyRule) error { return nil })

	result, err := runDNSPolicyActivation("tx-adapter-health", testActivationRules(), adapter)
	if err == nil {
		t.Fatal("health failure must return an error")
	}
	manifest, ok := transaction.Extract(err)
	if !ok || manifest.State != transaction.Failed {
		t.Fatalf("expected failed transaction, got %#v / %v", manifest, err)
	}
	if result.Activated {
		t.Fatalf("activation must remain false: %#v", result)
	}
	for _, call := range primitive.calls {
		if call == "apply-canonical" {
			t.Fatalf("apply called after validation health failure: %#v", primitive.calls)
		}
	}
}
