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
