package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNF2BSiteContextCases(t *testing.T) {
	tests := []struct {
		name           string
		diag           string
		action         string
		running        bool
		binding        string
		cutoff         bool
		expectedAction string
		expectedText   string
		hasTCP16       bool
	}{
		{"stopped before selector", "TLS_HANDSHAKE_TIMEOUT", "selector", false, "ONE_PRODUCTION_PROFILE_MATCH", false, "service", "остановлена", false},
		{"multiple candidate profiles", "TLS_HANDSHAKE_TIMEOUT", "selector", true, "MULTIPLE_PRODUCTION_PROFILE_MATCHES", false, "selector", "несколько", false},
		{"orphan list match", "TLS_HANDSHAKE_TIMEOUT", "selector", true, "LIST_MATCH_WITHOUT_PROFILE_BINDING", false, "selector", "списке", false},
		{"dns not strategy", "DNS_RESOLUTION_FAILURE", "dns", true, "NO_ACTIVE_BINDING_PROVEN", false, "dns", "IP-списки", false},
		{"tcp16 requires repetition", "HTTP_STREAM_CUTOFF_12_20K_SUSPECTED", "retry", true, "ONE_PRODUCTION_PROFILE_MATCH", true, "retry", "потенциально", true},
		{"working site", "CLEAR_END_TO_END", "none", true, "ONE_PRODUCTION_PROFILE_MATCH", false, "none", "потенциально", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := v2DetectResponse{Diagnostic: v2DiagnosticVerdict{Code: tc.diag}, Guidance: v2DomainGuidance{NextAction: tc.action}, Cutoff16KSuspected: tc.cutoff}
			i := v2InspectResponse{RuntimeRunning: tc.running, BindingVerdict: tc.binding}
			got := v2DescribeSiteContext(d, i)
			if got.NextAction != tc.expectedAction || !strings.Contains(got.BindingDetail, tc.expectedText) || (got.TCP16Detail != "") != tc.hasTCP16 {
				t.Fatalf("unexpected context: %+v", got)
			}
			if !strings.Contains(got.NetworkScope, "LAN") {
				t.Fatalf("missing router-side limitation: %+v", got)
			}
		})
	}
}

func TestNF2BSiteEndpointRequiresAuthorization(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/v2/site-diagnose", strings.NewReader(`{"target":"example.com"}`))
	response := httptest.NewRecorder()
	mutationOnly(handleV2SiteDiagnose)(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unexpected HTTP %d", response.Code)
	}
}

func TestNF2BSiteEndpointRejectsBadTargetWithoutNetwork(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/v2/site-diagnose", strings.NewReader(`{"target":"http://localhost/admin"}`))
	request.Header.Set(authHeader, authValue)
	response := httptest.NewRecorder()
	mutationOnly(handleV2SiteDiagnose)(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unexpected HTTP %d: %s", response.Code, response.Body.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["error"] == nil {
		t.Fatalf("missing explanation: %v", decoded)
	}
}
