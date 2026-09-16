package main

import "testing"

func TestCollectDNSPolicyNamesMapsIndexToHumanName(t *testing.T) {
	payload := map[string]any{
		"policy": []any{
			map[string]any{"index": float64(0), "name": "123", "mark": float64(0)},
			map[string]any{"index": float64(1), "name": "nfqws", "mark": float64(1)},
		},
	}
	out := map[string]string{"System": "System"}
	collectDNSPolicyNames(payload, "", out)
	if out["Policy0"] != "123" || out["Policy1"] != "nfqws" {
		t.Fatalf("policy map = %#v", out)
	}
}

func TestDNSPolicyInventoryStableOrdering(t *testing.T) {
	inventory := dnsPolicyInventoryFromNames(map[string]string{
		"Policy10": "ten",
		"Policy2":  "two",
		"System":   "System",
		"Policy1":  "one",
	})
	if len(inventory) != 4 {
		t.Fatalf("inventory length = %d", len(inventory))
	}
	want := []string{"System", "Policy1", "Policy2", "Policy10"}
	for i, proxy := range want {
		if inventory[i].Proxy != proxy {
			t.Fatalf("inventory[%d] = %#v, want proxy %q", i, inventory[i], proxy)
		}
	}
	if !inventory[0].System {
		t.Fatalf("System entry must be marked system: %#v", inventory[0])
	}
	if inventory[3].Ordinal != 10 {
		t.Fatalf("Policy10 ordinal = %d", inventory[3].Ordinal)
	}
}

func TestDNSPolicyInventoryNormalizesBlankDisplay(t *testing.T) {
	inventory := dnsPolicyInventoryFromNames(map[string]string{
		"policy7": "   ",
	})
	if len(inventory) != 1 {
		t.Fatalf("inventory = %#v", inventory)
	}
	if inventory[0].Proxy != "Policy7" || inventory[0].DisplayName != "Policy7" || inventory[0].Ordinal != 7 {
		t.Fatalf("normalized inventory = %#v", inventory[0])
	}
}

func TestPolicyProxyOrdinal(t *testing.T) {
	if got, ok := policyProxyOrdinal("Policy12"); !ok || got != 12 {
		t.Fatalf("Policy12 ordinal = %d, %v", got, ok)
	}
	if _, ok := policyProxyOrdinal("System"); ok {
		t.Fatal("System must not have policy ordinal")
	}
}

func TestEvaluateDNSPolicySpecificityWins(t *testing.T) {
	allowed := map[string]bool{"System": true, "Policy1": true, "Policy2": true}
	result, err := evaluateDNSPolicy(DNSPolicyEvaluationRequest{
		ClientIP:  "192.168.1.44",
		Domain:    "api.example.com",
		QueryType: "A",
		Rules: []DNSPolicyRule{
			{ID: "domain", Priority: 1, Policy: "Policy1", Match: DNSPolicyMatch{DomainSuffix: "example.com"}},
			{ID: "client-domain", Priority: 50, Policy: "Policy2", Match: DNSPolicyMatch{ClientCIDR: "192.168.1.0/24", DomainSuffix: "example.com"}},
		},
	}, allowed)
	if err != nil {
		t.Fatal(err)
	}
	if result.Policy != "Policy2" || result.RuleID != "client-domain" || result.Specificity != 6 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEvaluateDNSPolicyPriorityThenID(t *testing.T) {
	allowed := map[string]bool{"System": true, "Policy1": true, "Policy2": true}
	result, err := evaluateDNSPolicy(DNSPolicyEvaluationRequest{
		Domain: "example.com",
		Rules: []DNSPolicyRule{
			{ID: "z-rule", Priority: 10, Policy: "Policy2", Match: DNSPolicyMatch{DomainSuffix: "example.com"}},
			{ID: "a-rule", Priority: 10, Policy: "Policy1", Match: DNSPolicyMatch{DomainSuffix: "example.com"}},
		},
	}, allowed)
	if err != nil {
		t.Fatal(err)
	}
	if result.Policy != "Policy1" || result.RuleID != "a-rule" {
		t.Fatalf("tie must resolve by stable id: %#v", result)
	}
}

func TestEvaluateDNSPolicySystemFallback(t *testing.T) {
	result, err := evaluateDNSPolicy(DNSPolicyEvaluationRequest{
		Domain: "other.example",
		Rules: []DNSPolicyRule{
			{ID: "only", Priority: 1, Policy: "Policy1", Match: DNSPolicyMatch{DomainSuffix: "example.com"}},
		},
	}, map[string]bool{"System": true, "Policy1": true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Matched || result.Policy != "System" || !result.FallbackToSystem {
		t.Fatalf("unexpected fallback: %#v", result)
	}
}

func TestEvaluateDNSPolicyRejectsUnknownPolicy(t *testing.T) {
	_, err := evaluateDNSPolicy(DNSPolicyEvaluationRequest{
		Domain: "example.com",
		Rules: []DNSPolicyRule{
			{ID: "bad", Policy: "Policy9", Match: DNSPolicyMatch{}},
		},
	}, map[string]bool{"System": true, "Policy1": true})
	if err == nil {
		t.Fatal("unknown policy must be rejected")
	}
}
