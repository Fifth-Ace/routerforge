package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

//go:embed web/*
var webFS embed.FS

func startWeb(store *Store, listen string, version string) error {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	fileServer := http.FileServer(http.FS(sub))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "." || p == "" {
			p = "index.html"
		}
		if st, statErr := fs.Stat(sub, p); statErr == nil && !st.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		index, readErr := fs.ReadFile(sub, "index.html")
		if readErr != nil {
			http.Error(w, "frontend unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	}))
	mux.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		data := store.Snapshot(200, 30, 80)
		data["version"] = version
		data["server_time"] = time.Now()
		_ = json.NewEncoder(w).Encode(data)
	})
	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		minutes := 60
		if raw := r.URL.Query().Get("minutes"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil {
				minutes = n
			}
		}
		coverage := store.HistoryCoverage(minutes)
		step := 1
		if minutes > 180 {
			step = 15
		} else if minutes > 60 {
			step = 3
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"minutes": minutes, "coverage": coverage, "sufficient": coverage >= 0.5, "step_minutes": step, "points": store.History(minutes)})
	})
	mux.HandleFunc("/api/quality", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		minutes := 5
		if raw := r.URL.Query().Get("minutes"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil {
				minutes = n
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"minutes": minutes, "upstreams": store.Quality(minutes)})
	})
	mux.HandleFunc("/api/fallbacks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		minutes := 60
		if raw := r.URL.Query().Get("minutes"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil {
				minutes = n
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"minutes": minutes, "edges": store.FallbackEdges(minutes)})
	})
	mux.HandleFunc("/api/error-bursts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		minutes := 60
		if raw := r.URL.Query().Get("minutes"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil {
				minutes = n
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"minutes": minutes, "bursts": store.ErrorBursts(minutes)})
	})
	mux.HandleFunc("/api/clients", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"clients": store.Clients(200)})
	})
	mux.HandleFunc("/api/client", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		ip := strings.TrimSpace(r.URL.Query().Get("ip"))
		limit := 500
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 2000 {
				limit = n
			}
		}
		client, events, ok := store.ClientDetail(ip, limit)
		if !ok {
			http.Error(w, `{"error":"client not found"}`, http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"client": client, "events": events})
	})
	mux.HandleFunc("/api/interfaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"interfaces": store.Interfaces()})
	})
	mux.HandleFunc("/api/system", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(readSystemInfo())
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	s := &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return s.ListenAndServe()
}
