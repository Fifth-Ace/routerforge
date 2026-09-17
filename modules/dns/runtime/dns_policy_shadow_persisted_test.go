package main

import (
	"strings"
	"testing"
	"time"
)

func TestDNSPolicyAllowedFromInventoryIncludesSystemAndPolicies(t *testing.T) {
	allowed := dnsPolicyAllowedFromInventory([]DNSPolicyOption{
		{Proxy: "Policy1"},
		{Proxy: "Policy2"},
	})
	for _, key := range []string{"System", "Policy1", "Policy2"} {
		if !allowed[key] {
			t.Fatalf("%s missing", key)
		}
	}
}

func TestPrepareDNSPolicyPersistedShadowConfigValidatesAgainstInventory(t *testing.T) {
	doc := DNSPolicyRulesDocument{
		Version: dnsPolicyRulesSchemaVersion,
		Rules: []DNSPolicyRule{
			{ID: "r1", Priority: 10, Policy: "Policy1", Match: DNSPolicyMatch{DomainSuffix: "example.com"}},
		},
	}
	cfg, err := prepareDNSPolicyPersistedShadowConfig(
		doc,
		[]DNSPolicyOption{{Proxy: "Policy1"}},
		"127.0.0.1:55353",
		"1.1.1.1:53",
		time.Second,
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Rules) != 1 || cfg.MaxConcurrent != 8 {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestPrepareDNSPolicyPersistedShadowConfigRejectsStalePolicy(t *testing.T) {
	doc := DNSPolicyRulesDocument{
		Version: dnsPolicyRulesSchemaVersion,
		Rules: []DNSPolicyRule{
			{ID: "stale", Priority: 10, Policy: "Policy9", Match: DNSPolicyMatch{DomainSuffix: "example.com"}},
		},
	}
	_, err := prepareDNSPolicyPersistedShadowConfig(
		doc,
		[]DNSPolicyOption{{Proxy: "Policy1"}},
		"127.0.0.1:55353",
		"1.1.1.1:53",
		time.Second,
		8,
	)
	if err == nil || !strings.Contains(err.Error(), "unknown policy") {
		t.Fatalf("err = %v", err)
	}
}

func TestPrepareDNSPolicyPersistedShadowConfigRejectsWrongSchema(t *testing.T) {
	_, err := prepareDNSPolicyPersistedShadowConfig(
		DNSPolicyRulesDocument{Version: 999},
		nil,
		"127.0.0.1:55353",
		"1.1.1.1:53",
		time.Second,
		8,
	)
	if err == nil {
		t.Fatal("wrong schema unexpectedly accepted")
	}
}
