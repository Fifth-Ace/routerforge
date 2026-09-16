package main

import (
	"path/filepath"
	"testing"
	"time"
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

func TestBuildDNSPolicyActivationPreview(t *testing.T) {
	doc := DNSPolicyRulesDocument{
		Version:   dnsPolicyRulesSchemaVersion,
		UpdatedAt: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC),
	}
	rules := []DNSPolicyRule{
		{ID: "p2", Priority: 20, Policy: "Policy2"},
		{ID: "sys", Priority: 10, Policy: "System"},
		{ID: "p1", Priority: 30, Policy: "Policy1"},
	}
	preview := buildDNSPolicyActivationPreview(doc, rules)
	if !preview.Ready || preview.Activated {
		t.Fatalf("unexpected readiness state: %#v", preview)
	}
	if preview.PersistedRules != 3 || preview.ActiveRules != 0 {
		t.Fatalf("unexpected rule counts: %#v", preview)
	}
	want := []string{"System", "Policy1", "Policy2"}
	if len(preview.Policies) != len(want) {
		t.Fatalf("policies = %#v", preview.Policies)
	}
	for i := range want {
		if preview.Policies[i] != want[i] {
			t.Fatalf("policies[%d] = %q, want %q", i, preview.Policies[i], want[i])
		}
	}
	if len(preview.Changes) != 1 || len(preview.Evidence) != 4 {
		t.Fatalf("preview evidence incomplete: %#v", preview)
	}
}

func TestBuildDNSPolicyActivationPreviewEmptyRules(t *testing.T) {
	doc := DNSPolicyRulesDocument{Version: dnsPolicyRulesSchemaVersion}
	preview := buildDNSPolicyActivationPreview(doc, nil)
	if !preview.Ready || preview.Activated || preview.PersistedRules != 0 || preview.ActiveRules != 0 {
		t.Fatalf("unexpected empty preview: %#v", preview)
	}
	if len(preview.Policies) != 0 {
		t.Fatalf("empty rules must have no policies: %#v", preview.Policies)
	}
}
