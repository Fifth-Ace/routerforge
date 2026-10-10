package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func exampleRule() managedKnockRule {
	return managedKnockRule{ID: "test-access", Engine: "iptables-recent", Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120, WAN: "eth0", Enabled: false}
}
func callStage(t *testing.T, h http.Handler, method string, body any, header bool) *httptest.ResponseRecorder {
	t.Helper()
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(method, "/v1/staged-rules", bytes.NewReader(data))
	if header {
		req.Header.Set("X-RouterForge-Action", "stage-rule")
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}
func TestStagingCRUDAndNoFirewallMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "rules.json")
	h := stagedRulesHandler(path)
	if got := callStage(t, h, http.MethodPost, map[string]any{"rule": exampleRule()}, false); got.Code != 403 {
		t.Fatalf("missing gate: %d", got.Code)
	}
	if got := callStage(t, h, http.MethodPost, map[string]any{"rule": exampleRule()}, true); got.Code != 200 {
		t.Fatalf("create: %d %s", got.Code, got.Body.String())
	}
	file, err := readManagedRules(path)
	if err != nil || len(file.Rules) != 1 || file.Rules[0].Enabled {
		t.Fatalf("persist: %+v %v", file, err)
	}
	get := callStage(t, h, http.MethodGet, nil, false)
	if get.Code != 200 || !bytes.Contains(get.Body.Bytes(), []byte(`"firewall_mutation":false`)) {
		t.Fatalf("read contract: %s", get.Body.String())
	}
	if got := callStage(t, h, http.MethodDelete, map[string]string{"id": "test-access"}, true); got.Code != 200 {
		t.Fatalf("delete %d", got.Code)
	}
	file, err = readManagedRules(path)
	if err != nil || len(file.Rules) != 0 {
		t.Fatalf("delete persist: %+v %v", file, err)
	}
}
func TestStagingRejectsActivationAndBadInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	h := stagedRulesHandler(path)
	cases := []managedKnockRule{exampleRule(), exampleRule(), exampleRule(), exampleRule()}
	cases[0].Enabled = true
	cases[1].WAN = "bad interface"
	cases[2].Sequence[2] = 41001
	cases[3].Engine = "fwknopd"
	for _, rule := range cases {
		if got := callStage(t, h, http.MethodPost, map[string]any{"rule": rule}, true); got.Code != 400 {
			t.Fatalf("unsafe rule accepted %+v: %s", rule, got.Body.String())
		}
	}
}
