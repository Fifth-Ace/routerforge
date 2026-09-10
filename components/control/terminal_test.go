package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func runTerminalRequest(t *testing.T, body adminTerminalRunRequest) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/terminal/run", bytes.NewReader(payload))
	req.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
	mutationOnly(handleAdminTerminalRun)(rec, req)
	return rec
}

func TestAdminTerminalRunsInAllowedDirectory(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)

	rec := runTerminalRequest(t, adminTerminalRunRequest{
		Command: "pwd; printf terminal-ok",
		Cwd:     root,
		Confirm: "RUN",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		OK       bool   `json:"ok"`
		Output   string `json:"output"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.ExitCode != 0 {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if !strings.Contains(body.Output, filepath.Clean(root)) || !strings.Contains(body.Output, "terminal-ok") {
		t.Fatalf("output=%q", body.Output)
	}
}

func TestAdminTerminalRequiresExactConfirmation(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)

	rec := runTerminalRequest(t, adminTerminalRunRequest{
		Command: "true",
		Cwd:     root,
		Confirm: "run",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminTerminalRejectsOutsideAllowedRoot(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)

	rec := runTerminalRequest(t, adminTerminalRunRequest{
		Command: "true",
		Cwd:     "/",
		Confirm: "RUN",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminTerminalRejectsOversizedCommand(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)

	rec := runTerminalRequest(t, adminTerminalRunRequest{
		Command: strings.Repeat("x", adminTerminalCommandLimit+1),
		Cwd:     root,
		Confirm: "RUN",
	})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
