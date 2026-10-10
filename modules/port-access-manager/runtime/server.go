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

func jsonReply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func routes(ui string) http.Handler {
	mux := http.NewServeMux()
	readOnly := func(f http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				jsonReply(w, 405, map[string]string{"error": "read-only module"})
				return
			}
			f(w, r)
		}
	}
	mux.HandleFunc("/v1/health", readOnly(func(w http.ResponseWriter, r *http.Request) {
		jsonReply(w, 200, map[string]any{"ok": true, "module": "port-access-manager", "version": version, "mode": "read-only-discovery", "mutation_api": false})
	}))
	mux.HandleFunc("/v1/status", readOnly(func(w http.ResponseWriter, r *http.Request) { jsonReply(w, 200, snapshot()) }))
	mux.HandleFunc("/v1/preflight", readOnly(func(w http.ResponseWriter, r *http.Request) { jsonReply(w, 200, preflightSnapshot()) }))
	mux.HandleFunc("/v1/ui", readOnly(func(w http.ResponseWriter, r *http.Request) { uiFile(w, r, ui) }))
	mux.HandleFunc("/v1/ui/", readOnly(func(w http.ResponseWriter, r *http.Request) { uiFile(w, r, ui) }))
	return mux
}

func uiFile(w http.ResponseWriter, r *http.Request, ui string) {
	rel := strings.TrimPrefix(r.URL.Path, "/v1/ui")
	rel = strings.TrimPrefix(rel, "/")
	clean := filepath.Clean(rel)
	if clean == "." {
		clean = "index.html"
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(ui, clean)
	info, err := os.Stat(full)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, filepath.Base(clean), info.ModTime(), file)
}

func serve(socket, ui string) error {
	if err := os.MkdirAll(filepath.Dir(socket), 0755); err != nil {
		return err
	}
	if _, err := os.Lstat(socket); err == nil {
		return errors.New("socket already exists; refusing to remove it")
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(socket)
	if err := os.Chmod(socket, 0600); err != nil {
		return err
	}
	srv := &http.Server{Handler: routes(ui), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	err = srv.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
