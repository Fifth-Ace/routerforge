//go:build linux

package main

import (
	"fmt"
	"time"
)

type DNSPolicyActivationAcceptanceResult struct {
	Activation DNSPolicyActivationResult
	RolledBack bool
	NativeOK   bool
}

func runDNSPolicyActivationAcceptance(iface, listenAddr, upstream string, timeout time.Duration) (DNSPolicyActivationAcceptanceResult, error) {
	primitive, err := newDNSPolicyProductionTakeoverPrimitive(iface, listenAddr, upstream, timeout)
	if err != nil {
		return DNSPolicyActivationAcceptanceResult{}, err
	}
	if err := primitive.ingress.ReconcileStale(); err != nil {
		return DNSPolicyActivationAcceptanceResult{}, err
	}

	inventory, err := readDNSPolicyInventory()
	if err != nil {
		return DNSPolicyActivationAcceptanceResult{}, err
	}
	allowed := dnsPolicyAllowedFromInventory(inventory)
	rules := []DNSPolicyRule{
		{ID: "accept-policy1", Priority: 10, Policy: "Policy1", Match: DNSPolicyMatch{DomainSuffix: "routerforge-accept-policy1.invalid", QueryType: "A"}},
		{ID: "accept-policy0", Priority: 20, Policy: "Policy0", Match: DNSPolicyMatch{DomainSuffix: "routerforge-accept-policy0.invalid", QueryType: "A"}},
	}
	validator := func(input []DNSPolicyRule) error {
		_, err := validateDNSPolicyRules(input, allowed)
		return err
	}
	driver := newDNSPolicyTakeoverActivationDriver(primitive, validator)
	activation, err := runDNSPolicyActivation("p18-final-acceptance", rules, driver)
	if err != nil {
		return DNSPolicyActivationAcceptanceResult{Activation: activation}, err
	}
	result := DNSPolicyActivationAcceptanceResult{Activation: activation}
	if !activation.Activated {
		return result, fmt.Errorf("activation transaction did not commit")
	}

	if err := driver.Rollback(activation.Snapshot); err != nil {
		return result, fmt.Errorf("acceptance rollback: %w", err)
	}
	if err := driver.VerifyRollback(activation.Snapshot); err != nil {
		return result, fmt.Errorf("acceptance rollback verify: %w", err)
	}
	result.RolledBack = true
	result.NativeOK = true
	return result, nil
}
