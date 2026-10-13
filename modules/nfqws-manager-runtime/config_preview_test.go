package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNF1AConfigValidateEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		wantOK  bool
	}{
		{name: "valid", content: "NFQWS_ARGS='--filter-tcp=443'\n", wantOK: true},
		{name: "invalid shell", content: "NFQWS_ARGS='unterminated\n", wantOK: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{"content": tc.content})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/config/validate", strings.NewReader(string(body)))
			req.Header.Set(authHeader, authValue)
			resp := httptest.NewRecorder()
			mutationOnly(handleConfigValidate)(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("HTTP %d: %s", resp.Code, resp.Body.String())
			}
			var result struct {
				OK           bool   `json:"ok"`
				SyntaxOK     bool   `json:"syntax_ok"`
				NativeDryRun string `json:"native_dry_run"`
				Error        string `json:"error"`
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.OK != tc.wantOK || result.SyntaxOK != tc.wantOK || result.NativeDryRun != "not_performed" {
				t.Fatalf("unexpected preview: %+v", result)
			}
			if !tc.wantOK && result.Error == "" {
				t.Fatal("syntax failure needs explanation")
			}
		})
	}
}

func TestNF1AConfigValidateRequiresCoreAuthorization(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/config/validate", strings.NewReader(`{"content":"A=1\n"}`))
	resp := httptest.NewRecorder()
	mutationOnly(handleConfigValidate)(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("unauthorized validation accepted: HTTP %d", resp.Code)
	}
}
