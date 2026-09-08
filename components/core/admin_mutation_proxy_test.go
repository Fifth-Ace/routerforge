package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testAdminAuthManager() (*authManager, string) {
	token := "phase8-test-session"
	return &authManager{
		config: securityConfig{AuthRequired: false},
		sessions: map[string]authSession{
			token: {User: "root", ExpiresAt: time.Now().Add(time.Hour)},
		},
		attempts: map[string]loginAttempt{},
	}, token
}

func TestAdminMutationMethodMatrix(t *testing.T) {
	if !moduleMethodAllowed("admin", http.MethodPost) {
		t.Fatal("admin POST must be allowed by module method matrix")
	}
	if moduleMethodAllowed("admin", http.MethodPatch) || moduleMethodAllowed("admin", http.MethodDelete) {
		t.Fatal("admin PATCH/DELETE must remain blocked in Phase 8A")
	}
}

func TestSecuredModuleProxyRequiresSessionEvenWhenGlobalAuthDisabled(t *testing.T) {
	auth, _ := testAdminAuthManager()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/modules/admin/processes/123/signal", strings.NewReader(`{"signal":"TERM","confirm_pid":123}`))
	req.Host = "router.local"
	req.Header.Set("Origin", "http://router.local")
	securedModuleProxy(auth)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestSecuredModuleProxyRejectsCrossOriginMutation(t *testing.T) {
	auth, token := testAdminAuthManager()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/modules/admin/processes/123/signal", strings.NewReader(`{"signal":"TERM","confirm_pid":123}`))
	req.Host = "router.local"
	req.Header.Set("Origin", "http://evil.local")
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: token})
	securedModuleProxy(auth)(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestSecuredModuleProxyAddsInternalMarkerOnlyAfterRootSession(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "admin.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	markerSeen := make(chan bool, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		markerSeen <- r.Header.Get(adminMutationAuthorizationHeader) == adminMutationAuthorizationValue
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	oldSockets := moduleSockets
	moduleSockets = map[string][]string{"admin": {socket}}
	t.Cleanup(func() { moduleSockets = oldSockets })

	auth, token := testAdminAuthManager()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/modules/admin/processes/123/signal", strings.NewReader(`{"signal":"TERM","confirm_pid":123}`))
	req.Host = "router.local"
	req.Header.Set("Origin", "http://router.local")
	req.Header.Set(adminMutationAuthorizationHeader, "spoofed")
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: token})
	securedModuleProxy(auth)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
	select {
	case ok := <-markerSeen:
		if !ok {
			t.Fatal("authorized proxy did not inject canonical internal marker")
		}
	case <-time.After(time.Second):
		t.Fatal("upstream marker observation timed out")
	}
}