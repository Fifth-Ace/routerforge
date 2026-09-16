package main

import (
	"fmt"
)

type DNSPolicyAdapterPrimitive struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Evidence  string `json:"evidence"`
}

type DNSPolicyAdapterDiscovery struct {
	ProductionDriverReady bool                        `json:"production_driver_ready"`
	KnownPrimitives       []DNSPolicyAdapterPrimitive `json:"known_primitives"`
	BlockingUnknowns      []string                    `json:"blocking_unknowns"`
}

func discoverDNSPolicyRuntimeAdapter() DNSPolicyAdapterDiscovery {
	return DNSPolicyAdapterDiscovery{
		ProductionDriverReady: false,
		KnownPrimitives: []DNSPolicyAdapterPrimitive{
			{
				Name:      "bounded-rci-read",
				Available: true,
				Evidence:  "dnsRCIClient.getJSON provides bounded context-aware RCI GET",
			},
			{
				Name:      "structured-rci-mutation",
				Available: true,
				Evidence:  "dnsRCIClient.postJSON/deleteSetting provide structured RCI mutation without shell interpolation",
			},
			{
				Name:      "configuration-save",
				Available: true,
				Evidence:  "existing DNS mutations persist native changes through /system/configuration/save",
			},
			{
				Name:      "exact-readback-verification",
				Available: true,
				Evidence:  "dnsControlManager verifies canonical native RCI readback before commit",
			},
			{
				Name:      "verified-rollback",
				Available: true,
				Evidence:  "dnsControlManager restores the captured native state and verifies it before reporting recovery",
			},
			{
				Name:      "runtime-health-probe",
				Available: true,
				Evidence:  "existing DNS mutation path runs the DNS Unix health contract before commit",
			},
		},
		BlockingUnknowns: []string{
			"exact Keenetic RCI read path and response schema for active policy-router rule state",
			"exact Keenetic RCI mutation path and payload schema for installing policy-router rules",
			"stable runtime identity/readback contract that proves the installed policy-router state equals the desired rules",
		},
	}
}

type dnsPolicyRuntimePrimitive interface {
	SnapshotRuntime() (DNSPolicyRuntimeSnapshot, error)
	ApplyCanonicalRules([]DNSPolicyRule) error
	VerifyCanonicalRules([]DNSPolicyRule) error
	RestoreRuntime(DNSPolicyRuntimeSnapshot) error
	VerifyRuntimeSnapshot(DNSPolicyRuntimeSnapshot) error
	RuntimeHealth() error
}

type dnsPolicyActivationAdapter struct {
	primitive dnsPolicyRuntimePrimitive
	validate  func([]DNSPolicyRule) error
}

func newDNSPolicyActivationAdapter(
	primitive dnsPolicyRuntimePrimitive,
	validate func([]DNSPolicyRule) error,
) *dnsPolicyActivationAdapter {
	return &dnsPolicyActivationAdapter{
		primitive: primitive,
		validate:  validate,
	}
}

func (a *dnsPolicyActivationAdapter) Precheck(rules []DNSPolicyRule) error {
	if a == nil || a.primitive == nil {
		return fmt.Errorf("policy runtime primitive is unavailable")
	}
	if a.validate == nil {
		return fmt.Errorf("policy rule validator is unavailable")
	}
	return a.validate(rules)
}

func (a *dnsPolicyActivationAdapter) Snapshot() (DNSPolicyRuntimeSnapshot, error) {
	if a == nil || a.primitive == nil {
		return DNSPolicyRuntimeSnapshot{}, fmt.Errorf("policy runtime primitive is unavailable")
	}
	return a.primitive.SnapshotRuntime()
}

func (a *dnsPolicyActivationAdapter) Validate(rules []DNSPolicyRule) error {
	if err := a.Precheck(rules); err != nil {
		return err
	}
	return a.primitive.RuntimeHealth()
}

func (a *dnsPolicyActivationAdapter) Apply(rules []DNSPolicyRule) error {
	return a.primitive.ApplyCanonicalRules(rules)
}

func (a *dnsPolicyActivationAdapter) Verify(rules []DNSPolicyRule) error {
	if err := a.primitive.VerifyCanonicalRules(rules); err != nil {
		return err
	}
	return a.primitive.RuntimeHealth()
}

func (a *dnsPolicyActivationAdapter) Rollback(snapshot DNSPolicyRuntimeSnapshot) error {
	return a.primitive.RestoreRuntime(snapshot)
}

func (a *dnsPolicyActivationAdapter) VerifyRollback(snapshot DNSPolicyRuntimeSnapshot) error {
	if err := a.primitive.VerifyRuntimeSnapshot(snapshot); err != nil {
		return err
	}
	return a.primitive.RuntimeHealth()
}
