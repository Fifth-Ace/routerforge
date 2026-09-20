package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestBenchTransportDefaultsToHTTPS(t *testing.T) {
	profile, err := normalizeBenchTransport("")
	if err != nil {
		t.Fatal(err)
	}
	if profile.ID != benchTransportHTTPS || profile.Network != "tcp" || profile.RemotePort != 443 {
		t.Fatalf("unexpected default transport: %+v", profile)
	}
}

func TestBenchTransportHTTPProfile(t *testing.T) {
	profile, err := normalizeBenchTransport("http")
	if err != nil {
		t.Fatal(err)
	}
	if profile.ID != benchTransportHTTP || profile.Network != "tcp" || profile.RemotePort != 80 {
		t.Fatalf("unexpected HTTP transport: %+v", profile)
	}
	if profile.MetricScope != "http-full-response" {
		t.Fatalf("metric_scope=%q", profile.MetricScope)
	}
}

func TestBenchTransactionTransportDefaultsPreserveHTTPS(t *testing.T) {
	network, remotePort, err := normalizeBenchTransactionTransport(benchTransactionSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if network != "tcp" || remotePort != 443 {
		t.Fatalf("network=%q remote_port=%d", network, remotePort)
	}
}

func TestBenchRulePlanHTTPUsesExactTCP80Tuple(t *testing.T) {
	spec := benchTestSpec()
	spec.Network = "tcp"
	spec.RemotePort = 80

	rules, err := buildBenchRulePlan(spec)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(rules[0].RuleArgs, " ")
	in := strings.Join(rules[3].RuleArgs, " ")

	if !strings.Contains(out, "-p tcp") ||
		!strings.Contains(out, "-d 203.0.113.10") ||
		!strings.Contains(out, "--sport 43123") ||
		!strings.Contains(out, "--dport 80") {
		t.Fatalf("unexpected outbound HTTP tuple: %q", out)
	}
	if !strings.Contains(in, "-p tcp") ||
		!strings.Contains(in, "-s 203.0.113.10") ||
		!strings.Contains(in, "--sport 80") ||
		!strings.Contains(in, "--dport 43123") {
		t.Fatalf("unexpected inbound HTTP tuple: %q", in)
	}
}

func TestBenchRulePlanUDPFoundationUsesExactTuple(t *testing.T) {
	spec := benchTestSpec()
	spec.Network = "udp"
	spec.RemotePort = 3478

	rules, err := buildBenchRulePlan(spec)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(rules[1].RuleArgs, " ")
	in := strings.Join(rules[4].RuleArgs, " ")

	if !strings.Contains(out, "-p udp") || !strings.Contains(out, "--dport 3478") {
		t.Fatalf("unexpected outbound UDP tuple: %q", out)
	}
	if !strings.Contains(in, "-p udp") || !strings.Contains(in, "--sport 3478") {
		t.Fatalf("unexpected inbound UDP tuple: %q", in)
	}
}

func TestBenchHTTPProfileEligibility(t *testing.T) {
	transport, err := normalizeBenchTransport("http")
	if err != nil {
		t.Fatal(err)
	}
	profile := analyzeBenchStrategyProfile(0, []string{
		"--hostlist-domains=example.com",
		"--filter-tcp=80",
		"--filter-l7=http",
		"--payload=http_req",
		"--lua-desync=http_methodeol:badsum",
	})
	profile = prepareBenchStrategyProfileForTransport(profile, transport)
	if !profile.CandidateEligible {
		t.Fatalf("HTTP profile unexpectedly rejected: %v", profile.Reasons)
	}
}

func TestBenchHTTPProfileRejectsTLSOnlyCandidate(t *testing.T) {
	transport, err := normalizeBenchTransport("http")
	if err != nil {
		t.Fatal(err)
	}
	profile := analyzeBenchStrategyProfile(0, []string{
		"--hostlist-domains=example.com",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	})
	profile = prepareBenchStrategyProfileForTransport(profile, transport)
	if profile.CandidateEligible {
		t.Fatal("TLS-only profile was accepted for HTTP/TCP80")
	}
}

func TestV2CustomHTTPProfileCompilesSelectionFree(t *testing.T) {
	transport, err := normalizeBenchTransport("http")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := v2CustomProfileForTransport([]string{
		"--hostlist=/opt/etc/nfqws2/lists/example.list",
		"--filter-tcp=80",
		"--filter-l7=http",
		"--payload=http_req",
		"--lua-desync=http_methodeol:badsum",
	}, "example.com", transport)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(profile.Args, " ")
	if strings.Contains(joined, "--hostlist=") {
		t.Fatalf("file-bound selector leaked into custom HTTP profile: %q", joined)
	}
	if !strings.Contains(joined, "--hostlist-domains=example.com") {
		t.Fatalf("exact target selector missing: %q", joined)
	}
}

func TestV2BenchProfilesRouteRegistered(t *testing.T) {
	mux := http.NewServeMux()
	registerStrategyIntelligenceV2Routes(mux)

	req, err := http.NewRequest(http.MethodGet, "http://unix/v1/v2/bench-profiles", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, pattern := mux.Handler(req)
	if pattern != "/v1/v2/bench-profiles" {
		t.Fatalf("pattern=%q", pattern)
	}
}

func TestV2BenchRequestAcceptsHTTPTransport(t *testing.T) {
	req := v2BenchRequest{
		Target:               "example.com",
		Transport:            "http",
		Args:                 []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=http_methodeol:badsum"},
		ExpectedConfigSHA256: strings.Repeat("a", 64),
		Confirm:              v2BenchConfirm,
	}
	if err := validateV2BenchRequest(req); err != nil {
		t.Fatalf("HTTP bench request rejected: %v", err)
	}
}
