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
		ProductionDriverReady: true,
		KnownPrimitives: []DNSPolicyAdapterPrimitive{
			{
				Name:      "persisted-rule-engine",
				Available: true,
				Evidence:  "P18B-P18Q validate and execute DNSPolicyRule inside the RouterForge shadow forwarder",
			},
			{
				Name:      "policy-mark-egress",
				Available: true,
				Evidence:  "P18J/P18K hardware evidence proves PolicyN -> SO_MARK -> Keenetic policy routing with fail-closed semantics",
			},
			{
				Name:      "udp-tcp-forwarder",
				Available: true,
				Evidence:  "P18M-P18Q hardware evidence proves loopback UDP/TCP forwarding, SERVFAIL and bounded concurrency",
			},
			{
				Name:      "activation-transaction-engine",
				Available: true,
				Evidence:  "P18F/R106 hardware acceptance proves snapshot/apply/verify/commit plus verified rollback semantics",
			},
			{
				Name:      "ipv4-interface-scoped-ingress-takeover",
				Available: true,
				Evidence:  "R105/R106 prove reversible IPv4 PREROUTING takeover; external client TCP/53 produced three packets on actual eth3 ingress and cleanup left no RouterForge rules",
			},
			{
				Name:      "native-dns-recovery",
				Available: true,
				Evidence:  "hardware acceptance proves native ndnproxy UDP/TCP recovery without process restart before RouterForge proxy shutdown",
			},
			{
				Name:      "bounded-rci-read",
				Available: true,
				Evidence:  "dnsRCIClient.getJSON provides bounded context-aware RCI GET",
			},
			{
				Name:      "verified-native-dns-mutation",
				Available: true,
				Evidence:  "dnsControlManager already snapshots, mutates, verifies and rolls back native Keenetic resolver configuration",
			},
		},
		BlockingUnknowns: []string{},
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
