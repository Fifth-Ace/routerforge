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
	"sync"
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

func handleCatalogRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, `{"error":"GET required"}`, http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(readCatalog())
}

const (
	catalogRefreshCooldown   = 5 * time.Second
	catalogRefreshModeHeader = "X-RouterForge-Catalog-Refresh"
)

type catalogRefreshResult struct {
	Release  routerForgeReleaseStatus
	Registry routerForgeRegistryStatus
	Catalog  catalogSnapshot
}

type catalogRefreshCall struct {
	done   chan struct{}
	result catalogRefreshResult
}

type catalogRefreshCoordinator struct {
	mu            sync.Mutex
	inFlight      *catalogRefreshCall
	last          catalogRefreshResult
	lastCompleted time.Time
	hasLast       bool
	cooldown      time.Duration
	now           func() time.Time
	run           func() catalogRefreshResult
}

func newCatalogRefreshCoordinator(cooldown time.Duration, run func() catalogRefreshResult) *catalogRefreshCoordinator {
	return &catalogRefreshCoordinator{
		cooldown: cooldown,
		now:      time.Now,
		run:      run,
	}
}

func (c *catalogRefreshCoordinator) do(ctx context.Context) (catalogRefreshResult, string, error) {
	return c.doWithPolicy(ctx, true)
}

func (c *catalogRefreshCoordinator) doFresh(ctx context.Context) (catalogRefreshResult, string, error) {
	return c.doWithPolicy(ctx, false)
}

func (c *catalogRefreshCoordinator) doWithPolicy(ctx context.Context, allowCached bool) (catalogRefreshResult, string, error) {
	now := c.now()

	c.mu.Lock()
	if c.inFlight != nil {
		call := c.inFlight
		c.mu.Unlock()
		select {
		case <-call.done:
			return call.result, "joined", nil
		case <-ctx.Done():
			return catalogRefreshResult{}, "", ctx.Err()
		}
	}

	if allowCached && c.hasLast && c.cooldown > 0 {
		age := now.Sub(c.lastCompleted)
		if age >= 0 && age < c.cooldown {
			result := c.last
			c.mu.Unlock()
			return result, "cached", nil
		}
	}

	call := &catalogRefreshCall{done: make(chan struct{})}
	c.inFlight = call
	c.mu.Unlock()

	go c.execute(call)

	select {
	case <-call.done:
		return call.result, "fresh", nil
	case <-ctx.Done():
		return catalogRefreshResult{}, "", ctx.Err()
	}
}
func (c *catalogRefreshCoordinator) execute(call *catalogRefreshCall) {
	result := c.run()

	c.mu.Lock()
	call.result = result
	c.last = result
	c.lastCompleted = c.now()
	c.hasLast = true
	if c.inFlight == call {
		c.inFlight = nil
	}
	close(call.done)
	c.mu.Unlock()
}

func performCatalogRefresh() catalogRefreshResult {
	releaseDone := make(chan routerForgeReleaseStatus, 1)
	registryDone := make(chan routerForgeRegistryStatus, 1)
	sourcesDone := make(chan struct{}, 1)
	go func() { releaseDone <- forceRefreshRouterForgeReleaseIndex() }()
	go func() { registryDone <- forceRefreshRouterForgeRegistry() }()
	go func() {
		forceRefreshUserAppSources()
		sourcesDone <- struct{}{}
	}()

	releaseStatus := <-releaseDone
	registryStatus := <-registryDone
	<-sourcesDone
	invalidateEntwareCatalog()

	return catalogRefreshResult{
		Release:  releaseStatus,
		Registry: registryStatus,
		Catalog:  refreshCatalog(),
	}
}

var catalogRefreshHTTP = newCatalogRefreshCoordinator(catalogRefreshCooldown, performCatalogRefresh)

func handleCatalogRefreshWithCoordinator(w http.ResponseWriter, r *http.Request, coordinator *catalogRefreshCoordinator) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if !sameOriginRequest(r) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin catalog refresh rejected"})
		return
	}

	freshValue := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("fresh")))
	requireFresh := freshValue == "1" || freshValue == "true"

	var (
		result catalogRefreshResult
		mode   string
		err    error
	)
	if requireFresh {
		result, mode, err = coordinator.doFresh(r.Context())
	} else {
		result, mode, err = coordinator.do(r.Context())
	}
	if err != nil {
		return
	}

	w.Header().Set(catalogRefreshModeHeader, mode)
	writeCatalogJSON(w, http.StatusOK, map[string]any{
		"ok":       result.Release.Online && result.Registry.Online,
		"release":  result.Release,
		"registry": result.Registry,
		"catalog":  result.Catalog,
	})
}

func handleCatalogRefresh(w http.ResponseWriter, r *http.Request) {
	handleCatalogRefreshWithCoordinator(w, r, catalogRefreshHTTP)
}

const (
	coreReadHeaderTimeout = 5 * time.Second
	coreReadTimeout       = 15 * time.Second
	coreIdleTimeout       = 60 * time.Second
	coreMaxHeaderBytes    = 32 << 10
)

func newCoreHTTPServer(listen string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              listen,
		Handler:           handler,
		ReadHeaderTimeout: coreReadHeaderTimeout,
		ReadTimeout:       coreReadTimeout,
		IdleTimeout:       coreIdleTimeout,
		MaxHeaderBytes:    coreMaxHeaderBytes,
		// Keep WriteTimeout disabled: Core exposes long-lived SSE streams.
		WriteTimeout: 0,
	}
}

func startWeb(listen string, version string) error {
	sub, err := frontendFS()
	if err != nil {
		return err
	}

	// Build one bounded runtime snapshot at Core startup. GET /api/catalog only
	// serializes this cache; active discovery is owned by explicit refresh paths.
	refreshCatalog()

	mux := http.NewServeMux()
	auth := newAuthManager()
	auth.registerHandlers(mux)
	registerAppCenterHandlers(mux)
	registerAppSourceHandlers(mux)
	registerAppActionHandlers(mux)
	registerPlatformHandlers(mux)
	registerCatalogWebProbeHandler(mux)
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
	mux.HandleFunc("/api/modules/", securedModuleProxy(auth))

	mux.HandleFunc("/api/catalog/channel", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, status := routerForgeReleaseSnapshot()
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(status)
		case http.MethodPost:
			if !sameOriginRequest(r) {
				writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin release-channel request rejected"})
				return
			}
			var request struct {
				Channel string `json:"channel"`
			}
			if err := decodeSmallJSON(w, r, &request); err != nil {
				return
			}
			if err := setReleaseChannel(request.Channel); err != nil {
				writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
				return
			}
			status := forceRefreshRouterForgeReleaseIndex()
			writeCatalogJSON(w, http.StatusOK, map[string]any{
				"ok":      status.Supported,
				"release": status,
				"catalog": refreshCatalog(),
			})
		default:
			w.Header().Set("Allow", "GET, POST")
			writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET or POST required"})
		}
	})

	mux.HandleFunc("/api/catalog/refresh", handleCatalogRefresh)

	mux.HandleFunc("/api/catalog/action", handleCatalogActionTest)
	mux.HandleFunc("/api/catalog/install", handleCatalogInstallTest)
	mux.HandleFunc("/api/catalog", handleCatalogRead)

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

	server := newCoreHTTPServer(
		listen,
		profiledHTTPHandler(auth.middleware(mux)),
	)
	return server.ListenAndServe()
}
