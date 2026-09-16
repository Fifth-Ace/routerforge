package main

import (
	"fmt"
	"strings"
)

type DNSPolicyEgressTarget struct {
	Policy     string
	Mark       uint32
	Table      int
	HasDefault bool
	System     bool
}

func resolveDNSPolicyEgressTarget(policy string, routes map[string]policyRoute) (DNSPolicyEgressTarget, error) {
	policy = strings.TrimSpace(policy)
	if policy == "" {
		return DNSPolicyEgressTarget{}, fmt.Errorf("empty DNS policy")
	}
	if policy == "System" {
		return DNSPolicyEgressTarget{Policy: "System", System: true}, nil
	}
	route, ok := routes[policy]
	if !ok {
		return DNSPolicyEgressTarget{}, fmt.Errorf("DNS policy %q is unavailable", policy)
	}
	if route.Mark == 0 {
		return DNSPolicyEgressTarget{}, fmt.Errorf("DNS policy %q has no routing mark", policy)
	}
	if route.Table <= 0 {
		return DNSPolicyEgressTarget{}, fmt.Errorf("DNS policy %q has invalid routing table %d", policy, route.Table)
	}
	return DNSPolicyEgressTarget{
		Policy:     policy,
		Mark:       route.Mark,
		Table:      route.Table,
		HasDefault: route.HasDefault,
	}, nil
}
