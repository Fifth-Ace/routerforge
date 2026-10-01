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
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error":        "GET or HEAD required",
				"mutation_api": false,
			})
			return
		}
		next(w, r)
	}
}

func serveAntiscanManager(cfg runtimeConfig) error {
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
			"module":         "antiscan-manager",
			"version":        version,
			"api_version":    1,
			"mode":           "read-only-intelligence",
			"mutation_api":   false,
			"upstream":       antiscanUpstreamURL,
			"uptime_seconds": time.Since(started).Seconds(),
		})
	}))
	mux.HandleFunc("/v1/status", getOnly(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, buildAntiscanSnapshot(r.Context(), cfg))
	}))
	mux.HandleFunc("/v1/inspect", getOnly(func(w http.ResponseWriter, r *http.Request) {
		ip := strings.TrimSpace(r.URL.Query().Get("ip"))
		result, status := inspectAntiscanIP(r.Context(), cfg, ip)
		writeJSON(w, status, result)
	}))
	registerUIRoutes(mux, cfg.UIPath)

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       12 * time.Second,
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
		http.Redirect(w, r, "/v1/ui/", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/v1/ui/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, "/v1/ui/")
		clean := filepath.Clean(rel)
		if clean == "." {
			clean = "index.html"
		}
		if clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		clone := r.Clone(r.Context())
		clone.URL.Path = "/" + clean
		uiFS.ServeHTTP(w, clone)
	})
}
