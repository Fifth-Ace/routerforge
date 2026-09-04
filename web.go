package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

func snapshotData(store *Store, version string) map[string]any {
	data := store.Snapshot(200, 30, 80)
	data["version"] = version
	data["server_time"] = time.Now()
	return data
}

func startWeb(store *Store, listen string, version string) error {
	sub, err := frontendFS()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	auth := newAuthManager()
	auth.registerHandlers(mux)
	fileServer := http.FileServer(http.FS(sub))

	serveIndex := func(w http.ResponseWriter) {
		index, readErr := fs.ReadFile(sub, "index.html")
		if readErr != nil {
			http.Error(w, "frontend unavailable: build frontend first", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	}

	mux.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(snapshotData(store, version))
	})

	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		interval := 2 * time.Second
		if raw := r.URL.Query().Get("interval_ms"); raw != "" {
			if ms, parseErr := strconv.Atoi(raw); parseErr == nil {
				if ms < 1000 {
					ms = 1000
				}
				if ms > 30000 {
					ms = 30000
				}
				interval = time.Duration(ms) * time.Millisecond
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache, no-transform")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		fmt.Fprint(w, "retry: 1500\n\n")
		flusher.Flush()

		send := func() bool {
			// An SSE connection may have been opened while auth was disabled.
			// Re-check before every snapshot so enabling auth closes old anonymous streams.
			if auth.authRequired() {
				if _, authenticated := auth.sessionUser(r); !authenticated {
					return false
				}
			}
			payload, marshalErr := json.Marshal(snapshotData(store, version))
			if marshalErr != nil {
				return false
			}
			if _, writeErr := fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload); writeErr != nil {
				return false
			}
			flusher.Flush()
			return true
		}
		if !send() {
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				if !send() {
					return
				}
			}
		}
	})

	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		minutes := 60
		if raw := r.URL.Query().Get("minutes"); raw != "" {
			if n, parseErr := strconv.Atoi(raw); parseErr == nil {
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"minutes": minutes, "coverage": coverage, "sufficient": coverage >= 0.5,
			"step_minutes": step, "points": store.History(minutes),
		})
	})

	mux.HandleFunc("/api/quality", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		minutes := 5
		if raw := r.URL.Query().Get("minutes"); raw != "" {
			if n, parseErr := strconv.Atoi(raw); parseErr == nil {
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
			if n, parseErr := strconv.Atoi(raw); parseErr == nil {
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
			if n, parseErr := strconv.Atoi(raw); parseErr == nil {
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
			if n, parseErr := strconv.Atoi(raw); parseErr == nil && n > 0 && n <= 2000 {
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

	mux.HandleFunc("/api/plain-dns", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, `{"error":"read-only"}`, http.StatusMethodNotAllowed)
			return
		}
		limit := 100
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, parseErr := strconv.Atoi(raw); parseErr == nil && n > 0 && n <= 500 {
				limit = n
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(plainDNS.Snapshot(limit))
	})

	mux.HandleFunc("/api/dns/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "GET required"})
			return
		}
		if !dnsModuleEnabled() {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "RouterForge DNS is not installed"})
			return
		}
		info, infoErr := readDNSInfo()
		if infoErr != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": infoErr.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(info)
	})

	mux.HandleFunc("/api/admin/", proxyAdminAPI)
	mux.HandleFunc("/api/modules/", proxyModuleAPI)

	mux.HandleFunc("/api/catalog/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, `{"error":"POST required"}`, http.StatusMethodNotAllowed)
			return
		}

		releaseDone := make(chan routerForgeReleaseStatus, 1)
		registryDone := make(chan routerForgeRegistryStatus, 1)
		go func() { releaseDone <- forceRefreshRouterForgeReleaseIndex() }()
		go func() { registryDone <- forceRefreshRouterForgeRegistry() }()

		releaseStatus := <-releaseDone
		registryStatus := <-registryDone
		catalog := readCatalog()

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":       releaseStatus.Online && registryStatus.Online,
			"release":  releaseStatus,
			"registry": registryStatus,
			"catalog":  catalog,
		})
	})

	mux.HandleFunc("/api/catalog/action", handleCatalogActionTest)
	mux.HandleFunc("/api/catalog/install", handleCatalogInstallTest)
	mux.HandleFunc("/api/catalog", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, `{"error":"GET required"}`, http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(readCatalog())
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "." || p == "" {
			serveIndex(w)
			return
		}
		if stat, statErr := fs.Stat(sub, p); statErr == nil && !stat.IsDir() {
			if strings.HasPrefix(p, "_app/immutable/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		serveIndex(w)
	}))

	server := &http.Server{
		Addr: listen, Handler: profiledHTTPHandler(auth.middleware(mux)), ReadHeaderTimeout: 5 * time.Second,
	}
	return server.ListenAndServe()
}
