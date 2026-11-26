package lite

import (
	"encoding/json"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestE09PolicyAuditReadOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	s := dr.NewConfigStore(p)
	cfg, rev, e := s.Load()
	if e != nil {
		t.Fatal(e)
	}
	cfg.Upstreams = []dr.Upstream{{ID: "one", Transport: "udp", Address: "192.0.2.1:53"}}
	cfg.Rules = []dr.Rule{{ID: "example", Domain: "example.org", Match: "suffix", Upstream: "one"}}
	rev, e = s.Save(cfg, rev)
	if e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	m := &Manager{Store: s}
	for _, tc := range []struct {
		method, path string
		code         int
	}{{"POST", "/v1/service/policy-audit", 405}, {"GET", "/v1/service/policy-audit?domain=a&domain=b", 400}, {"GET", "/v1/service/policy-audit?x=1", 400}, {"GET", "/v1/service/policy-audit?mac=bad", 400}, {"GET", "/v1/service/policy-audit?domain=bad..org", 400}, {"GET", "/v1/service/policy-audit?domain=example.org", 200}} {
		w := httptest.NewRecorder()
		m.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.code {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if tc.code == http.StatusOK {
			var x struct {
				SavedRevision string                  `json:"saved_revision"`
				Active        bool                    `json:"active"`
				Applied       *dr.PolicyAuditE09      `json:"applied"`
				DryRun        bool                    `json:"dry_run"`
				SavedExplain  dr.DiagnosticExplainE09 `json:"saved_explain"`
			}
			if json.Unmarshal(w.Body.Bytes(), &x) != nil || x.SavedRevision != rev || x.Active || x.Applied != nil || !x.DryRun || x.SavedExplain.Decision.RuleID != "example" {
				t.Fatal(w.Body.String())
			}
		}
	}
	after, e := os.ReadFile(p)
	if e != nil || string(before) != string(after) {
		t.Fatal("saved config changed")
	}
}
func TestE09ActiveAndSavedAreDistinct(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	store := dr.NewConfigStore(p)
	cfg, rev, e := store.Load()
	if e != nil {
		t.Fatal(e)
	}
	cfg.Upstreams = []dr.Upstream{{ID: "one", Transport: "udp", Address: "192.0.2.1:53"}, {ID: "two", Transport: "udp", Address: "192.0.2.2:53"}}
	cfg.Rules = []dr.Rule{{ID: "old", Domain: "example.org", Match: "suffix", Upstream: "one"}}
	oldRev, e := store.Save(cfg, rev)
	if e != nil {
		t.Fatal(e)
	}
	compiled, e := dr.Compile(cfg)
	if e != nil {
		t.Fatal(e)
	}
	next := cfg
	next.Rules = []dr.Rule{{ID: "new", Domain: "example.org", Match: "suffix", Upstream: "two"}}
	nextRev, e := store.Save(next, oldRev)
	if e != nil {
		t.Fatal(e)
	}
	m := &Manager{Store: store, engine: &Engine{policy: compiled}, applied: cfg, revision: oldRev}
	w := httptest.NewRecorder()
	m.ServeHTTP(w, httptest.NewRequest("GET", "/v1/service/policy-audit?domain=example.org", nil))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var x struct {
		Same           bool                    `json:"same_revision"`
		Saved          string                  `json:"saved_revision"`
		Applied        string                  `json:"applied_revision"`
		SavedExplain   dr.DiagnosticExplainE09 `json:"saved_explain"`
		AppliedExplain dr.DiagnosticExplainE09 `json:"applied_explain"`
	}
	if json.Unmarshal(w.Body.Bytes(), &x) != nil || x.Same || x.Saved != nextRev || x.Applied != oldRev || x.SavedExplain.Decision.RuleID != "new" || x.AppliedExplain.Decision.RuleID != "old" {
		t.Fatal(w.Body.String())
	}
}
