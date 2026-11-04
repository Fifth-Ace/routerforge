package dnsrouter

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// P24.4: the live DNS management HTTP handler must serve the Advanced
// settings assets. Source-only JS tests cannot detect a 404 from this API.
func TestP244AdvancedSettingsHTTPAssets(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"p25-settings.js", "p25-settings.css"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("asset:"+name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	h := NewAPI(nil, dir).Handler()
	cases := []struct {
		path string
		mime string
	}{
		{"/v1/ui/p25-settings.js", "text/javascript; charset=utf-8"},
		{"/v1/ui/p25-settings.css", "text/css; charset=utf-8"},
		{"/ui/p25-settings.js", "text/javascript; charset=utf-8"},
		{"/ui/p25-settings.css", "text/css; charset=utf-8"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("UI GET %s -> %d, expected 200", tc.path, w.Code)
			}
			if got := w.Header().Get("Content-Type"); got != tc.mime {
				t.Fatalf("MIME mismatch %q != %q", got, tc.mime)
			}
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("UI cache-control changed: %s", got)
			}
			if got := w.Body.String(); got != "asset:"+filepath.Base(tc.path) {
				t.Fatalf("unexpected response body: %q", got)
			}
			w = httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, tc.path, nil))
			if w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("mutation accepted for UI path: %d", w.Code)
			}
		})
	}
	for _, forbidden := range []string{"/ui/unknown.js", "/ui/../private", "/v1/ui/unknown.css"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, forbidden, nil))
		if w.Code == http.StatusOK || strings.Contains(w.Body.String(), "asset:") {
			t.Fatalf("unlisted file served: %q / HTTP %d", forbidden, w.Code)
		}
	}
}
