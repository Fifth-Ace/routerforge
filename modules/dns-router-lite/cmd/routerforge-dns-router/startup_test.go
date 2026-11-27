package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestE15ACStartupGate(t *testing.T) {
	calls := 0
	g := &startupGate{build: "test", next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(204) })}
	request := func(method, path string, want int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		g.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != want {
			t.Fatalf("%s %s: %d != %d", method, path, w.Code, want)
		}
		return w
	}
	check := func(want string) {
		t.Helper()
		w := request("GET", "/v1/startup", 200)
		var s startupStatus
		if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		got, err := checkedStartupState(s, "test")
		if err != nil || got != want {
			t.Fatalf("%+v %v", s, err)
		}
	}
	check("STARTING")
	request("POST", "/v1/startup", 405)
	request("POST", "/v1/service/control", 503)
	request("GET", "/v1/health", 503)
	if calls != 0 {
		t.Fatal("handler reached during startup")
	}
	g.complete(nil, nil)
	check("READY")
	request("GET", "/v1/health", 204)
	g.complete(errors.New("resume failed"), nil)
	check("DEGRADED")
	request("GET", "/v1/service/status", 204)
	g.complete(nil, errors.New("profile failed"))
	check("DEGRADED")
}
func TestE15ACStartupIntent(t *testing.T) {
	var s startupServiceStatus
	s.Desired.Version = 1
	s.Connection.State = "disconnected"
	if !s.ready() {
		t.Fatal("intent OFF must be ready without listener")
	}
	s.Desired.Enabled = true
	if s.ready() {
		t.Fatal("missing enabled listener")
	}
	s.Active = true
	s.Revision = "a"
	s.Desired.Revision = "b"
	if s.ready() {
		t.Fatal("wrong applied revision")
	}
	s.Desired.Revision = "a"
	if !s.ready() {
		t.Fatal("enabled applied revision must be ready")
	}
	s.Desired.Interception = true
	if s.ready() {
		t.Fatal("missing desired interception")
	}
	s.Interception = true
	if !s.ready() {
		t.Fatal("matching interception must be ready")
	}
	s.Connection.State = "pending"
	if s.ready() {
		t.Fatal("pending client profile")
	}
	s.Connection.State = "connected"
	s.Maintenance = true
	if s.ready() {
		t.Fatal("maintenance not ready")
	}
	s.Maintenance = false
	s.LastError = "failure"
	if s.ready() {
		t.Fatal("runtime error not ready")
	}
}
func TestE15ACStartupIdentity(t *testing.T) {
	for _, s := range []startupStatus{{"other", "test", "READY"}, {"dns-router", "old", "READY"}, {"dns-router", "test", "unknown"}} {
		if _, err := checkedStartupState(s, "test"); err == nil {
			t.Fatal("accepted invalid startup identity")
		}
	}
}
