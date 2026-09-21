package main

import (
	"strings"
	"testing"
)

func TestV2PortableCandidateArgsStripsSelectors(t *testing.T) {
	in := []string{
		"--hostlist-domains=example.com",
		"--hostlist=/opt/etc/nfqws2/lists/user.list",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	}
	got := v2PortableCandidateArgs(in)
	text := strings.Join(got, " ")
	if strings.Contains(text, "hostlist") {
		t.Fatalf("portable args retained selection-only hostlist: %q", text)
	}
	if !strings.Contains(text, "--filter-tcp=443") || !strings.Contains(text, "--lua-desync=") {
		t.Fatalf("portable args lost strategy technique: %q", text)
	}
}

func TestV2BuiltinCandidateCatalogUniqueAndCompilable(t *testing.T) {
	seenID := map[string]bool{}
	seenFP := map[string]bool{}
	for _, item := range v2BuiltinHTTPSCandidates {
		if item.ID == "" || seenID[item.ID] {
			t.Fatalf("duplicate/empty builtin id: %q", item.ID)
		}
		seenID[item.ID] = true
		if item.Protocol != "https" {
			t.Fatalf("unexpected protocol for %s: %s", item.ID, item.Protocol)
		}
		if _, err := v2CustomProfile(item.Args, "example.com"); err != nil {
			t.Fatalf("builtin %s does not compile through V2 candidate compiler: %v", item.ID, err)
		}
		fp := v2CandidateTechniqueFingerprint(item.Args)
		if fp == "" || seenFP[fp] {
			t.Fatalf("duplicate/empty technique fingerprint for %s", item.ID)
		}
		seenFP[fp] = true
	}
	if len(seenID) < 8 {
		t.Fatalf("builtin catalog unexpectedly small: %d", len(seenID))
	}
}

func TestV2AppendPoolItemDeduplicatesTechnique(t *testing.T) {
	seen := map[string]bool{}
	a := v2CandidatePoolItem{
		ID: "a", Name: "a", Source: "builtin", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld"},
	}
	b := a
	b.ID = "b"
	b.Source = "memory"
	b.Args = append([]string{"--hostlist-domains=example.com"}, b.Args...)
	out := v2AppendPoolItem(nil, seen, a)
	out = v2AppendPoolItem(out, seen, b)
	if len(out) != 1 {
		t.Fatalf("expected technique dedupe to keep one candidate, got %d", len(out))
	}
}

func TestV2RecommendationProtocolCompatibilityIsExact(t *testing.T) {
	https, _ := normalizeBenchTransport(benchTransportHTTPS)
	if !v2RecommendationProtocolCompatible("https", https) {
		t.Fatal("https recommendation should match https transport")
	}
	if v2RecommendationProtocolCompatible("quic", https) {
		t.Fatal("quic recommendation must not leak into https planning")
	}
}

func TestV2RecommendationSourcePrefersConcreteOrigin(t *testing.T) {
	entry := v2StrategyRegistryEntry{Sources: []string{"memory", "builtin"}}
	if got := v2RecommendationSource(entry); got != "builtin" {
		t.Fatalf("source=%q want builtin", got)
	}
}
