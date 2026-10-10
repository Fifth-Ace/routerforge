package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPreflightNeverAuthorizesApply(t *testing.T) {
	for _, candidate := range []struct {
		recent bool
		tables string
		err    error
	}{
		{true, "filter\nnat", nil},
		{false, "", errors.New("not readable")},
		{false, "nat", nil},
	} {
		p := preflightFrom(candidate.recent, candidate.tables, candidate.err)
		if p.MutationAPI || p.ReadyForApply || len(p.Checks) != 6 {
			t.Fatalf("unsafe preflight result: %+v", p)
		}
		for _, check := range p.Checks {
			if check.State == "pass" || check.State == "ready" {
				t.Fatalf("unauthorized gate: %+v", check)
			}
		}
	}
}

func TestPreflightEndpointReadOnly(t *testing.T) {
	handler := routes(t.TempDir())
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodDelete} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(method, "/v1/preflight", nil))
		expected := http.StatusOK
		if method != http.MethodGet && method != http.MethodHead {
			expected = http.StatusMethodNotAllowed
		}
		if w.Code != expected {
			t.Fatalf("%s code=%d", method, w.Code)
		}
		if method == http.MethodGet && (!strings.Contains(w.Body.String(), `"ready_for_apply":false`) || !strings.Contains(w.Body.String(), `"mutation_api":false`)) {
			t.Fatal("preflight did not enforce dry-run")
		}
	}
}
