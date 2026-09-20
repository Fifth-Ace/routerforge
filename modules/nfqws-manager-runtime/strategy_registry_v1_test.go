package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStrategyRegistryMergesBuiltinLibraryAndMemory(t *testing.T) {
	args := append([]string{}, v2BuiltinHTTPSCandidates[0].Args...)
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)

	library := v2StrategyLibraryDocument{
		Version: 1,
		Strategies: []v2StoredStrategy{{
			ID: "custom-alias", Name: "My alias", Source: "custom",
			Args: args, Fingerprint: v2StrategyFingerprint(args),
		}},
	}
	memory := v2TargetMemoryDocument{
		Version: v2TargetMemoryVersion,
		Entries: []v2TargetMemoryEntry{{
			Target: "example.com", Protocol: "https", IPFamily: "ipv4",
			CandidateName: "Verified alias", CandidateSource: "custom",
			Args: args, Fingerprint: v2StrategyFingerprint(args),
			ResultClass: "WORKING", SuccessRate: 1, CompleteRate: 1,
			VerifiedCount: 3, SuccessStreak: 3, FailureStreak: 0,
			LastVerified:     now.Add(-time.Hour).Format(time.RFC3339Nano),
			InfrastructureOK: true, CleanupProven: true,
		}},
	}

	registry := v2BuildStrategyRegistry(library, memory, now)
	fp := v2CandidateTechniqueFingerprint(args)
	var found *v2StrategyRegistryEntry
	for i := range registry.Entries {
		if registry.Entries[i].Fingerprint == fp {
			found = &registry.Entries[i]
			break
		}
	}
	if found == nil {
		t.Fatal("merged strategy not found")
	}
	if !registry.ReadOnly {
		t.Fatal("registry must be read-only")
	}
	if found.Evidence.Targets != 1 || found.Evidence.VerifiedCount != 3 {
		t.Fatalf("unexpected evidence: %+v", found.Evidence)
	}
	if found.Evidence.Confidence != v2MemoryConfidenceTrusted {
		t.Fatalf("confidence=%q", found.Evidence.Confidence)
	}
	for _, want := range []string{"builtin", "custom", "memory"} {
		ok := false
		for _, got := range found.Sources {
			if got == want {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("source %q missing from %+v", want, found.Sources)
		}
	}
}

func TestStrategyRegistryInfersProtocolAndFamily(t *testing.T) {
	protocol, family := v2RegistryInferProtocolFamily([]string{
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=fakedsplit:pos=midsld",
	})
	if protocol != "https" || family != "fake-split" {
		t.Fatalf("protocol=%q family=%q", protocol, family)
	}
}

func TestStrategyRegistryRouteIsGetOnly(t *testing.T) {
	mux := http.NewServeMux()
	registerStrategyRegistryV1Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/v2/strategy-registry", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status=%d body=%s", rr.Code, rr.Body.String())
	}
}
