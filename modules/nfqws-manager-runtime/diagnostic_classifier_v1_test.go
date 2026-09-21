package main

import "testing"

func diagnosticStages(dns, tcp, tls, http string) map[string]v2StageResult {
	return map[string]v2StageResult{
		"dns":  {State: dns},
		"tcp":  {State: tcp},
		"tls":  {State: tls},
		"http": {State: http},
	}
}

func TestDiagnosticDNSFailureStopsBeforeStrategy(t *testing.T) {
	stages := diagnosticStages("fail", "skipped", "skipped", "skipped")
	stages["dns"] = v2StageResult{State: "fail", Detail: "lookup example.com: no such host"}
	got := v2ClassifyDiagnostic(stages, v2HTTPMetrics{})
	if got.Code != "DNS_RESOLUTION_FAILURE" || got.StrategyRelevant || got.Confidence != "high" {
		t.Fatalf("got=%+v", got)
	}
}

func TestDiagnosticTCPRefusedIsNotDPIProof(t *testing.T) {
	stages := diagnosticStages("pass", "fail", "skipped", "skipped")
	stages["tcp"] = v2StageResult{State: "fail", Detail: "dial tcp: connect: connection refused"}
	got := v2ClassifyDiagnostic(stages, v2HTTPMetrics{})
	if got.Code != "TCP_CONNECT_REFUSED" || got.StrategyRelevant {
		t.Fatalf("got=%+v", got)
	}
}

func TestDiagnosticTLSFailureIsStrategyRelevant(t *testing.T) {
	stages := diagnosticStages("pass", "pass", "fail", "skipped")
	stages["tls"] = v2StageResult{State: "fail", Detail: "i/o timeout"}
	got := v2ClassifyDiagnostic(stages, v2HTTPMetrics{})
	if got.Code != "TLS_HANDSHAKE_TIMEOUT" || !got.StrategyRelevant || got.FaultDomain != "tls_path" {
		t.Fatalf("got=%+v", got)
	}
}

func TestDiagnosticCutoffPrecedesGenericHTTPFailure(t *testing.T) {
	stages := diagnosticStages("pass", "pass", "pass", "fail")
	stages["http"] = v2StageResult{State: "fail", Detail: "unexpected EOF"}
	got := v2ClassifyDiagnostic(stages, v2HTTPMetrics{
		Bytes: 15 << 10, ProgressProven: true, Cutoff16KSuspected: true,
	})
	if got.Code != "HTTP_STREAM_CUTOFF_12_20K_SUSPECTED" || !got.StrategyRelevant {
		t.Fatalf("got=%+v", got)
	}
}

func TestDiagnosticBare451IsApplicationResponseNotBlockProof(t *testing.T) {
	stages := diagnosticStages("pass", "pass", "pass", "warn")
	stages["http"] = v2StageResult{State: "warn", Detail: "HTTP 451", StatusCode: 451}
	got := v2ClassifyDiagnostic(stages, v2HTTPMetrics{HTTPStatus: 451, ResponseComplete: true})
	if got.Code != "HTTP_RESPONSE_RESTRICTED_NOT_BLOCK_PROOF" || got.StrategyRelevant || got.FaultDomain != "application" {
		t.Fatalf("got=%+v", got)
	}
}

func TestDiagnosticClearEndToEnd(t *testing.T) {
	stages := diagnosticStages("pass", "pass", "pass", "pass")
	got := v2ClassifyDiagnostic(stages, v2HTTPMetrics{
		HTTPStatus: 200, ResponseComplete: true, ProgressProven: true,
	})
	if got.Code != "CLEAR_END_TO_END" || got.Severity != "ok" || got.StrategyRelevant {
		t.Fatalf("got=%+v", got)
	}
}

func TestLegacyClassificationRemainsCompatible(t *testing.T) {
	stages := diagnosticStages("pass", "pass", "fail", "skipped")
	class, _ := v2ClassifyDetect(stages, v2HTTPMetrics{})
	if class != "tls_failure" {
		t.Fatalf("legacy class=%q", class)
	}
}
