package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminFileWriteBodyLimitIsPathScoped(t *testing.T) {
	writeReq := httptest.NewRequest(http.MethodPost, "/api/modules/admin/files/write", nil)
	if got, want := moduleMutationBodyLimitForRequest(writeReq, "admin"), adminFileWriteRequestBodyLimit; got != want {
		t.Fatalf("file write limit=%d want=%d", got, want)
	}

	processReq := httptest.NewRequest(http.MethodPost, "/api/modules/admin/processes/123/signal", nil)
	if got, want := moduleMutationBodyLimitForRequest(processReq, "admin"), adminModuleMutationBodyLimit; got != want {
		t.Fatalf("process mutation limit=%d want=%d", got, want)
	}

	readReq := httptest.NewRequest(http.MethodGet, "/api/modules/admin/files/read?path=%2Fopt%2Fetc%2Fconfig", nil)
	if got, want := moduleMutationBodyLimitForRequest(readReq, "admin"), adminModuleMutationBodyLimit; got != want {
		t.Fatalf("GET file limit=%d want legacy admin limit=%d", got, want)
	}
}

func TestAdminFileWriteBodyLimitRejectsAboveDedicatedLimit(t *testing.T) {
	body := strings.Repeat("x", int(adminFileWriteRequestBodyLimit)+1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/modules/admin/files/write", strings.NewReader(body))

	bounded, ok := boundedModuleMutationRequest(rec, req, "admin")
	if ok || bounded != nil {
		t.Fatal("oversized Admin file write request was accepted")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
}
