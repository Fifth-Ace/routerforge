package main

import (
	"encoding/json"
	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestConsumerHTTP(t *testing.T) {
	s := contract.NewStore(filepath.Join(t.TempDir(), "catalog.json"))
	m := http.NewServeMux()
	registerFoundationConsumers(m, s)
	for _, c := range []struct {
		method, query string
		status        int
	}{{"GET", "?consumer=dns", 200}, {"POST", "?consumer=dns", 405}, {"GET", "?consumer=invalid", 400}, {"GET", "?consumer=dns&consumer=network", 400}, {"GET", "?consumer=dns&id=test&etag=old", 409}, {"GET", "?consumer=dns&url=anything", 400}} {
		w := httptest.NewRecorder()
		m.ServeHTTP(w, httptest.NewRequest(c.method, foundationPath+"/consumers"+c.query, nil))
		if w.Code != c.status {
			t.Fatalf("%v -> %d: %s", c, w.Code, w.Body.String())
		}
		if c.status == 200 {
			var v map[string]any
			if json.Unmarshal(w.Body.Bytes(), &v) != nil || v["etag"] == nil || len(v["groups"].([]any)) != 0 {
				t.Fatal(w.Body.String())
			}
		}
	}
}

func TestConsumerLocalMemory(t *testing.T) {
	m, s := foundationMemPairs("MemTotal: 100 kB\nMemAvailable: 25 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n")
	if m == nil || m.Total != 102400 || m.Used != 76800 || s == nil || s.Total != 0 {
		t.Fatal(m, s)
	}
	m, s = foundationMemPairs("MemTotal: 100 kB\nSwapTotal: 2 kB\nSwapFree: 3 kB\n")
	if m != nil || s != nil {
		t.Fatal(m, s)
	}
	if foundationUptime("0.00 0") != uint64(0) || foundationUptime("NaN") != nil || foundationUptime("") != nil {
		t.Fatal("uptime")
	}
}

func TestConsumerLocalMetricsHTTP(t *testing.T) {
	mux := http.NewServeMux()
	registerFoundationMetrics(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", foundationPath+"/local-metrics", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var p struct {
		Metrics       json.RawMessage `json:"metrics"`
		ObserverScope string          `json:"observerScope"`
		DeviceID      *string         `json:"deviceId"`
		ConnectionID  *string         `json:"connectionId"`
	}
	if json.Unmarshal(w.Body.Bytes(), &p) != nil || p.ObserverScope != "process" || p.DeviceID != nil || p.ConnectionID != nil || contract.ValidateMetrics(p.Metrics) != nil {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", foundationPath+"/local-metrics", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}
