package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOperationLogsReadOnlyAndRedacted(t *testing.T) {
	operationLog.Lock()
	old := operationLog.Entries
	oldNext := operationLog.Next
	operationLog.Entries = nil
	operationLog.Next = 0
	operationLog.Unlock()
	defer func() {
		operationLog.Lock()
		operationLog.Entries = old
		operationLog.Next = oldNext
		operationLog.Unlock()
	}()

	secret := "DO_NOT_LEAK_SPA_KEY"
	wrapped := auditOperation("odin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/odin", strings.NewReader(secret))
	req.Header.Set("X-RouterForge-Module-Authorized", "core-authorized-v1")
	wrapped(httptest.NewRecorder(), req)
	response := httptest.NewRecorder()
	operationLogsHandler(response, httptest.NewRequest(http.MethodGet, "/v1/operation-logs", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "FAIL HTTP Conflict") {
		t.Fatalf("expected recorded failure: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), secret) {
		t.Fatal("request body leaked into operation log")
	}
	response = httptest.NewRecorder()
	operationLogsHandler(response, httptest.NewRequest(http.MethodPost, "/v1/operation-logs", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatal("log API must be read-only")
	}
}
