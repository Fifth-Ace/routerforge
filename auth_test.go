package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestReadPasswordHashFrom(t *testing.T) {
	dir := t.TempDir()
	shadow := filepath.Join(dir, "shadow")
	passwd := filepath.Join(dir, "passwd")
	if err := os.WriteFile(shadow, []byte("root:$6$saltstring$hash:1:2:3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(passwd, []byte("root:x:0:0:root:/root:/bin/sh\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := readPasswordHashFrom("root", shadow, passwd)
	if err != nil {
		t.Fatal(err)
	}
	if got != "$6$saltstring$hash" {
		t.Fatalf("got %q", got)
	}
}

func TestReadPasswordHashFallsBackFromPasswdMarker(t *testing.T) {
	dir := t.TempDir()
	shadow := filepath.Join(dir, "shadow")
	passwd := filepath.Join(dir, "passwd")
	if err := os.WriteFile(shadow, []byte("root:x:1:2:3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(passwd, []byte("root:$1$deadbeef$Q7g0UO4hRC0mgQUQ/qkjZ0:0:0:root:/opt/root:/opt/bin/sh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readPasswordHashFrom("root", shadow, passwd)
	if err != nil {
		t.Fatal(err)
	}
	if got != "$1$deadbeef$Q7g0UO4hRC0mgQUQ/qkjZ0" {
		t.Fatalf("got %q", got)
	}
}

func TestLockedPasswordRejected(t *testing.T) {
	if ok, err := verifyUnixCrypt("!locked", "anything"); err == nil || ok {
		t.Fatalf("locked password accepted: ok=%v err=%v", ok, err)
	}
}

func TestAuthMiddlewareBlocksProtectedAPI(t *testing.T) {
	a := &authManager{
		config:   securityConfig{AuthRequired: true},
		sessions: make(map[string]authSession),
		attempts: make(map[string]loginAttempt),
	}
	hit := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusNoContent)
	})

	r := httptest.NewRequest(http.MethodGet, "http://router/api/snapshot", nil)
	w := httptest.NewRecorder()
	a.middleware(next).ServeHTTP(w, r)
	if hit {
		t.Fatal("protected API reached without a session")
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d", w.Code)
	}

	hit = false
	r = httptest.NewRequest(http.MethodGet, "http://router/api/auth/status", nil)
	w = httptest.NewRecorder()
	a.middleware(next).ServeHTTP(w, r)
	if !hit || w.Code != http.StatusNoContent {
		t.Fatalf("public auth status was blocked: hit=%v status=%d", hit, w.Code)
	}
}

func TestAuthSessionAllowsProtectedAPI(t *testing.T) {
	a := &authManager{
		config:   securityConfig{AuthRequired: true},
		sessions: make(map[string]authSession),
		attempts: make(map[string]loginAttempt),
	}
	loginRecorder := httptest.NewRecorder()
	if err := a.createSession(loginRecorder, "root"); err != nil {
		t.Fatal(err)
	}
	response := loginRecorder.Result()
	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one session cookie, got %d", len(cookies))
	}
	if !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("session cookie security flags are missing")
	}

	hit := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusNoContent)
	})
	r := httptest.NewRequest(http.MethodGet, "http://router/api/snapshot", nil)
	r.AddCookie(cookies[0])
	w := httptest.NewRecorder()
	a.middleware(next).ServeHTTP(w, r)
	if !hit || w.Code != http.StatusNoContent {
		t.Fatalf("valid session was rejected: hit=%v status=%d", hit, w.Code)
	}
}
