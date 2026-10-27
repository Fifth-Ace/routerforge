package dnsrouter

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestDPR2DUpstreamDiagnosticsPassive(t *testing.T) {
	st := NewConfigStore(filepath.Join(t.TempDir(), "cfg.json"))
	_, rev, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := shadowConfig("udp")
	_, err = st.Save(cfg, rev)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAPI(st, "").Handler()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/v1/upstreams/status", nil))
	if w.Code != 200 {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	var doc struct {
		IDs       []string `json:"upstream_ids"`
		Passive   bool     `json:"passive_only"`
		Probe     bool     `json:"probe_network"`
		Intercept bool     `json:"dns_interception"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.IDs) != 1 || doc.IDs[0] != "pool" || !doc.Passive || doc.Probe || doc.Intercept {
		t.Fatalf("not passive: %+v", doc)
	}
}
