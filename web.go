package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

func coreSnapshot(version string) map[string]any {
	data := map[string]any{
		"version":           version,
		"server_time":       time.Now(),
		"dns_module_online": false,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	raw, err := readModuleRaw(ctx, "dns", "/v1/snapshot", 4*time.Second)
	if err != nil {
		return data
	}
	var module map[string]any
	if json.Unmarshal(raw, &module) != nil {
		return data
	}
	if dnsVersion, ok := module["version"]; ok {
		data["dns_version"] = dnsVersion
	}
	for key, value := range module {
		if key == "version" || key == "server_time" {
			continue
		}
		data[key] = value
	}
	data["version"] = version
	data["server_time"] = time.Now()
	data["dns_module_online"] = true
	return data
}

func startWeb(listen string, version string) error {
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
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, `{"error":"GET required"}`, http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(coreSnapshot(version))
	})

	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
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
			// Preserve the 0.3.x security guarantee: enabling auth closes anonymous
			// SSE streams even though DNS state now lives in another process.
			if auth.authRequired() {
				if _, authenticated := auth.sessionUser(r); !authenticated {
					return false
				}
			}
			payload, marshalErr := json.Marshal(coreSnapshot(version))
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

	legacyDNSProxy := func(target string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			clone := r.Clone(r.Context())
			clone.URL.Path = "/api/modules/dns/" + strings.TrimPrefix(target, "/")
			proxyModuleAPI(w, clone)
		}
	}
	mux.HandleFunc("/api/history", legacyDNSProxy("history"))
	mux.HandleFunc("/api/quality", legacyDNSProxy("quality"))
	mux.HandleFunc("/api/fallbacks", legacyDNSProxy("fallbacks"))
	mux.HandleFunc("/api/error-bursts", legacyDNSProxy("error-bursts"))
	mux.HandleFunc("/api/clients", legacyDNSProxy("clients"))
	mux.HandleFunc("/api/client", legacyDNSProxy("client"))
	mux.HandleFunc("/api/interfaces", legacyDNSProxy("interfaces"))
	mux.HandleFunc("/api/system", legacyDNSProxy("system"))
	mux.HandleFunc("/api/plain-dns", legacyDNSProxy("plain-dns"))
	mux.HandleFunc("/api/dns/info", legacyDNSProxy("info"))

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
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":         true,
			"version":    version,
			"module_abi": "v1",
		})
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
