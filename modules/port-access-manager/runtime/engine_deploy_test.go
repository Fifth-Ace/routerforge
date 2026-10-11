package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEngineDeployRequiresCoreAuthorization(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/engine-deploy", bytes.NewBufferString(`{"engine":"knockd","action":"enable","confirm":"APPLY"}`))
	result := httptest.NewRecorder()
	engineRequestHandler(result, req)
	if result.Code != http.StatusForbidden {
		t.Fatalf("unauthorized operation returned %d", result.Code)
	}
}
func TestEngineDeployRejectsUnknownAndTrailingJSON(t *testing.T) {
	for _, body := range []string{`{"engine":"odin","action":"enable","confirm":"APPLY"}`, `{"engine":"knockd","action":"enable","confirm":"APPLY","unknown":"x"}`, `{"engine":"knockd","action":"enable","confirm":"APPLY"}{}`} {
		r := httptest.NewRequest(http.MethodPost, "/v1/engine-deploy", strings.NewReader(body))
		r.Header.Set("X-RouterForge-Module-Authorized", "core-authorized-v1")
		w := httptest.NewRecorder()
		engineRequestHandler(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("bad input %q accepted with %d", body, w.Code)
		}
	}
}
func TestEngineDeployStatusNoSecrets(t *testing.T) {
	for _, engine := range []string{"knockd", "fwknopd"} {
		r := httptest.NewRequest(http.MethodGet, "/v1/engine-deploy?engine="+engine, nil)
		w := httptest.NewRecorder()
		engineRequestHandler(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		var d map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		if _, ok := d["credential_once"]; ok {
			t.Fatal("GET includes credentials")
		}
	}
}
func TestGeneratedEngineConfigAndGateScope(t *testing.T) {
	req := exampleWizard()
	files, creds, err := generatedConfig(req)
	if err != nil {
		t.Fatal(err)
	}
	knock := string(files["/opt/etc/knockd.conf"])
	if creds != "" || !strings.Contains(knock, "Start_Command = /bin/sh "+engineScript+" authorize knockd %IP%") {
		t.Fatal("knockd authorization helper not configured")
	}
	gate := string(engineGateConfig(req))
	for _, s := range []string{"WAN=eth3", "SERVICE=192.168.10.103:18080", "SERVICE=192.168.10.104:22", "TTL=120"} {
		if !strings.Contains(gate, s) {
			t.Fatalf("missing gate restriction %q", s)
		}
	}
	req.Engine = "fwknopd"
	req.Knock = nil
	files, creds, err = generatedConfig(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"KEY_BASE64 ", "HMAC_KEY_BASE64 ", "OPEN_PORTS tcp/18080,tcp/22"} {
		if !strings.Contains(string(files["/opt/etc/fwknop/access.conf"]), s) {
			t.Fatalf("missing %s", s)
		}
	}
	fwcfg := string(files["/opt/etc/fwknop/fwknopd.conf"])
	if !strings.Contains(fwcfg, "ENABLE_IPT_FORWARDING Y") || !strings.Contains(fwcfg, "IPT_FORWARD_ACCESS ACCEPT,filter,RFPA_F_GATE,1,FWKNOP_FORWARD,1") {
		t.Fatal("SPA chain is not scoped through RouterForge gate")
	}
	if strings.Contains(fwcfg, "IPT_FORWARD_ACCESS ACCEPT,filter,FORWARD,1") {
		t.Fatal("global FORWARD hook bypasses per-service scope")
	}
	if !strings.Contains(creds, "ENCRYPTION_KEY_BASE64=") || !strings.Contains(creds, "HMAC_KEY_BASE64=") {
		t.Fatal("SPA client keys missing")
	}
}
