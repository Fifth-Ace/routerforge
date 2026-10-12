package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// P40B is deliberately side-effect-free: malformed and unauthorized requests
// must be rejected before reaching any firewall, service or filesystem mutation.
func TestP40BMutationAuthorizationAndDecode(t *testing.T) {
	h := routes(t.TempDir())
	cases := []struct {
		name, path, body, auth string
		want                   int
	}{
		{"deploy unauthorized", "/v1/engine-deploy", `{"engine":"knockd","action":"enable","confirm":"APPLY"}`, "", 403},
		{"deploy wrong token", "/v1/engine-deploy", `{}`, "core-authorized-v2", 403},
		{"deploy unknown field", "/v1/engine-deploy", `{"unexpected":1}`, "core-authorized-v1", 400},
		{"deploy trailing JSON", "/v1/engine-deploy", `{} {}`, "core-authorized-v1", 400},
		{"deploy missing confirmation", "/v1/engine-deploy", `{"engine":"knockd","action":"enable"}`, "core-authorized-v1", 400},
		{"deploy unknown engine", "/v1/engine-deploy", `{"engine":"nope","action":"enable","confirm":"APPLY"}`, "core-authorized-v1", 400},
		{"wizard unauthorized", "/v1/engine-wizard-preview", `{}`, "", 403},
		{"wizard unknown field", "/v1/engine-wizard-preview", `{"unexpected":1}`, "core-authorized-v1", 400},
		{"wizard trailing JSON", "/v1/engine-wizard-preview", `{} {}`, "core-authorized-v1", 400},
		{"recent revoke unauthorized", "/v1/recent-revoke", `{"ip":"127.0.0.1","confirm":"REVOKE"}`, "", 403},
		{"recent revoke unknown field", "/v1/recent-revoke", `{"unknown":true}`, "core-authorized-v1", 400},
		{"recent revoke missing confirmation", "/v1/recent-revoke", `{"ip":"127.0.0.1"}`, "core-authorized-v1", 400},
		{"recent revoke zone scoped IP", "/v1/recent-revoke", `{"ip":"fe80::1%eth0","confirm":"REVOKE"}`, "core-authorized-v1", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			if tc.auth != "" {
				req.Header.Set("X-RouterForge-Module-Authorized", tc.auth)
			}
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			if resp.Code != tc.want {
				t.Fatalf("status=%d want=%d response=%s", resp.Code, tc.want, resp.Body.String())
			}
			var obj map[string]any
			if err := json.Unmarshal(resp.Body.Bytes(), &obj); err != nil {
				t.Fatalf("not JSON: %v", err)
			}
			if _, ok := obj["error"]; !ok {
				t.Fatalf("missing error field: %v", obj)
			}
			if got := resp.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("cache-control=%q", got)
			}
			if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("nosniff=%q", got)
			}
		})
	}
}

func TestP40BReadOnlyEndpointsRejectMutation(t *testing.T) {
	h := routes(t.TempDir())
	for _, path := range []string{"/v1/health", "/v1/status", "/v1/preflight", "/v1/knock-preview", "/v1/forward-order", "/v1/nat-evidence", "/v1/odin-access", "/v1/ui/", "/v1/operation-logs", "/v1/recent-access"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"unsafe":true}`))
			req.Header.Set("X-RouterForge-Module-Authorized", "core-authorized-v1")
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			if resp.Code != http.StatusMethodNotAllowed {
				t.Fatalf("POST %s returned %d: %s", path, resp.Code, resp.Body.String())
			}
		})
	}
}

func TestP40BOperationAuditNoSecretLeakAndBounded(t *testing.T) {
	operationLog.Lock()
	previous, previousID := operationLog.Entries, operationLog.Next
	operationLog.Entries, operationLog.Next = nil, 0
	operationLog.Unlock()
	defer func() {
		operationLog.Lock()
		operationLog.Entries, operationLog.Next = previous, previousID
		operationLog.Unlock()
	}()

	secret := "P40B_PRIVATE_SPA_TOKEN_NOT_FOR_LOGGING"
	wrapped := auditOperation("p40b-test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/engine-deploy", strings.NewReader(secret))
	wrapped(httptest.NewRecorder(), req)
	operationLog.Lock()
	if len(operationLog.Entries) != 0 {
		t.Fatal("unauthorized operation was audited")
	}
	operationLog.Unlock()
	req.Header.Set("X-RouterForge-Module-Authorized", "core-authorized-v1")
	wrapped(httptest.NewRecorder(), req)
	for i := 0; i < operationLogLimit+20; i++ {
		recordOperation("p40b-test", "request", "FAIL")
	}
	resp := httptest.NewRecorder()
	operationLogsHandler(resp, httptest.NewRequest(http.MethodGet, "/v1/operation-logs", nil))
	if resp.Code != http.StatusOK || strings.Contains(resp.Body.String(), secret) {
		t.Fatalf("unsafe log output: %s", resp.Body.String())
	}
	var payload struct {
		Entries []operationLogEntry `json:"entries"`
		LastID  uint64              `json:"last_id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Entries) != operationLogLimit {
		t.Fatalf("log size=%d limit=%d", len(payload.Entries), operationLogLimit)
	}
	if payload.LastID != uint64(operationLogLimit+22) {
		t.Fatalf("last id=%d", payload.LastID)
	}
}
