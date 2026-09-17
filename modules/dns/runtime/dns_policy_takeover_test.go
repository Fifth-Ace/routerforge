package main

import (
	"errors"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/transaction"
)

type fakeDNSPolicyTakeoverPrimitive struct {
	calls    []string
	failures map[string]error
}

func (f *fakeDNSPolicyTakeoverPrimitive) step(name string) error {
	f.calls = append(f.calls, name)
	if f.failures != nil {
		if err := f.failures[name]; err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDNSPolicyTakeoverPrimitive) PrecheckIngress(_ []DNSPolicyRule) error {
	return f.step("precheck-ingress")
}

func (f *fakeDNSPolicyTakeoverPrimitive) SnapshotIngress() (DNSPolicyIngressSnapshot, error) {
	if err := f.step("snapshot-ingress"); err != nil {
		return DNSPolicyIngressSnapshot{}, err
	}
	return DNSPolicyIngressSnapshot{
		Identity:     "native-before",
		NativeOwner:  "keenetic",
		NativePort53: true,
	}, nil
}

func (f *fakeDNSPolicyTakeoverPrimitive) StartRouterForgeProxy(_ []DNSPolicyRule) error {
	return f.step("start-proxy")
}

func (f *fakeDNSPolicyTakeoverPrimitive) VerifyRouterForgeProxy(_ []DNSPolicyRule) error {
	return f.step("verify-proxy")
}

func (f *fakeDNSPolicyTakeoverPrimitive) SwitchIngressToRouterForge() error {
	return f.step("switch-ingress")
}

func (f *fakeDNSPolicyTakeoverPrimitive) VerifyIngressOnRouterForge() error {
	return f.step("verify-ingress")
}

func (f *fakeDNSPolicyTakeoverPrimitive) RestoreNativeIngress(_ DNSPolicyIngressSnapshot) error {
	return f.step("restore-native")
}

func (f *fakeDNSPolicyTakeoverPrimitive) StopRouterForgeProxy() error {
	return f.step("stop-proxy")
}

func (f *fakeDNSPolicyTakeoverPrimitive) VerifyNativeIngress(_ DNSPolicyIngressSnapshot) error {
	return f.step("verify-native")
}

func takeoverRules() []DNSPolicyRule {
	return []DNSPolicyRule{{ID: "one", Priority: 10, Policy: "Policy1"}}
}

func TestDNSPolicyTakeoverActivationOrder(t *testing.T) {
	primitive := &fakeDNSPolicyTakeoverPrimitive{}
	driver := newDNSPolicyTakeoverActivationDriver(primitive, func([]DNSPolicyRule) error { return nil })

	result, err := runDNSPolicyActivation("takeover-success", takeoverRules(), driver)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Activated || result.Manifest.State != transaction.Committed {
		t.Fatalf("result = %#v", result)
	}

	want := []string{
		"precheck-ingress",
		"snapshot-ingress",
		"precheck-ingress",
		"start-proxy",
		"verify-proxy",
		"switch-ingress",
		"verify-ingress",
	}
	if len(primitive.calls) != len(want) {
		t.Fatalf("calls = %#v", primitive.calls)
	}
	for i := range want {
		if primitive.calls[i] != want[i] {
			t.Fatalf("calls[%d] = %q want %q; all=%#v", i, primitive.calls[i], want[i], primitive.calls)
		}
	}
}

func TestDNSPolicyTakeoverVerifyFailureRestoresNativeBeforeStoppingProxy(t *testing.T) {
	primitive := &fakeDNSPolicyTakeoverPrimitive{
		failures: map[string]error{"verify-ingress": errors.New("verify failed")},
	}
	driver := newDNSPolicyTakeoverActivationDriver(primitive, func([]DNSPolicyRule) error { return nil })

	result, err := runDNSPolicyActivation("takeover-rollback", takeoverRules(), driver)
	if err == nil {
		t.Fatal("verify failure must return error")
	}
	if result.Manifest.State != transaction.RolledBack {
		t.Fatalf("state = %s", result.Manifest.State)
	}

	restore := -1
	stop := -1
	verify := -1
	for i, call := range primitive.calls {
		switch call {
		case "restore-native":
			restore = i
		case "stop-proxy":
			stop = i
		case "verify-native":
			verify = i
		}
	}
	if restore < 0 || stop < 0 || verify < 0 || !(restore < stop && stop < verify) {
		t.Fatalf("recovery ordering invalid: %#v", primitive.calls)
	}
}

func TestDNSPolicyTakeoverProxyVerifyFailureStillRollsBack(t *testing.T) {
	primitive := &fakeDNSPolicyTakeoverPrimitive{
		failures: map[string]error{"verify-proxy": errors.New("proxy verify failed")},
	}
	driver := newDNSPolicyTakeoverActivationDriver(primitive, func([]DNSPolicyRule) error { return nil })

	result, err := runDNSPolicyActivation("takeover-proxy-fail", takeoverRules(), driver)
	if err == nil {
		t.Fatal("proxy verify failure must return error")
	}
	if result.Manifest.State != transaction.RolledBack {
		t.Fatalf("state = %s", result.Manifest.State)
	}
	if len(primitive.calls) < 2 {
		t.Fatalf("calls = %#v", primitive.calls)
	}
}

func TestDNSPolicyTakeoverRestoreFailureBecomesAmbiguous(t *testing.T) {
	primitive := &fakeDNSPolicyTakeoverPrimitive{
		failures: map[string]error{
			"verify-ingress": errors.New("verify failed"),
			"restore-native": errors.New("restore failed"),
		},
	}
	driver := newDNSPolicyTakeoverActivationDriver(primitive, func([]DNSPolicyRule) error { return nil })

	result, err := runDNSPolicyActivation("takeover-ambiguous", takeoverRules(), driver)
	if err == nil {
		t.Fatal("restore failure must return error")
	}
	if result.Manifest.State != transaction.Ambiguous {
		t.Fatalf("state = %s", result.Manifest.State)
	}
}
