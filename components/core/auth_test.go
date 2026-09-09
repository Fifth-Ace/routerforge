package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestAuthMiddlewareAllowsOnlyLoopbackModuleHealthWithoutSession(t *testing.T) {
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
	handler := a.middleware(next)

	tests := []struct {
		name       string
		method     string
		path       string
		remoteAddr string
		wantHit    bool
	}{
		{name: "loopback get health", method: http.MethodGet, path: "/api/modules/dns/health", remoteAddr: "127.0.0.1:41000", wantHit: true},
		{name: "loopback head health", method: http.MethodHead, path: "/api/modules/dns/health", remoteAddr: "127.0.0.1:41001", wantHit: true},
		{name: "lan get health", method: http.MethodGet, path: "/api/modules/dns/health", remoteAddr: "192.168.10.55:41002", wantHit: false},
		{name: "loopback post health", method: http.MethodPost, path: "/api/modules/dns/health", remoteAddr: "127.0.0.1:41003", wantHit: false},
		{name: "loopback other module api", method: http.MethodGet, path: "/api/modules/dns/info", remoteAddr: "127.0.0.1:41004", wantHit: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit = false
			r := httptest.NewRequest(tt.method, "http://router"+tt.path, nil)
			r.RemoteAddr = tt.remoteAddr
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if hit != tt.wantHit {
				t.Fatalf("hit=%v, want %v; status=%d", hit, tt.wantHit, w.Code)
			}
			if tt.wantHit && w.Code != http.StatusNoContent {
				t.Fatalf("allowed health status=%d, want %d", w.Code, http.StatusNoContent)
			}
			if !tt.wantHit && w.Code != http.StatusUnauthorized {
				t.Fatalf("blocked request status=%d, want %d", w.Code, http.StatusUnauthorized)
			}
		})
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

func TestAuthLoginAttemptsPruneStaleEntries(t *testing.T) {
	now := time.Now()
	a := &authManager{
		attempts: map[string]loginAttempt{
			"stale": {
				WindowStart: now.Add(-authLoginAttemptWindow - time.Second),
			},
			"fresh": {
				WindowStart: now.Add(-time.Minute),
			},
			"blocked": {
				WindowStart:  now.Add(-authLoginAttemptWindow - time.Minute),
				BlockedUntil: now.Add(time.Minute),
			},
		},
	}

	a.mu.Lock()
	a.cleanupLoginAttemptsLocked(now, "")
	a.mu.Unlock()

	if _, ok := a.attempts["stale"]; ok {
		t.Fatal("stale login attempt was retained")
	}
	if _, ok := a.attempts["fresh"]; !ok {
		t.Fatal("fresh login attempt was pruned")
	}
	if _, ok := a.attempts["blocked"]; !ok {
		t.Fatal("active block was pruned")
	}
}

func TestAuthLoginAttemptsAreStrictlyBounded(t *testing.T) {
	now := time.Now()
	a := &authManager{attempts: make(map[string]loginAttempt)}
	for i := 0; i < authLoginAttemptMaxEntries; i++ {
		a.attempts[fmt.Sprintf("client-%04d", i)] = loginAttempt{
			WindowStart: now.Add(-time.Duration(i%120) * time.Second),
		}
	}

	a.recordLoginFailure("current-client")

	if got := len(a.attempts); got != authLoginAttemptMaxEntries {
		t.Fatalf("attempt map size=%d, want %d", got, authLoginAttemptMaxEntries)
	}
	attempt, ok := a.attempts["current-client"]
	if !ok {
		t.Fatal("current client was evicted during its own cleanup")
	}
	if attempt.Failures != 1 {
		t.Fatalf("current client failures=%d, want 1", attempt.Failures)
	}
}

func TestAuthLoginAttemptsPreferActiveBlocksDuringEviction(t *testing.T) {
	now := time.Now()
	a := &authManager{attempts: make(map[string]loginAttempt)}
	for i := 0; i < authLoginAttemptMaxEntries; i++ {
		a.attempts[fmt.Sprintf("client-%04d", i)] = loginAttempt{
			WindowStart: now.Add(-time.Duration(i%120) * time.Second),
		}
	}
	a.attempts["blocked-client"] = loginAttempt{
		WindowStart:  now.Add(-time.Minute),
		BlockedUntil: now.Add(time.Minute),
	}

	a.mu.Lock()
	a.cleanupLoginAttemptsLocked(now, "")
	a.mu.Unlock()

	if got := len(a.attempts); got != authLoginAttemptMaxEntries {
		t.Fatalf("attempt map size=%d, want %d", got, authLoginAttemptMaxEntries)
	}
	if _, ok := a.attempts["blocked-client"]; !ok {
		t.Fatal("active blocked client was evicted before an unblocked attempt")
	}
}

func TestAuthLoginFailureStillBlocksAfterFiveFailures(t *testing.T) {
	a := &authManager{attempts: make(map[string]loginAttempt)}
	for i := 0; i < 5; i++ {
		a.recordLoginFailure("client")
	}

	wait := a.loginBlockedFor("client")
	if wait <= 0 {
		t.Fatal("client was not blocked after five failures")
	}
	if wait > authLoginAttemptBlockTTL {
		t.Fatalf("block duration=%s exceeds configured ttl %s", wait, authLoginAttemptBlockTTL)
	}
}
