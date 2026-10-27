package dnsrouter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxAPIBytes int64 = MaxConfigBytes

type API struct {
	Store *ConfigStore
	UI    string
}

func NewAPI(store *ConfigStore, ui string) *API { return &API{Store: store, UI: ui} }

func respondJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, err error) {
	respondJSON(w, status, map[string]any{"error": err.Error()})
}
func requireMutation(w http.ResponseWriter, r *http.Request, action string) bool {
	if r.Header.Get("X-RouterForge-Action") != action {
		fail(w, http.StatusForbidden, errors.New("X-RouterForge-Action header required"))
		return false
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		fail(w, http.StatusUnsupportedMediaType, errors.New("application/json required"))
		return false
	}
	return true
}
func decodeBody(r *http.Request, dest any, limit int64) error {
	if r.ContentLength > limit {
		return errors.New("payload too large")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, limit+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON objects are not permitted")
		}
		return err
	}
	return nil
}
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			return
		}
		cfg, rev, err := a.Store.Load()
		if err != nil {
			fail(w, 500, err)
			return
		}
		respondJSON(w, 200, map[string]any{"component": "dns-router", "status": "inactive", "dns_listener": false, "dns_interception": false, "installed_dns_manager_required": false, "config_revision": rev, "rules": len(cfg.Rules), "upstreams": len(cfg.Upstreams), "time": time.Now().UTC().Format(time.RFC3339)})
	})
	mux.HandleFunc("/v1/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cfg, rev, err := a.Store.Load()
			if err != nil {
				fail(w, 500, err)
				return
			}
			w.Header().Set("ETag", fmt.Sprintf("%q", rev))
			respondJSON(w, 200, map[string]any{"config": cfg, "revision": rev, "active": false})
		case http.MethodPut:
			if !requireMutation(w, r, "save") {
				return
			}
			var cfg Config
			if err := decodeBody(r, &cfg, maxAPIBytes); err != nil {
				fail(w, 400, err)
				return
			}
			expected := strings.Trim(r.Header.Get("If-Match"), "\"")
			if expected == "" {
				fail(w, 428, errors.New("If-Match required"))
				return
			}
			rev, err := a.Store.Save(cfg, expected)
			if errors.Is(err, ErrStale) {
				fail(w, 409, err)
				return
			}
			if err != nil {
				fail(w, 400, err)
				return
			}
			w.Header().Set("ETag", fmt.Sprintf("%q", rev))
			respondJSON(w, 200, map[string]any{"revision": rev, "active": false, "saved": true})
		default:
			w.Header().Set("Allow", "GET, PUT")
			w.WriteHeader(405)
		}
	})
	mux.HandleFunc("/v1/evaluate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(405)
			return
		}
		if !requireMutation(w, r, "evaluate") {
			return
		}
		var req struct {
			Domain string `json:"domain"`
		}
		if err := decodeBody(r, &req, 4096); err != nil {
			fail(w, 400, err)
			return
		}
		cfg, _, err := a.Store.Load()
		if err != nil {
			fail(w, 500, err)
			return
		}
		e, err := Compile(cfg)
		if err != nil {
			fail(w, 500, err)
			return
		}
		result, err := e.Evaluate(req.Domain)
		if err != nil {
			fail(w, 400, err)
			return
		}
		respondJSON(w, 200, map[string]any{"decision": result, "dry_run": true})
	})
	mux.HandleFunc("/v1/import-preview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(405)
			return
		}
		if !requireMutation(w, r, "import-preview") {
			return
		}
		var req ImportRequest
		if err := decodeBody(r, &req, 2<<20); err != nil {
			fail(w, 400, err)
			return
		}
		cfg, rev, err := a.Store.Load()
		if err != nil {
			fail(w, 500, err)
			return
		}
		p, err := PreviewImport(cfg, req)
		if err != nil {
			fail(w, 400, err)
			return
		}
		respondJSON(w, 200, map[string]any{"preview": p, "base_revision": rev, "saved": false})
	})
	mux.HandleFunc("/v1/export", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			return
		}
		cfg, _, err := a.Store.Load()
		if err != nil {
			fail(w, 500, err)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=routerforge-dns-router.json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(ExportContent(cfg))
	})
	mux.HandleFunc("/ui/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/ui/")
		if name == "" {
			name = "index.html"
		}
		if name != "index.html" && name != "app.js" && name != "module.css" {
			http.NotFound(w, r)
			return
		}
		if a.UI == "" {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(filepath.Join(a.UI, name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		switch name {
		case "index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case "app.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case "module.css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		}
		_, _ = io.Copy(w, f)
	})
	return mux
}
