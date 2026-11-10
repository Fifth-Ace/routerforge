package dnsrouter

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestP223UIAssetsServedWithSafeMIMEAndAllowlist(t *testing.T) {
	folder := t.TempDir()
	files := map[string]string{
		"index.html":     "<!doctype html><title>DNS Router</title>",
		"svelte-mode.js": "console.log('svelte')",
		"module.css":     ".base{color:green}",
		"app.js":         "console.log('legacy')",

		"secret.txt": "must-not-be-served",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	store := NewConfigStore(filepath.Join(t.TempDir(), "config.json"))
	handler := NewAPI(store, folder).Handler()
	mimes := map[string]string{
		"index.html": "text/html", "svelte-mode.js": "text/javascript", "module.css": "text/css",
	}
	for _, prefix := range []string{"/v1/ui/", "/ui/"} {
		for name, mime := range mimes {
			t.Run(prefix+name, func(t *testing.T) {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, prefix+name, nil))
				if response.Code != http.StatusOK || response.Body.String() != files[name] {
					t.Fatalf("UI GET %s: status=%d body=%q", prefix+name, response.Code, response.Body.String())
				}
				if ct := response.Header().Get("Content-Type"); !strings.HasPrefix(ct, mime) {
					t.Fatalf("wrong MIME %s: %q", name, ct)
				}
				if response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("UI must use nosniff and no-store: %v", response.Header())
				}
			})
		}
		for _, bad := range []string{"secret.txt", "app.js", "p22-visual.js", "svelte-mode.js.map", "another.js"} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, prefix+bad, nil))
			if response.Code != http.StatusNotFound {
				t.Fatalf("unsafe path %s status %d", bad, response.Code)
			}
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, prefix+"svelte-mode.js", nil))
		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("write denied: %d", response.Code)
		}
	}
}
