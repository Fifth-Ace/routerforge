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
