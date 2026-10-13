package main

import "testing"

func TestNF2AGuidanceDoesNotSuggestStrategyWithoutEvidence(t *testing.T) {
	for _, code := range []string{
		"DNS_RESOLUTION_FAILURE", "TCP_CONNECT_TIMEOUT", "TCP_CONNECT_RESET",
		"HTTP_RESPONSE_RESTRICTED_NOT_BLOCK_PROOF", "CLEAR_END_TO_END", "INCONCLUSIVE",
		"HTTP_STREAM_CUTOFF_12_20K_SUSPECTED",
	} {
		got := v2GuideDiagnostic(v2DiagnosticVerdict{Code: code, StrategyRelevant: true})
		if got.NextAction == "selector" {
			t.Fatalf("%s incorrectly suggests direct selector", code)
		}
		if got.Title == "" || got.Summary == "" || got.Scope == "" {
			t.Fatalf("%s has incomplete guidance: %+v", code, got)
		}
	}
}

func TestNF2AGuidanceSuggestsSelectorForRelevantTLSOrHTTPFailure(t *testing.T) {
	for _, code := range []string{"TLS_HANDSHAKE_TIMEOUT", "TLS_HANDSHAKE_RESET", "HTTP_TRANSPORT_EOF", "HTTP_FIRST_BYTE_FAILURE"} {
		got := v2GuideDiagnostic(v2DiagnosticVerdict{Code: code, StrategyRelevant: true})
		if got.NextAction != "selector" || got.NextLabel == "" {
			t.Fatalf("%s should suggest selector: %+v", code, got)
		}
	}
}

func TestNF2AGuidanceDoesNotClaimClientPathProven(t *testing.T) {
	got := v2GuideDiagnostic(v2DiagnosticVerdict{Code: "CLEAR_END_TO_END"})
	if got.NextAction != "none" || got.Scope == "" {
		t.Fatalf("clear result should not suggest bypass: %+v", got)
	}
}
