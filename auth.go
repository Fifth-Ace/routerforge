package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	securityConfigPath = "/opt/etc/routerforge/security.json"
	authCookieName     = "routerforge_session"
	authSessionTTL     = 12 * time.Hour
)

type securityConfig struct {
	AuthRequired bool `json:"auth_required"`
}

type authSession struct {
	User      string
	ExpiresAt time.Time
}

type loginAttempt struct {
	WindowStart  time.Time
	Failures     int
	BlockedUntil time.Time
}

type authManager struct {
	mu       sync.Mutex
	config   securityConfig
	sessions map[string]authSession
	attempts map[string]loginAttempt
}

type authStatus struct {
	Required      bool   `json:"required"`
	Authenticated bool   `json:"authenticated"`
	User          string `json:"user,omitempty"`
	Backend       string `json:"backend"`
	SessionHours  int    `json:"session_hours"`
}

func newAuthManager() *authManager {
	a := &authManager{
		sessions: make(map[string]authSession),
		attempts: make(map[string]loginAttempt),
	}
	if data, err := os.ReadFile(securityConfigPath); err == nil {
		if json.Unmarshal(data, &a.config) != nil {
			// Existing but malformed security config fails closed.
			a.config.AuthRequired = true
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		// Core normally runs as root. A security file that exists but cannot be
		// read must never silently disable authentication.
		a.config.AuthRequired = true
	}
	return a
}

func (a *authManager) authRequired() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config.AuthRequired
}

func (a *authManager) status(r *http.Request) authStatus {
	required := a.authRequired()
	user, authenticated := a.sessionUser(r)
	if !required {
		authenticated = true
	}
	return authStatus{
		Required:      required,
		Authenticated: authenticated,
		User:          user,
		Backend:       "entware-root",
		SessionHours:  int(authSessionTTL / time.Hour),
	}
}

func (a *authManager) sessionUser(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(authCookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	session, ok := a.sessions[cookie.Value]
	if !ok {
		return "", false
	}
	if !session.ExpiresAt.After(now) {
		delete(a.sessions, cookie.Value)
		return "", false
	}
	return session.User, true
}

func (a *authManager) createSession(w http.ResponseWriter, user string) error {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expires := time.Now().Add(authSessionTTL)

	a.mu.Lock()
	a.sessions[token] = authSession{User: user, ExpiresAt: expires}
	a.cleanupSessionsLocked(time.Now())
	a.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
		MaxAge:   int(authSessionTTL / time.Second),
	})
	return nil
}

func (a *authManager) clearSession(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(authCookieName); err == nil && cookie.Value != "" {
		a.mu.Lock()
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}

func (a *authManager) cleanupSessionsLocked(now time.Time) {
	for token, session := range a.sessions {
		if !session.ExpiresAt.After(now) {
			delete(a.sessions, token)
		}
	}
}

func (a *authManager) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || authPublicPath(r.URL.Path) || !a.authRequired() {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := a.sessionUser(r); !ok {
			writeAuthJSON(w, http.StatusUnauthorized, map[string]any{
				"error":         "authentication required",
				"auth_required": true,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authPublicPath(path string) bool {
	switch path {
	case "/api/auth/status", "/api/auth/login", "/api/auth/logout", "/api/auth/config", "/api/health":
		return true
	default:
		return false
	}
}

func (a *authManager) registerHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/status", a.handleStatus)
	mux.HandleFunc("/api/auth/login", a.handleLogin)
	mux.HandleFunc("/api/auth/logout", a.handleLogout)
	mux.HandleFunc("/api/auth/config", a.handleConfig)
}

func (a *authManager) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowedJSON(w, http.MethodGet)
		return
	}
	writeAuthJSON(w, http.StatusOK, a.status(r))
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *authManager) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowedJSON(w, http.MethodPost)
		return
	}
	if !sameOriginRequest(r) {
		writeAuthJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin request rejected"})
		return
	}

	client := authClientKey(r)
	if wait := a.loginBlockedFor(client); wait > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(wait.Seconds())+1))
		writeAuthJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many failed login attempts"})
		return
	}

	var request loginRequest
	if err := decodeSmallJSON(w, r, &request); err != nil {
		writeAuthJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid login request"})
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	ok, err := verifyEntwareRoot(request.Username, request.Password)
	if err != nil || !ok {
		a.recordLoginFailure(client)
		writeAuthJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid Entware root credentials"})
		return
	}
	if err := a.createSession(w, "root"); err != nil {
		writeAuthJSON(w, http.StatusInternalServerError, map[string]any{"error": "session creation failed"})
		return
	}
	a.clearLoginFailures(client)
	writeAuthJSON(w, http.StatusOK, map[string]any{"ok": true, "user": "root"})
}

func (a *authManager) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowedJSON(w, http.MethodPost)
		return
	}
	if !sameOriginRequest(r) {
		writeAuthJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin request rejected"})
		return
	}
	a.clearSession(w, r)
	writeAuthJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type authConfigRequest struct {
	Required bool   `json:"required"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func (a *authManager) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowedJSON(w, http.MethodPost)
		return
	}
	if !sameOriginRequest(r) {
		writeAuthJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin request rejected"})
		return
	}

	var request authConfigRequest
	if err := decodeSmallJSON(w, r, &request); err != nil {
		writeAuthJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid auth configuration request"})
		return
	}

	currentlyRequired := a.authRequired()
	if request.Required {
		if !currentlyRequired {
			ok, err := verifyEntwareRoot(strings.TrimSpace(request.Username), request.Password)
			if err != nil || !ok {
				writeAuthJSON(w, http.StatusUnauthorized, map[string]any{"error": "valid Entware root credentials are required before enabling authentication"})
				return
			}
			if err := a.saveConfig(securityConfig{AuthRequired: true}); err != nil {
				writeAuthJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot save security settings"})
				return
			}
			if err := a.createSession(w, "root"); err != nil {
				writeAuthJSON(w, http.StatusInternalServerError, map[string]any{"error": "session creation failed"})
				return
			}
			writeAuthJSON(w, http.StatusOK, authStatus{Required: true, Authenticated: true, User: "root", Backend: "entware-root", SessionHours: int(authSessionTTL / time.Hour)})
			return
		}
		writeAuthJSON(w, http.StatusOK, a.status(r))
		return
	}

	if currentlyRequired {
		if _, ok := a.sessionUser(r); !ok {
			writeAuthJSON(w, http.StatusUnauthorized, map[string]any{"error": "authentication required to disable authentication"})
			return
		}
	}
	if err := a.saveConfig(securityConfig{AuthRequired: false}); err != nil {
		writeAuthJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot save security settings"})
		return
	}
	a.clearSession(w, r)
	writeAuthJSON(w, http.StatusOK, authStatus{Required: false, Authenticated: true, Backend: "entware-root", SessionHours: int(authSessionTTL / time.Hour)})
}

func (a *authManager) saveConfig(config securityConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(securityConfigPath), 0755); err != nil {
		return err
	}
	tmp := securityConfigPath + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, securityConfigPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	a.mu.Lock()
	a.config = config
	if !config.AuthRequired {
		a.sessions = make(map[string]authSession)
	}
	a.mu.Unlock()
	return nil
}

func (a *authManager) loginBlockedFor(client string) time.Duration {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	attempt := a.attempts[client]
	if attempt.BlockedUntil.After(now) {
		return attempt.BlockedUntil.Sub(now)
	}
	if !attempt.WindowStart.IsZero() && now.Sub(attempt.WindowStart) > 5*time.Minute {
		delete(a.attempts, client)
	}
	return 0
}

func (a *authManager) recordLoginFailure(client string) {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	attempt := a.attempts[client]
	if attempt.WindowStart.IsZero() || now.Sub(attempt.WindowStart) > 5*time.Minute {
		attempt = loginAttempt{WindowStart: now}
	}
	attempt.Failures++
	if attempt.Failures >= 5 {
		attempt.BlockedUntil = now.Add(30 * time.Second)
		attempt.Failures = 0
		attempt.WindowStart = now
	}
	a.attempts[client] = attempt
}

func (a *authManager) clearLoginFailures(client string) {
	a.mu.Lock()
	delete(a.attempts, client)
	a.mu.Unlock()
}

func authClientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func sameOriginRequest(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, r.Host)
}

func decodeSmallJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func methodNotAllowedJSON(w http.ResponseWriter, method string) {
	w.Header().Set("Allow", method)
	writeAuthJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": method + " required"})
}

func writeAuthJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func verifyEntwareRoot(username, password string) (bool, error) {
	if username != "root" {
		return false, nil
	}
	hash, err := readPasswordHashFrom(username, "/opt/etc/shadow", "/opt/etc/passwd")
	if err != nil {
		return false, err
	}
	return verifyUnixCrypt(hash, password)
}

func readPasswordHashFrom(username string, paths ...string) (string, error) {
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Split(scanner.Text(), ":")
			if len(fields) >= 2 && fields[0] == username {
				_ = file.Close()
				if fields[1] == "x" {
					break
				}
				return fields[1], nil
			}
		}
		scanErr := scanner.Err()
		_ = file.Close()
		if scanErr != nil {
			return "", scanErr
		}
	}
	return "", errors.New("Entware root password hash not found")
}
