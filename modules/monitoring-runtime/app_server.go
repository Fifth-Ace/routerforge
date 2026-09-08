package main

import (
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func serveMonitoringApp(socket, uiPath string) error {
	if err := os.MkdirAll(filepath.Dir(socket), 0755); err != nil {
		return err
	}
	_ = os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(socket)
	_ = os.Chmod(socket, 0600)

	started := time.Now()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "module": "monitoring", "version": version, "api_version": 1,
			"mode": "read-only", "mutation_api": false,
			"logical_modules": monitoringModuleIDs, "uptime_seconds": time.Since(started).Seconds(),
		})
	}))

	uiFS := http.FileServer(http.Dir(uiPath))
	mux.HandleFunc("/v1/ui", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/v1/ui/index.html", http.StatusTemporaryRedirect)
	})
	mux.Handle("/v1/ui/", http.StripPrefix("/v1/ui/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		uiFS.ServeHTTP(w, r)
	})))

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 3 * time.Second}
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
