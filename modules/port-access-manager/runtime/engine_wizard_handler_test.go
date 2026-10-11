package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEngineWizardPreviewAuthorizationAndValidation(t *testing.T) {
	valid, err := json.Marshal(exampleWizard())
	if err != nil {
		t.Fatal(err)
	}
	call := func(data []byte, authorized bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/engine-wizard-preview", bytes.NewReader(data))
		if authorized {
			req.Header.Set("X-RouterForge-Module-Authorized", "core-authorized-v1")
		}
		w := httptest.NewRecorder()
		engineWizardPreviewHandler(w, req)
		return w
	}
	if got := call(valid, false).Code; got != 403 {
		t.Fatalf("unauthorized: %d", got)
	}
	success := call(valid, true)
	if success.Code != 200 {
		t.Fatalf("valid: %d %s", success.Code, success.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(success.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["applied"] != false || result["preview_only"] != true {
		t.Fatal("preview claimed application")
	}
	req := exampleWizard()
	req.Services[0].Address = "127.0.0.1"
	bad, _ := json.Marshal(req)
	if got := call(bad, true).Code; got != 422 {
		t.Fatalf("unsafe destination: %d", got)
	}
	if got := call(append(valid, []byte(" {}")...), true).Code; got != 400 {
		t.Fatalf("trailing JSON: %d", got)
	}
}
