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
