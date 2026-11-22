package dnsrouter

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// P24.4: the live Runtime handler must deliver the assets actually shipped by
// the Svelte 5 package. Stale legacy JS/CSS must stay inaccessible even if an
// old upgrade leaves those files on disk. Test both public UI route prefixes.
func TestP244AdvancedSettingsHTTPAssets(t *testing.T) {
	const bundleName = "svelte-mode.js"
	html, err := os.ReadFile(filepath.Join("frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `src="`+bundleName+`"`) || !strings.Contains(string(html), `href="module.css"`) {
		t.Fatal("Runtime HTML does not load the packaged Svelte JS/CSS")
	}
	for _, old := range []string{`src="app.js"`, `src="p25-settings.js"`, `href="p25-settings.css"`} {
		if strings.Contains(string(html), old) {
			t.Fatalf("legacy UI reference remains in Runtime entrypoint: %s", old)
		}
	}

	dir := t.TempDir()
	shipped := map[string]string{
		"index.html":     string(html),
		"svelte-mode.js": "/* Svelte production-bundle fixture */",
		"module.css":     "/* Runtime stylesheet fixture */",
	}
	for name, body := range shipped {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Deliberately create legacy assets: the strict HTTP allowlist must still
	// deny them. This prevents accidental revival of old executable handlers.
	for _, name := range []string{"app.js", "p22-visual.js", "p22-visual.css", "p25-settings.js", "p25-settings.css"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("UNSAFE_LEGACY_ASSET:"+name), 0600); err != nil {
			t.Fatal(err)
		}
	}

	h := NewAPI(nil, dir).Handler()
	mime := map[string]string{
		"index.html":     "text/html; charset=utf-8",
		"svelte-mode.js": "text/javascript; charset=utf-8",
		"module.css":     "text/css; charset=utf-8",
	}
	for _, prefix := range []string{"/v1/ui/", "/ui/"} {
		for name, contentType := range mime {
			t.Run(prefix+name, func(t *testing.T) {
				path := prefix + name
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				if w.Code != http.StatusOK {
					t.Fatalf("UI GET %s -> %d, expected 200", path, w.Code)
				}
				if got := w.Header().Get("Content-Type"); got != contentType {
					t.Fatalf("MIME mismatch for %s: %q != %q", path, got, contentType)
				}
				if got := w.Header().Get("Cache-Control"); got != "no-store" {
					t.Fatalf("UI cache-control changed: %s", got)
				}
				if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
					t.Fatalf("UI nosniff missing: %s", got)
				}
				if got := w.Body.String(); got != shipped[name] {
					t.Fatalf("unexpected response body for %s", path)
				}
				for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
					w = httptest.NewRecorder()
					h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
					if w.Code != http.StatusMethodNotAllowed {
						t.Fatalf("UI %s %s -> %d, expected 405", method, path, w.Code)
					}
				}
			})
		}
		for _, name := range []string{"app.js", "p22-visual.js", "p22-visual.css", "p25-settings.js", "p25-settings.css", "unknown.js", "unknown.css"} {
			t.Run(prefix+name+"/denied", func(t *testing.T) {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, prefix+name, nil))
				if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "UNSAFE_LEGACY_ASSET:") {
					t.Fatalf("legacy/unlisted UI asset exposed: %s -> HTTP %d", prefix+name, w.Code)
				}
			})
		}
	}
	// Deny traversal even if ServeMux chooses a redirect rather than a 404.
	for _, forbidden := range []string{"/ui/../private", "/v1/ui/../../private", "/ui/other/index.html"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, forbidden, nil))
		if w.Code == http.StatusOK || strings.Contains(w.Body.String(), "UNSAFE_LEGACY_ASSET:") {
			t.Fatalf("unsafe UI path served: %q / HTTP %d", forbidden, w.Code)
		}
	}
	// A missing *allowed* file is not fabricated into a 200 response.
	if err := os.Remove(filepath.Join(dir, bundleName)); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/ui/"+bundleName, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing Svelte bundle unexpectedly served: HTTP %d", w.Code)
	}
}

// DEV Lite ships two additional CSS files.  Their presence alone must not
// make them routable from the experimental Runtime UI (tested above); the
// installed Svelte index must opt in by an exact local stylesheet link.
func TestP244DevLiteReferencedStylesOnly(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"index.html":       `<!doctype html><link rel="stylesheet" href="module.css"><link rel="stylesheet" href="p22-visual.css"><link rel="stylesheet" href="p25-settings.css"><script defer src="svelte-mode.js"></script>`,
		"svelte-mode.js":   "/* compiled Svelte */",
		"module.css":       ".base { display:block }",
		"p22-visual.css":   ".visual { display:block }",
		"p25-settings.css": ".settings { display:block }",
		"p22-visual.js":    "FORBIDDEN_JS",
		"p25-settings.js":  "FORBIDDEN_JS",
		"app.js":           "FORBIDDEN_JS",
		"unknown.css":      "FORBIDDEN_CSS",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	h := NewAPI(nil, dir).Handler()
	for _, prefix := range []string{"/v1/ui/", "/ui/"} {
		for _, name := range []string{"index.html", "svelte-mode.js", "module.css", "p22-visual.css", "p25-settings.css"} {
			t.Run(prefix+name, func(t *testing.T) {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, prefix+name, nil))
				if w.Code != http.StatusOK {
					t.Fatalf("GET %s: HTTP %d", prefix+name, w.Code)
				}
				if name == "p22-visual.css" || name == "p25-settings.css" {
					if got := w.Header().Get("Content-Type"); got != "text/css; charset=utf-8" {
						t.Fatalf("wrong CSS MIME %q", got)
					}
				}
				if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("missing UI safety headers: %s", prefix+name)
				}
				for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
					w = httptest.NewRecorder()
					h.ServeHTTP(w, httptest.NewRequest(method, prefix+name, nil))
					if w.Code != http.StatusMethodNotAllowed {
						t.Fatalf("%s %s: HTTP %d", method, prefix+name, w.Code)
					}
				}
			})
		}
		for _, name := range []string{"app.js", "p22-visual.js", "p25-settings.js", "unknown.css"} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, prefix+name, nil))
			if w.Code != http.StatusNotFound {
				t.Fatalf("legacy or unexpected asset exposed: %s%s HTTP %d", prefix, name, w.Code)
			}
		}
	}
	// If a stylesheet is no longer linked in the current installed entry,
	// it must no longer be exposed even if its old file remains on disk.
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<!doctype html><link rel="stylesheet" href="module.css">`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"p22-visual.css", "p25-settings.css"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/ui/"+name, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("unreferenced stylesheet still exposed: %s HTTP %d", name, w.Code)
		}
	}
}
