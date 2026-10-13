package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNF1B1PreviewSemanticErrorsAreNotSyntaxErrors(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/config/validate", strings.NewReader(`{"content":"NFQWS_ARGS='--dpi-desync=fake'\n"}`))
	request.Header.Set(authHeader, authValue)
	response := httptest.NewRecorder()
	mutationOnly(handleConfigValidate)(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("http %d: %s", response.Code, response.Body.String())
	}
	var parsed struct {
		OK           bool              `json:"ok"`
		SyntaxOK     bool              `json:"syntax_ok"`
		SemanticOK   bool              `json:"semantic_ok"`
		NativeDryRun string            `json:"native_dry_run"`
		Issues       []nf1bConfigIssue `json:"issues"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.OK || !parsed.SyntaxOK || parsed.SemanticOK || parsed.NativeDryRun != "not_performed" {
		t.Fatalf("semantic failure conflated with syntax/native: %+v", parsed)
	}
	if len(parsed.Issues) == 0 || parsed.Issues[0].Code != "LEGACY_NFQWS_V1" {
		t.Fatalf("missing semantic issue: %+v", parsed.Issues)
	}
}

func TestNF1B1SemanticApplyRejectsDeprecatedNfqws1Option(t *testing.T) {
	if err := validateConfig("NFQWS_ARGS='--dpi-desync=fake'\n"); err == nil || !strings.Contains(err.Error(), "--dpi-desync") {
		t.Fatalf("deprecated option not rejected: %v", err)
	}
}
