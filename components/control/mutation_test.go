package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequestedSignalWhitelist(t *testing.T) {
	for _, name := range []string{"TERM", "SIGTERM", "HUP", "INT", "KILL"} {
		if _, _, ok := requestedSignal(name); !ok {
			t.Fatalf("signal %q unexpectedly rejected", name)
		}
	}
	if _, _, ok := requestedSignal("USR1"); ok {
		t.Fatal("USR1 must not be enabled in Phase 8A")
	}
}

func TestProcessSignalPathAndConfirmationContract(t *testing.T) {
	if pid, ok := parseProcessSignalPath("/v1/processes/123/signal"); !ok || pid != 123 {
		t.Fatalf("parse result pid=%d ok=%v", pid, ok)
	}
	for _, bad := range []string{
		"/v1/processes/1/kill",
		"/v1/processes/../signal",
		"/v1/processes/0/signal",
		"/v1/processes/abc/signal",
	} {
		if _, ok := parseProcessSignalPath(bad); ok {
			t.Fatalf("unsafe process path accepted: %s", bad)
		}
	}
}

func TestValidServiceIDRejectsTraversal(t *testing.T) {
	for _, good := range []string{"S99dummy", "S20-test_service", "S01foo.bar"} {
		if !validServiceID(good) {
			t.Fatalf("valid service id rejected: %s", good)
		}
	}
	for _, bad := range []string{"", "dummy", "../S99dummy", "S99/foo", "S99foo bar"} {
		if validServiceID(bad) {
			t.Fatalf("unsafe service id accepted: %s", bad)
		}
	}
}

func TestMutationOnlyRequiresCoreAuthorizationMarker(t *testing.T) {
	handler := mutationOnly(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/processes/123/signal", strings.NewReader(`{}`))
	handler(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d", rec.Code, http.StatusForbidden)
	}
}

func TestServiceActionUsesExactInitScriptWithoutShell(t *testing.T) {
	tmp := t.TempDir()
	oldDir := serviceInitDir
	serviceInitDir = tmp
	t.Cleanup(func() { serviceInitDir = oldDir })

	result := filepath.Join(tmp, "action.txt")
	script := filepath.Join(tmp, "S99dummy")
	body := "#!/bin/sh\nprintf '%s' \"$1\" > '" + result + "'\n"
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/services/S99dummy/action", strings.NewReader(`{"action":"restart","confirm_id":"S99dummy"}`))
	req.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
	mutationOnly(handleServiceAction)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(result)
	if err != nil {
		t.Fatalf("read action result: %v", err)
	}
	if string(got) != "restart" {
		t.Fatalf("action=%q want restart", string(got))
	}
}

func TestRouterForgeServiceMutationProtected(t *testing.T) {
	service := serviceInfo{ID: "S91routerforge-admin", Name: "routerforge-admin", Path: "/opt/etc/init.d/S91routerforge-admin"}
	if !protectedServiceMutation(service) {
		t.Fatal("RouterForge service must be protected from Admin service actions")
	}
}
func TestServiceActionResponseIncludesCommittedTransaction(t *testing.T) {
	tmp := t.TempDir()
	oldDir := serviceInitDir
	serviceInitDir = tmp
	t.Cleanup(func() { serviceInitDir = oldDir })

	script := filepath.Join(tmp, "S99evidence")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/services/S99evidence/action",
		strings.NewReader(`{"action":"start","confirm_id":"S99evidence"}`),
	)
	req.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
	mutationOnly(handleServiceAction)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Transaction struct {
			State    string `json:"state"`
			Evidence []struct {
				Stage  string `json:"stage"`
				Status string `json:"status"`
			} `json:"evidence"`
		} `json:"transaction"`
		Rollback string `json:"rollback"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Transaction.State != "committed" {
		t.Fatalf("transaction state=%q want committed", response.Transaction.State)
	}
	if response.Rollback != "not-supported-generic-service-lifecycle" {
		t.Fatalf("rollback contract=%q", response.Rollback)
	}

	required := map[string]bool{
		"precheck":             false,
		"target-precheck":      false,
		"snapshot":             false,
		"runtime-snapshot":     false,
		"validated":            false,
		"action-validation":    false,
		"applied":              false,
		"service-action":       false,
		"post-action-readback": false,
		"verified":             false,
		"committed":            false,
	}
	for _, evidence := range response.Transaction.Evidence {
		if _, exists := required[evidence.Stage]; exists {
			required[evidence.Stage] = true
		}
	}
	for stage, found := range required {
		if !found {
			t.Fatalf("missing transaction evidence stage %q: %+v", stage, response.Transaction.Evidence)
		}
	}
}
