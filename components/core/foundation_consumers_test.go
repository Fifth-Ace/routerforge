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
	previous := foundationObserverID
	var err error
	foundationObserverID, err = loadFoundationObserver(filepath.Join(t.TempDir(), "observer.id"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { foundationObserverID = previous })
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
	if json.Unmarshal(w.Body.Bytes(), &p) != nil || p.ObserverScope != "installation" || p.DeviceID != nil || p.ConnectionID != nil || contract.ValidateMetrics(p.Metrics) != nil {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", foundationPath+"/local-metrics", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}

// The UI must revalidate a saved catalogue revision before inserting a target.
// Exercise the real store/API boundary, not just an empty catalogue or pure projection.
func TestConsumerSavedGroupHTTP(t *testing.T) {
	s := contract.NewStore(filepath.Join(t.TempDir(), "catalog.json"))
	mux := http.NewServeMux()
	registerFoundationConsumers(mux, s)
	initial, err := s.Get()
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"schemaVersion":1,"devices":[],"connections":[],"checkPolicies":[],"policyObjects":[{"id":"browser-fixture","name":"Fixture catalogue","type":"domain-group","revision":1,"scope":"local","members":[{"kind":"domain","value":"catalog-target.example"}]}]}`)
	p, err := s.Preview(initial.Current.ETag, body)
	if err != nil {
		t.Fatal(err)
	}
	saved, changed, err := s.Commit(initial.Current.ETag, p.Candidate.ETag, body)
	if err != nil || !changed {
		t.Fatalf("save: %v %v", changed, err)
	}
	get := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", foundationPath+"/consumers?"+query, nil))
		return w
	}
	w := get("consumer=network")
	var listing struct {
		ETag   string                   `json:"etag"`
		Groups []contract.ConsumerGroup `json:"groups"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &listing) != nil || listing.ETag != saved.Current.ETag || len(listing.Groups) != 1 || listing.Groups[0].ID != "browser-fixture" {
		t.Fatal(w.Code, w.Body.String())
	}
	query := "consumer=network&id=browser-fixture&etag=" + listing.ETag
	w = get(query)
	var preview contract.ConsumerPreview
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &preview) != nil || preview.ETag != listing.ETag || len(preview.Values) != 1 || preview.Values[0] != "catalog-target.example" || len(preview.Blocked) != 0 {
		t.Fatal(w.Code, w.Body.String())
	}
	next := []byte(`{"schemaVersion":1,"devices":[],"connections":[],"checkPolicies":[],"policyObjects":[]}`)
	p, err = s.Preview(saved.Current.ETag, next)
	if err != nil {
		t.Fatal(err)
	}
	if _, changed, err = s.Commit(saved.Current.ETag, p.Candidate.ETag, next); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if w = get(query); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
}
