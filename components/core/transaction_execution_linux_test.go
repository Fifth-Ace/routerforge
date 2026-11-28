//go:build linux

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecutionRequestStrictBinding(t *testing.T) {
	id := strings.Repeat("1", 32)
	hash := strings.Repeat("b", 64)
	commit := strings.Repeat("a", 40)
	body := `{"id":"` + id + `","expected_commit":"` + commit + `","packages":["routerforge-core"],"plan_sha256":"` + hash + `","confirm":"EXECUTE:` + hash + `"}`
	for _, v := range []struct {
		raw string
		ok  bool
	}{
		{body, true}, {strings.Replace(body, `"confirm":"EXECUTE:`, `"confirm":"OTHER:`, 1), false},
		{strings.Replace(body, `"id":"`, `"id":"duplicate","id":"`, 1), false},
		{strings.Replace(body, `"packages":`, `"command":"opkg","packages":`, 1), false},
		{body + ` {}`, false}, {strings.Replace(body, id, "bad", 1), false},
		{strings.Replace(body, commit, "bad", 1), false}, {strings.Replace(body, `["routerforge-core"]`, `["routerforge-core","routerforge-core"]`, 1), false},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/apps/transactions/start", strings.NewReader(v.raw))
		r.Header.Set("Content-Type", "application/json")
		_, err := decodeTransactionStart(httptest.NewRecorder(), r)
		if (err == nil) != v.ok {
			t.Fatalf("unexpected validation %v / %v", v.ok, err)
		}
	}
}
func TestExecutionRouteRequiresAuthAndAcceptance(t *testing.T) {
	for _, v := range []struct {
		method, origin string
		auth           *authManager
		want           int
	}{
		{http.MethodGet, "http://example.com", &authManager{}, http.StatusMethodNotAllowed},
		{http.MethodPost, "http://example.com", &authManager{}, http.StatusForbidden},
		{http.MethodPost, "http://foreign.example", &authManager{config: securityConfig{AuthRequired: true}}, http.StatusForbidden},
	} {
		mux := http.NewServeMux()
		registerCoreAppTransactionHandlers(mux, v.auth)
		r := httptest.NewRequest(v.method, "http://example.com/api/apps/transactions/start", strings.NewReader(`{}`))
		r.Header.Set("Origin", v.origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != v.want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if coreExecutionConfigured() {
		t.Fatal("unaccepted execution enabled")
	}
}
func TestExecutionRebuildRejectsChangedHash(t *testing.T) {
	req, doc, status, items := coreTransactionFixture()
	id := strings.Repeat("1", 32)
	p, err := buildCoreTransactionPreview(id, req, doc, status, items)
	if err != nil {
		t.Fatal(err)
	}
	start := appTransactionStartRequest{ID: id, ExpectedCommit: req.ExpectedCommit, Packages: req.Packages, PlanSHA256: p.PlanSHA256, Confirm: "EXECUTE:" + p.PlanSHA256}
	if _, err = confirmedTransactionPlan(start, doc, status, items); err != nil {
		t.Fatal(err)
	}
	start.PlanSHA256 = strings.Repeat("b", 64)
	if _, err = confirmedTransactionPlan(start, doc, status, items); err == nil {
		t.Fatal("changed digest accepted")
	}
}

func TestAuthenticatedRootStillLockedWithoutDeviceApproval(t *testing.T) {
	auth := &authManager{config: securityConfig{AuthRequired: true}, sessions: map[string]authSession{}}
	cookieWriter := httptest.NewRecorder()
	if err := auth.createSession(cookieWriter, "root"); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerCoreAppTransactionHandlers(mux, auth)
	for _, v := range []struct {
		origin string
		want   int
	}{{"http://example.com", 409}, {"", 403}, {"http://other.example", 403}} {
		r := httptest.NewRequest(http.MethodPost, "http://example.com/api/apps/transactions/start", strings.NewReader(`{}`))
		r.Header.Set("Origin", v.origin)
		r.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookieWriter.Result().Cookies() {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != v.want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
