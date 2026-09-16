package main

import "testing"

func TestResolveDNSPolicyEgressTargetSystem(t *testing.T) {
	target, err := resolveDNSPolicyEgressTarget("System", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !target.System || target.Mark != 0 || target.Table != 0 {
		t.Fatalf("unexpected system target: %#v", target)
	}
}

func TestResolveDNSPolicyEgressTargetMapsMarkAndTable(t *testing.T) {
	routes := map[string]policyRoute{
		"Policy1": {
			Name:       "Policy1",
			Mark:       0x0ffffaab,
			Table:      4098,
			HasDefault: true,
		},
	}
	target, err := resolveDNSPolicyEgressTarget("Policy1", routes)
	if err != nil {
		t.Fatal(err)
	}
	if target.Policy != "Policy1" || target.Mark != 0x0ffffaab || target.Table != 4098 || !target.HasDefault || target.System {
		t.Fatalf("unexpected target: %#v", target)
	}
}

func TestResolveDNSPolicyEgressTargetAllowsFailClosedPolicyWithoutDefault(t *testing.T) {
	routes := map[string]policyRoute{
		"Policy0": {
			Name:       "Policy0",
			Mark:       0x0ffffaaa,
			Table:      4096,
			HasDefault: false,
		},
	}
	target, err := resolveDNSPolicyEgressTarget("Policy0", routes)
	if err != nil {
		t.Fatal(err)
	}
	if target.HasDefault {
		t.Fatalf("policy without default must remain explicit: %#v", target)
	}
}

func TestResolveDNSPolicyEgressTargetRejectsUnknownOrIncompletePolicy(t *testing.T) {
	cases := []struct {
		name   string
		policy string
		routes map[string]policyRoute
	}{
		{name: "unknown", policy: "Policy9", routes: map[string]policyRoute{}},
		{name: "zero-mark", policy: "Policy1", routes: map[string]policyRoute{"Policy1": {Table: 4098}}},
		{name: "zero-table", policy: "Policy1", routes: map[string]policyRoute{"Policy1": {Mark: 0x0ffffaab}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolveDNSPolicyEgressTarget(tc.policy, tc.routes); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
