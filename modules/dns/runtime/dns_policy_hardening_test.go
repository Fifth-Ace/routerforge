package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDNSPolicyJSONZeroValuesAreExplicit(t *testing.T) {
	payload, err := json.Marshal(DNSPolicyOption{Proxy: "System", DisplayName: "System", System: true})
	if err != nil {
		t.Fatal(err)
	}
	var option map[string]any
	if err := json.Unmarshal(payload, &option); err != nil {
		t.Fatal(err)
	}
	if _, ok := option["ordinal"]; !ok {
		t.Fatal("ordinal must be explicit even when zero")
	}

	evalPayload, err := json.Marshal(DNSPolicyEvaluation{Policy: "System"})
	if err != nil {
		t.Fatal(err)
	}
	var evaluation map[string]any
	if err := json.Unmarshal(evalPayload, &evaluation); err != nil {
		t.Fatal(err)
	}
	if _, ok := evaluation["rule_priority"]; !ok {
		t.Fatal("rule_priority must be explicit even when zero")
	}
	if _, ok := evaluation["specificity"]; !ok {
		t.Fatal("specificity must be explicit even when zero")
	}
}

func TestDNSPolicyStoreRepeatedOverwriteAndSemanticLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	store := newDNSPolicyStore(path)
	first := []DNSPolicyRule{{ID: "first", Priority: 10, Policy: "Policy1", Match: DNSPolicyMatch{DomainSuffix: "one.test"}}}
	second := []DNSPolicyRule{{ID: "second", Priority: 20, Policy: "Policy0", Match: DNSPolicyMatch{DomainSuffix: "two.test"}}}
	if _, err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(second); err != nil {
		t.Fatal(err)
	}
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Rules) != 1 || doc.Rules[0].ID != "second" {
		t.Fatalf("doc=%#v", doc)
	}

	bad := []byte(`{"version":1,"updated_at":"2026-09-17T00:00:00Z","rules":[{"id":"","priority":0,"policy":"Policy1","match":{}}]}`)
	if err := os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("semantically invalid persisted document unexpectedly loaded")
	}
}
