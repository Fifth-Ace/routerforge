package main

import (
	"strings"
	"testing"
	"time"
)

func TestValidateDNSPolicyShadowConfigRejectsNonLoopbackPort53AndBadConcurrency(t *testing.T) {
	base := DNSPolicyShadowConfig{Upstream: "1.1.1.1:53", Allowed: map[string]bool{"System": true}, Timeout: time.Second}
	for _, listen := range []string{"0.0.0.0:5300", "192.168.1.1:5300", "127.0.0.1:53"} {
		cfg := base
		cfg.ListenAddr = listen
		if _, err := validateDNSPolicyShadowConfig(cfg); err == nil {
			t.Fatalf("listen %s unexpectedly accepted", listen)
		}
	}
	cfg := base
	cfg.ListenAddr = "127.0.0.1:5533"
	cfg.MaxConcurrent = maxDNSPolicyShadowConcurrency + 1
	if _, err := validateDNSPolicyShadowConfig(cfg); err == nil {
		t.Fatal("oversized concurrency unexpectedly accepted")
	}
}

func TestValidateDNSPolicyShadowConfigAcceptsLoopbackNon53(t *testing.T) {
	cfg, err := validateDNSPolicyShadowConfig(DNSPolicyShadowConfig{
		ListenAddr: "127.0.0.1:5533",
		Upstream:   "1.1.1.1:53",
		Allowed:    map[string]bool{"System": true, "Policy1": true},
		Rules:      []DNSPolicyRule{{ID: "r1", Priority: 10, Policy: "Policy1", Match: DNSPolicyMatch{DomainSuffix: "example.com"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timeout != 4*time.Second {
		t.Fatalf("timeout = %v", cfg.Timeout)
	}
	if cfg.MaxConcurrent != defaultDNSPolicyShadowConcurrency {
		t.Fatalf("max concurrency = %d", cfg.MaxConcurrent)
	}
}

func TestPlanDNSPolicyShadowQueryUsesExistingEvaluatorAndEgressMapping(t *testing.T) {
	cfg, err := validateDNSPolicyShadowConfig(DNSPolicyShadowConfig{
		ListenAddr: "127.0.0.1:5533",
		Upstream:   "1.1.1.1:53",
		Allowed:    map[string]bool{"System": true, "Policy1": true},
		Rules:      []DNSPolicyRule{{ID: "domain-policy", Priority: 10, Policy: "Policy1", Match: DNSPolicyMatch{DomainSuffix: "example.com", QueryType: "A"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	routes := map[string]policyRoute{"Policy1": {Name: "Policy1", Mark: 0x0ffffaab, Table: 4098, HasDefault: true}}
	query := buildDNSQuery(0x1234, "www.example.com", 1)
	plan, err := planDNSPolicyShadowQuery(query, "127.0.0.1", cfg, routes)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Evaluation.Matched || plan.Evaluation.RuleID != "domain-policy" || plan.Evaluation.Policy != "Policy1" {
		t.Fatalf("evaluation = %#v", plan.Evaluation)
	}
	if plan.Target.Mark != 0x0ffffaab || plan.Target.Table != 4098 || plan.Target.System {
		t.Fatalf("target = %#v", plan.Target)
	}
}

func TestPlanDNSPolicyShadowQueryFallsBackToSystemWithoutMatch(t *testing.T) {
	cfg, err := validateDNSPolicyShadowConfig(DNSPolicyShadowConfig{
		ListenAddr: "127.0.0.1:5533",
		Upstream:   "1.1.1.1:53",
		Allowed:    map[string]bool{"System": true, "Policy1": true},
		Rules:      []DNSPolicyRule{{ID: "other", Priority: 10, Policy: "Policy1", Match: DNSPolicyMatch{DomainSuffix: "other.test"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planDNSPolicyShadowQuery(buildDNSQuery(1, "example.com", 1), "127.0.0.1", cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Evaluation.FallbackToSystem || !plan.Target.System {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanDNSPolicyShadowQueryRejectsResponsePacket(t *testing.T) {
	cfg, err := validateDNSPolicyShadowConfig(DNSPolicyShadowConfig{ListenAddr: "127.0.0.1:5533", Upstream: "1.1.1.1:53", Allowed: map[string]bool{"System": true}})
	if err != nil {
		t.Fatal(err)
	}
	query := buildDNSQuery(1, "example.com", 1)
	query[2] |= 0x80
	if _, err := planDNSPolicyShadowQuery(query, "127.0.0.1", cfg, nil); err == nil || !strings.Contains(err.Error(), "invalid shadow DNS query") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildDNSPolicyShadowFailureResponsePreservesQuestionAndSetsSERVFAIL(t *testing.T) {
	query := buildDNSQuery(0x4242, "example.com", 1)
	response, err := buildDNSPolicyShadowFailureResponse(query, 2)
	if err != nil {
		t.Fatal(err)
	}
	msg, ok := parseDNSMessage(response)
	if !ok || !msg.QR || msg.ID != 0x4242 || msg.RCode != 2 || msg.QName != "example.com" || msg.QType != 1 {
		t.Fatalf("response = %#v ok=%t", msg, ok)
	}
}
