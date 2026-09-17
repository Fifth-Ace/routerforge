package main

import (
	"fmt"
	"time"
)

type DNSPolicyPersistedShadowEvidence struct {
	StorePath         string
	DocumentSHA256    string
	DocumentVersion   int
	DocumentUpdatedAt time.Time
	RuleCount         int
	InventoryCount    int
	ListenAddr        string
	Upstream          string
	Duration          time.Duration
	Stats             DNSPolicyShadowStats
}

func dnsPolicyAllowedFromInventory(inventory []DNSPolicyOption) map[string]bool {
	allowed := map[string]bool{"System": true}
	for _, item := range inventory {
		allowed[normalizePolicyProxyName(item.Proxy)] = true
	}
	return allowed
}

func prepareDNSPolicyPersistedShadowConfig(
	doc DNSPolicyRulesDocument,
	inventory []DNSPolicyOption,
	listenAddr, upstream string,
	timeout time.Duration,
	maxConcurrent int,
) (DNSPolicyShadowConfig, error) {
	if doc.Version != dnsPolicyRulesSchemaVersion {
		return DNSPolicyShadowConfig{}, fmt.Errorf("unsupported persisted shadow schema version %d", doc.Version)
	}
	allowed := dnsPolicyAllowedFromInventory(inventory)
	rules, err := validateDNSPolicyRules(doc.Rules, allowed)
	if err != nil {
		return DNSPolicyShadowConfig{}, fmt.Errorf("persisted shadow rule validation: %w", err)
	}
	return validateDNSPolicyShadowConfig(DNSPolicyShadowConfig{
		ListenAddr:    listenAddr,
		Upstream:      upstream,
		Rules:         rules,
		Allowed:       allowed,
		Timeout:       timeout,
		MaxConcurrent: maxConcurrent,
	})
}
