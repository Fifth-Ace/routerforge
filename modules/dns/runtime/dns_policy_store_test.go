package main

import (
	"path/filepath"
	"testing"
)

func TestDNSPolicyStoreRoundTrip(t *testing.T) {
	store := newDNSPolicyStore(filepath.Join(t.TempDir(), "dns-policy-rules.json"))
	rules := []DNSPolicyRule{
		{ID: "b", Priority: 20, Policy: "Policy2", Match: DNSPolicyMatch{DomainSuffix: "example.org"}},
		{ID: "a", Priority: 10, Policy: "Policy1", Match: DNSPolicyMatch{ClientCIDR: "192.168.1.0/24"}},
	}
	saved, err := store.Save(rules)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version != dnsPolicyRulesSchemaVersion || saved.UpdatedAt.IsZero() {
		t.Fatalf("bad saved document: %#v", saved)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Rules) != 2 || loaded.Rules[0].ID != "b" || loaded.Rules[1].ID != "a" {
		t.Fatalf("round-trip changed rules: %#v", loaded.Rules)
	}
}

func TestDNSPolicyStoreMissingIsEmpty(t *testing.T) {
	store := newDNSPolicyStore(filepath.Join(t.TempDir(), "missing.json"))
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version != dnsPolicyRulesSchemaVersion || len(doc.Rules) != 0 {
		t.Fatalf("unexpected missing-store document: %#v", doc)
	}
}

func TestValidateDNSPolicyRulesCanonicalOrder(t *testing.T) {
	rules, err := validateDNSPolicyRules([]DNSPolicyRule{
		{ID: "z", Priority: 20, Policy: "Policy2"},
		{ID: "b", Priority: 10, Policy: "Policy1"},
		{ID: "a", Priority: 10, Policy: "System"},
	}, map[string]bool{"System": true, "Policy1": true, "Policy2": true})
	if err != nil {
		t.Fatal(err)
	}
	if rules[0].ID != "a" || rules[1].ID != "b" || rules[2].ID != "z" {
		t.Fatalf("unexpected canonical order: %#v", rules)
	}
}

func TestValidateDNSPolicyRulesRejectsDuplicateAndUnknown(t *testing.T) {
	_, err := validateDNSPolicyRules([]DNSPolicyRule{
		{ID: "same", Policy: "Policy1"},
		{ID: "same", Policy: "Policy1"},
	}, map[string]bool{"System": true, "Policy1": true})
	if err == nil {
		t.Fatal("duplicate id must fail")
	}
	_, err = validateDNSPolicyRules([]DNSPolicyRule{
		{ID: "unknown", Policy: "Policy9"},
	}, map[string]bool{"System": true, "Policy1": true})
	if err == nil {
		t.Fatal("unknown policy must fail")
	}
}
