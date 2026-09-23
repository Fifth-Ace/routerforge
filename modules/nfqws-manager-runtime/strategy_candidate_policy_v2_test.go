package main

import "testing"

func TestC3BBuiltinCatalogTransportAware(t *testing.T) {
	cases := map[string]int{
		benchTransportHTTPS: 8,
		benchTransportHTTP:  3,
		benchTransportQUIC:  4,
		benchTransportSTUN:  4,
	}
	for id, want := range cases {
		transport, err := normalizeBenchTransport(id)
		if err != nil {
			t.Fatal(err)
		}
		items := v2BuiltinCandidatesForTransport(transport)
		if len(items) != want {
			t.Fatalf("transport=%s builtins=%d want=%d", id, len(items), want)
		}
		seenID := map[string]bool{}
		seenFP := map[string]bool{}
		for _, item := range items {
			if item.ID == "" || seenID[item.ID] {
				t.Fatalf("transport=%s duplicate/empty id=%q", id, item.ID)
			}
			seenID[item.ID] = true
			if item.Protocol != id {
				t.Fatalf("transport=%s builtin=%s protocol=%s", id, item.ID, item.Protocol)
			}
			if _, err := v2CustomProfileForTransport(item.Args, "example.com", transport); err != nil {
				t.Fatalf("transport=%s builtin=%s compile: %v", id, item.ID, err)
			}
			fp := v2CandidateTechniqueFingerprint(item.Args)
			if fp == "" || seenFP[fp] {
				t.Fatalf("transport=%s duplicate/empty fingerprint for %s", id, item.ID)
			}
			seenFP[fp] = true
		}
	}
}

func TestC3BMemoryFingerprintProtocolSeparation(t *testing.T) {
	const configSHA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if got, want := v2MemoryEnvironmentFingerprintForProtocol(configSHA, benchTransportHTTPS), v2MemoryEnvironmentFingerprint(configSHA); got != want {
		t.Fatalf("https fingerprint compatibility changed: got=%s want=%s", got, want)
	}
	seen := map[string]bool{}
	for _, protocol := range []string{benchTransportHTTPS, benchTransportHTTP, benchTransportQUIC, benchTransportSTUN} {
		fp := v2MemoryEnvironmentFingerprintForProtocol(configSHA, protocol)
		if fp == "" || seen[fp] {
			t.Fatalf("protocol=%s fingerprint is empty or collides", protocol)
		}
		seen[fp] = true
	}
}
