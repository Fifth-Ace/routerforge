package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOnlyAPI(t *testing.T) {
	h := routes(t.TempDir())
	for _, path := range []string{"/v1/health", "/v1/status"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 200 {
			t.Fatalf("GET %s: %d", path, w.Code)
		}
		if !strings.Contains(w.Body.String(), "read-only-discovery") {
			t.Fatal("missing discovery mode")
		}
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		if w.Code != 405 {
			t.Fatalf("POST %s: %d", path, w.Code)
		}
	}
}
func TestUITraversalAndHead(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "index.html"), []byte("PASS"), 0600); err != nil {
		t.Fatal(err)
	}
	h := routes(d)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/v1/ui/index.html", nil))
	if w.Code != 200 || w.Body.String() != "PASS" {
		t.Fatalf("ui: %d %q", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/v1/ui/", nil))
	if w.Code != 405 {
		t.Fatalf("write accepted: %d", w.Code)
	}
}
func TestEngineSchema(t *testing.T) {
	s := snapshot()
	if len(s.Engines) != 3 || s.Engines[0].ID != "knockd" || s.Engines[1].ID != "fwknopd" || s.Engines[2].ID != "iptables-recent" || s.MutationAPI {
		t.Fatalf("unexpected state: %+v", s)
	}
}

func TestKernelMatchPresent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matches")
	if kernelMatchPresent(path, "recent") {
		t.Fatal("missing file must be unsupported")
	}
	if err := os.WriteFile(path, []byte("set\nconntrack\nrecent\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !kernelMatchPresent(path, "recent") || kernelMatchPresent(path, "rec") {
		t.Fatal("exact kernel match detection failed")
	}
}
