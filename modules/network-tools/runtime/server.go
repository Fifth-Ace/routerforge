package main

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error": "GET or HEAD required", "mutation_api": false,
			})
			return
		}
		next(w, r)
	}
}

func serveNetworkTools(cfg runtimeConfig) error {
	if err := os.MkdirAll(filepath.Dir(cfg.Socket), 0755); err != nil {
		return err
	}
	_ = os.Remove(cfg.Socket)
	listener, err := net.Listen("unix", cfg.Socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(cfg.Socket)
	if err := os.Chmod(cfg.Socket, 0600); err != nil {
		return err
	}

	started := time.Now()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":             true,
			"module":         "network-tools",
			"version":        version,
			"api_version":    1,
			"mode":           "read-only-diagnostics",
			"mutation_api":   false,
			"uptime_seconds": time.Since(started).Seconds(),
		})
	}))
	registerNetworkToolsRoutes(mux)
	registerUIRoutes(mux, cfg.UIPath)

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       16 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func registerUIRoutes(mux *http.ServeMux, uiPath string) {
	uiFS := http.FileServer(http.Dir(uiPath))
	mux.HandleFunc("/v1/ui", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/v1/ui/index.html", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/v1/ui/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, "/v1/ui/")
		clean := filepath.Clean(rel)
		if clean == "." || clean == "index.html" {
			data, err := os.ReadFile(filepath.Join(uiPath, "index.html"))
			if err != nil {
				http.Error(w, "Network Tools UI unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(data)
			return
		}
		if clean == ".." || strings.HasPrefix(clean, "../") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		clone := r.Clone(r.Context())
		clone.URL.Path = "/" + clean
		uiFS.ServeHTTP(w, clone)
	})
}
