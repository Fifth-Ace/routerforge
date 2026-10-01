package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseAntiscanAuditLimit(t *testing.T) {
	if got, err := parseAntiscanAuditLimit(""); err != nil || got != antiscanAuditDefaultLimit {
		t.Fatalf("default got=%d err=%v", got, err)
	}
	if got, err := parseAntiscanAuditLimit("100"); err != nil || got != 100 {
		t.Fatalf("100 got=%d err=%v", got, err)
	}
	for _, raw := range []string{"0", "101", "wat"} {
		if _, err := parseAntiscanAuditLimit(raw); err == nil {
			t.Fatalf("limit %q accepted", raw)
		}
	}
}

func TestRecordAndReadAntiscanAuditLatestFirst(t *testing.T) {
	cfg := runtimeConfig{AuditLog: filepath.Join(t.TempDir(), "antiscan.jsonl")}
	for _, action := range []string{"first", "second", "third"} {
		if err := recordAntiscanAudit(cfg, antiscanAuditEvent{
			Timestamp:  time.Now().UTC(),
			Action:     action,
			Outcome:    "success",
			HTTPStatus: http.StatusOK,
			Summary:    action,
		}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := readAntiscanAudit(cfg, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 2 || page.Events[0].Action != "third" || page.Events[1].Action != "second" {
		t.Fatalf("unexpected history: %+v", page.Events)
	}
	if !page.Persistent || page.Skipped != 0 {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestAuditAntiscanMutationCapturesFailureWithoutConfigPayload(t *testing.T) {
	cfg := runtimeConfig{AuditLog: filepath.Join(t.TempDir(), "antiscan.jsonl")}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":              "reload failed",
			"rollback_performed": true,
			"warnings":           []string{"old config restored"},
		})
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/config", strings.NewReader(`{"confirm":"APPLY_CONFIG","base_sha256":"secret-ish","ports":"22"}`))
	response := httptest.NewRecorder()
	auditAntiscanMutation(cfg, "config", next).ServeHTTP(response, request)

	page, err := readAntiscanAudit(cfg, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 {
		t.Fatalf("events=%d", len(page.Events))
	}
	event := page.Events[0]
	if event.Action != "config" || event.Target != "ascn.conf" || event.Outcome != "failed" || event.HTTPStatus != http.StatusConflict || !event.Rollback {
		t.Fatalf("unexpected event: %+v", event)
	}
	if strings.Contains(event.Summary, "secret-ish") || strings.Contains(event.Target, "secret-ish") {
		t.Fatalf("config payload leaked into audit event: %+v", event)
	}
}

func TestAuditAntiscanMutationCapturesLifecycleTarget(t *testing.T) {
	cfg := runtimeConfig{AuditLog: filepath.Join(t.TempDir(), "antiscan.jsonl")}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"changed": true, "verified": true})
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/lifecycle", strings.NewReader(`{"action":"reload","confirm":"RELOAD"}`))
	response := httptest.NewRecorder()
	auditAntiscanMutation(cfg, "lifecycle", next).ServeHTTP(response, request)

	page, err := readAntiscanAudit(cfg, 10)
	if err != nil {
		t.Fatal(err)
	}
	event := page.Events[0]
	if event.Action != "lifecycle:reload" || event.Target != "reload" || event.Outcome != "success" || !event.Changed || !event.Verified {
		t.Fatalf("unexpected event: %+v", event)
	}
}
