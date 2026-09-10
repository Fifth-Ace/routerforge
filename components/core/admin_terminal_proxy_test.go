package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminModuleTerminalRequest(t *testing.T) {
	good := []string{
		"/api/modules/admin/terminal/run",
		"/api/modules/admin/terminal/session",
		"/api/modules/admin/terminal/session/0123456789abcdef0123456789abcdef/output",
	}
	for _, path := range good {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if !adminModuleTerminalRequest(req) {
			t.Fatalf("terminal route not detected: %s", path)
		}
	}

	bad := []string{
		"/api/modules/admin/term",
		"/api/modules/admin/terminally",
		"/api/modules/dns/terminal/session",
	}
	for _, path := range bad {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if adminModuleTerminalRequest(req) {
			t.Fatalf("non-terminal route detected: %s", path)
		}
	}
}

func TestSecuredModuleProxyRequiresRootWhenAuthEnabledForTerminalOutput(t *testing.T) {
	auth, _ := testAdminAuthManager()
	auth.config.AuthRequired = true
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/modules/admin/terminal/session/0123456789abcdef0123456789abcdef/output?cursor=0",
		nil,
	)
	req.Host = "router.local"
	req.Header.Set("Origin", "http://router.local")

	securedModuleProxy(auth)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}
