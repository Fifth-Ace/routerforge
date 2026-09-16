package main

import "testing"

func TestParseRouteRecordIPv4(t *testing.T) {
	route, ok := parseRouteRecord("default via 192.168.1.1 dev eth0 proto dhcp src 192.168.1.20 metric 100 table 100", "ipv4")
	if !ok {
		t.Fatal("route did not parse")
	}
	if route.Destination != "default" || route.Gateway != "192.168.1.1" || route.Interface != "eth0" || route.Source != "192.168.1.20" || route.Metric != 100 || route.Table != "100" || route.Protocol != "dhcp" {
		t.Fatalf("unexpected route: %#v", route)
	}
}

func TestParseRouteRecordIPv6(t *testing.T) {
	route, ok := parseRouteRecord("default via fe80::1 dev eth0 metric 1024 pref medium", "ipv6")
	if !ok {
		t.Fatal("route did not parse")
	}
	if route.Family != "ipv6" || route.Gateway != "fe80::1" || route.Interface != "eth0" || route.Metric != 1024 || route.Preference != "medium" {
		t.Fatalf("unexpected IPv6 route: %#v", route)
	}
}

func TestParsePolicyRule(t *testing.T) {
	rule, ok := parsePolicyRule("1000: from 192.168.10.0/24 fwmark 0x10 lookup 100", "ipv4")
	if !ok {
		t.Fatal("rule did not parse")
	}
	if rule.Priority != 1000 || rule.From != "192.168.10.0/24" || rule.Mark != "0x10" || rule.Table != "100" {
		t.Fatalf("unexpected rule: %#v", rule)
	}
}

func TestParseRouteGet(t *testing.T) {
	decision := parseRouteGet("8.8.8.8 via 192.168.1.1 dev eth0 src 192.168.1.20 table 100 metric 5 uid 0", "ipv4")
	if !decision.Available || decision.Destination != "8.8.8.8" || decision.Gateway != "192.168.1.1" || decision.Interface != "eth0" || decision.Source != "192.168.1.20" || decision.Table != "100" || decision.Metric != 5 {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestNonDefaultPolicyRules(t *testing.T) {
	rules := []policyRule{{Priority: 0, Table: "local"}, {Priority: 32766, Table: "main"}, {Priority: 32767, Table: "default"}, {Priority: 1000, Table: "100"}}
	if got := nonDefaultPolicyRules(rules); got != 1 {
		t.Fatalf("expected 1 non-default rule, got %d", got)
	}
}

func TestDoctorRouteStagesPolicyTable(t *testing.T) {
	decision := kernelRouteDecision{
		Available:   true,
		Family:      "ipv4",
		Destination: "8.8.8.8",
		Gateway:     "192.168.1.1",
		Interface:   "eth0",
		Source:      "192.168.1.20",
		Table:       "100",
		Type:        "unicast",
	}
	routeStage, policyStage := doctorRouteStages(decision, "active", true)
	if routeStage.Status != "ok" || policyStage.Status != "ok" {
		t.Fatalf("unexpected stages: route=%+v policy=%+v", routeStage, policyStage)
	}
	if policyStage.Detail != "policy routing active; kernel selected table 100" {
		t.Fatalf("unexpected policy detail: %q", policyStage.Detail)
	}
}

func TestDoctorRouteStagesIPv6(t *testing.T) {
	decision := kernelRouteDecision{
		Available:   true,
		Family:      "ipv6",
		Destination: "2001:4860:4860::8888",
		Gateway:     "fe80::1",
		Interface:   "eth0",
		Table:       "main",
		Type:        "unicast",
	}
	routeStage, _ := doctorRouteStages(decision, "default-only", true)
	if routeStage.Status != "ok" {
		t.Fatalf("IPv6 kernel decision rejected: %+v", routeStage)
	}
}

func TestDoctorRouteStagesBlockingRoute(t *testing.T) {
	decision := kernelRouteDecision{
		Available:   true,
		Family:      "ipv4",
		Destination: "203.0.113.7",
		Table:       "100",
		Type:        "blackhole",
	}
	routeStage, policyStage := doctorRouteStages(decision, "active", true)
	if routeStage.Status != "fail" {
		t.Fatalf("blackhole route must fail: %+v", routeStage)
	}
	if policyStage.Status != "ok" {
		t.Fatalf("active policy itself must not be failure: %+v", policyStage)
	}
}

func TestDoctorRouteStagesUnresolvedTarget(t *testing.T) {
	routeStage, policyStage := doctorRouteStages(kernelRouteDecision{}, "unavailable", false)
	if routeStage.Status != "skipped" || policyStage.Status != "unavailable" {
		t.Fatalf("unexpected unresolved stages: route=%+v policy=%+v", routeStage, policyStage)
	}
}

func TestDoctorPathStagesHealthyMainPath(t *testing.T) {
	defaultRoute := &routeEntry{Gateway: "192.168.1.1", Interface: "eth0"}
	decision := kernelRouteDecision{
		Available: true, Family: "ipv4", Destination: "8.8.8.8",
		Gateway: "192.168.1.1", Interface: "eth0", Source: "192.168.1.20",
		Table: "main", Type: "unicast",
	}
	egress, gateway, source := doctorPathStages(decision, defaultRoute, doctorPathFacts{
		EgressExists: true, EgressUp: true, SourceChecked: true, SourceMatches: true,
	})
	if egress.Status != "ok" || gateway.Status != "ok" || source.Status != "ok" {
		t.Fatalf("unexpected stages: egress=%+v gateway=%+v source=%+v", egress, gateway, source)
	}
}

func TestDoctorPathStagesEgressDown(t *testing.T) {
	decision := kernelRouteDecision{
		Available: true, Family: "ipv4", Destination: "8.8.8.8",
		Gateway: "192.168.1.1", Interface: "eth0", Source: "192.168.1.20",
		Table: "main", Type: "unicast",
	}
	egress, _, _ := doctorPathStages(decision, nil, doctorPathFacts{
		EgressExists: true, EgressUp: false, SourceChecked: true, SourceMatches: true,
	})
	if egress.Status != "fail" {
		t.Fatalf("down kernel-selected egress must fail: %+v", egress)
	}
}

func TestDoctorPathStagesSourceMismatch(t *testing.T) {
	decision := kernelRouteDecision{
		Available: true, Family: "ipv4", Destination: "8.8.8.8",
		Gateway: "192.168.1.1", Interface: "eth0", Source: "10.0.0.10",
		Table: "100", Type: "unicast",
	}
	_, gateway, source := doctorPathStages(decision, nil, doctorPathFacts{
		EgressExists: true, EgressUp: true, SourceChecked: true, SourceMatches: false,
	})
	if gateway.Status != "ok" {
		t.Fatalf("policy-table gateway should remain valid: %+v", gateway)
	}
	if source.Status != "fail" {
		t.Fatalf("source/interface mismatch must fail: %+v", source)
	}
}

func TestDoctorVerdictPrefersPathMismatch(t *testing.T) {
	stages := []doctorStage{
		{ID: "internet", Status: "fail"},
		{ID: "source_consistency", Status: "fail"},
	}
	verdict := doctorVerdictFor(stages)
	if verdict.Code != "route_source_mismatch" || verdict.FaultDomain != "local" {
		t.Fatalf("unexpected verdict: %+v", verdict)
	}
}
