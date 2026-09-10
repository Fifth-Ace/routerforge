package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAdminModuleFileRequestExactPath(t *testing.T) {
	good := []string{
		"/api/modules/admin/files",
		"/api/modules/admin/files/list",
		"/api/modules/ADMIN/files/read",
	}
	for _, path := range good {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if !adminModuleFileRequest(req) {
			t.Fatalf("file route not detected: %s", path)
		}
	}

	bad := []string{
		"/api/modules/admin/file",
		"/api/modules/admin/filesystem",
		"/api/modules/dns/files/list",
		"/api/admin/files/list",
	}
	for _, path := range bad {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if adminModuleFileRequest(req) {
			t.Fatalf("non-file route detected as Admin file access: %s", path)
		}
	}
}

func TestSecuredModuleProxyRequiresRootSessionForAdminFiles(t *testing.T) {
	auth, _ := testAdminAuthManager()
	auth.config.AuthRequired = true
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/modules/admin/files/list?path=%2Fopt", nil)
	req.Host = "router.local"
	req.Header.Set("Origin", "http://router.local")

	securedModuleProxy(auth)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestSecuredModuleProxyRejectsCrossOriginAdminFiles(t *testing.T) {
	auth, token := testAdminAuthManager()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/modules/admin/files/list?path=%2Fopt", nil)
	req.Host = "router.local"
	req.Header.Set("Origin", "http://evil.local")
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: token})

	securedModuleProxy(auth)(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestSecuredModuleProxyInjectsCanonicalMarkerForRootFileAccess(t *testing.T) {
	socketDir, err := os.MkdirTemp(os.TempDir(), "rfu-")
	if err != nil {
		t.Fatalf("short unix socket temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })

	socket := filepath.Join(socketDir, "a.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	type observation struct {
		marker string
		path   string
	}
	observed := make(chan observation, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed <- observation{marker: r.Header.Get(adminMutationAuthorizationHeader), path: r.URL.Path}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	oldSockets := moduleSockets
	moduleSockets = map[string][]string{"admin": {socket}}
	t.Cleanup(func() { moduleSockets = oldSockets })

	auth, _ := testAdminAuthManager()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/modules/admin/files/list?path=%2Fopt", nil)
	req.Host = "router.local"
	req.Header.Set("Origin", "http://router.local")
	req.Header.Set(adminMutationAuthorizationHeader, "spoofed")

	securedModuleProxy(auth)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}

	select {
	case got := <-observed:
		if got.marker != adminMutationAuthorizationValue {
			t.Fatalf("marker=%q want canonical value", got.marker)
		}
		if got.path != "/v1/files/list" {
			t.Fatalf("upstream path=%q want=/v1/files/list", got.path)
		}
	case <-time.After(time.Second):
		t.Fatal("upstream file-access observation timed out")
	}
}

func TestSecuredModuleProxyStillInjectsCanonicalMarkerForAdminMutation(t *testing.T) {
	socketDir, err := os.MkdirTemp(os.TempDir(), "rfm-")
	if err != nil {
		t.Fatalf("short unix socket temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })

	socket := filepath.Join(socketDir, "m.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	observed := make(chan string, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed <- r.Header.Get(adminMutationAuthorizationHeader)
		w.WriteHeader(http.StatusNoContent)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	oldSockets := moduleSockets
	moduleSockets = map[string][]string{"admin": {socket}}
	t.Cleanup(func() { moduleSockets = oldSockets })

	auth, token := testAdminAuthManager()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/modules/admin/services/S99awg-manager/action",
		strings.NewReader(`{"action":"restart","confirm_id":"S99awg-manager"}`),
	)
	req.Host = "router.local"
	req.Header.Set("Origin", "http://router.local")
	req.Header.Set(adminMutationAuthorizationHeader, "spoofed")
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: token})

	securedModuleProxy(auth)(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	select {
	case marker := <-observed:
		if marker != adminMutationAuthorizationValue {
			t.Fatalf("marker=%q want canonical value", marker)
		}
	case <-time.After(time.Second):
		t.Fatal("upstream mutation marker observation timed out")
	}
}
