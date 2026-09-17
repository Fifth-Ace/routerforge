package main

import (
	"fmt"
)

type DNSPolicyIngressSnapshot struct {
	Identity        string
	NativeOwner     string
	NativePort53    bool
	RouterForgeAddr string
}

type dnsPolicyTakeoverPrimitive interface {
	PrecheckIngress([]DNSPolicyRule) error
	SnapshotIngress() (DNSPolicyIngressSnapshot, error)
	StartRouterForgeProxy([]DNSPolicyRule) error
	VerifyRouterForgeProxy([]DNSPolicyRule) error
	SwitchIngressToRouterForge() error
	VerifyIngressOnRouterForge() error
	RestoreNativeIngress(DNSPolicyIngressSnapshot) error
	StopRouterForgeProxy() error
	VerifyNativeIngress(DNSPolicyIngressSnapshot) error
}

type dnsPolicyTakeoverActivationDriver struct {
	primitive dnsPolicyTakeoverPrimitive
	validate  func([]DNSPolicyRule) error
	snapshot  DNSPolicyIngressSnapshot
	started   bool
	switched  bool
}

func newDNSPolicyTakeoverActivationDriver(
	primitive dnsPolicyTakeoverPrimitive,
	validate func([]DNSPolicyRule) error,
) *dnsPolicyTakeoverActivationDriver {
	return &dnsPolicyTakeoverActivationDriver{
		primitive: primitive,
		validate:  validate,
	}
}

func (d *dnsPolicyTakeoverActivationDriver) Precheck(rules []DNSPolicyRule) error {
	if d == nil || d.primitive == nil {
		return fmt.Errorf("DNS takeover primitive is unavailable")
	}
	if d.validate == nil {
		return fmt.Errorf("DNS takeover validator is unavailable")
	}
	if err := d.validate(rules); err != nil {
		return err
	}
	return d.primitive.PrecheckIngress(rules)
}

func (d *dnsPolicyTakeoverActivationDriver) Snapshot() (DNSPolicyRuntimeSnapshot, error) {
	if d == nil || d.primitive == nil {
		return DNSPolicyRuntimeSnapshot{}, fmt.Errorf("DNS takeover primitive is unavailable")
	}
	snapshot, err := d.primitive.SnapshotIngress()
	if err != nil {
		return DNSPolicyRuntimeSnapshot{}, err
	}
	if snapshot.Identity == "" {
		return DNSPolicyRuntimeSnapshot{}, fmt.Errorf("empty DNS ingress snapshot identity")
	}
	d.snapshot = snapshot
	return DNSPolicyRuntimeSnapshot{Identity: snapshot.Identity, RuleCount: 0}, nil
}

func (d *dnsPolicyTakeoverActivationDriver) Validate(rules []DNSPolicyRule) error {
	if err := d.Precheck(rules); err != nil {
		return err
	}
	if d.snapshot.Identity == "" {
		return fmt.Errorf("DNS ingress snapshot is unavailable")
	}
	return nil
}

func (d *dnsPolicyTakeoverActivationDriver) Apply(rules []DNSPolicyRule) error {
	if d == nil || d.primitive == nil {
		return fmt.Errorf("DNS takeover primitive is unavailable")
	}
	if err := d.primitive.StartRouterForgeProxy(rules); err != nil {
		return err
	}
	d.started = true
	if err := d.primitive.VerifyRouterForgeProxy(rules); err != nil {
		return err
	}
	if err := d.primitive.SwitchIngressToRouterForge(); err != nil {
		return err
	}
	d.switched = true
	return nil
}

func (d *dnsPolicyTakeoverActivationDriver) Verify(_ []DNSPolicyRule) error {
	if d == nil || d.primitive == nil {
		return fmt.Errorf("DNS takeover primitive is unavailable")
	}
	if !d.started || !d.switched {
		return fmt.Errorf("DNS takeover apply sequence is incomplete")
	}
	return d.primitive.VerifyIngressOnRouterForge()
}

func (d *dnsPolicyTakeoverActivationDriver) Rollback(_ DNSPolicyRuntimeSnapshot) error {
	if d == nil || d.primitive == nil {
		return fmt.Errorf("DNS takeover primitive is unavailable")
	}
	if d.snapshot.Identity == "" {
		return fmt.Errorf("DNS ingress rollback snapshot is unavailable")
	}

	// Recovery ordering is deliberate: restore native ingress ownership first,
	// then stop the RouterForge proxy. This avoids leaving clients with no DNS
	// listener if the RouterForge side is torn down before native recovery.
	if err := d.primitive.RestoreNativeIngress(d.snapshot); err != nil {
		return err
	}
	d.switched = false
	if d.started {
		if err := d.primitive.StopRouterForgeProxy(); err != nil {
			return err
		}
		d.started = false
	}
	return nil
}

func (d *dnsPolicyTakeoverActivationDriver) VerifyRollback(_ DNSPolicyRuntimeSnapshot) error {
	if d == nil || d.primitive == nil {
		return fmt.Errorf("DNS takeover primitive is unavailable")
	}
	if d.snapshot.Identity == "" {
		return fmt.Errorf("DNS ingress rollback snapshot is unavailable")
	}
	return d.primitive.VerifyNativeIngress(d.snapshot)
}
