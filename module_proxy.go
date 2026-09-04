package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

var moduleSockets = map[string][]string{
	"dns":     {"/opt/var/run/routerforge-dns.sock"},
	"system":  {"/opt/var/run/routerforge-system.sock", "/opt/var/run/dns-monitor-system.sock"},
	"thermal": {"/opt/var/run/routerforge-thermal.sock", "/opt/var/run/dns-monitor-thermal.sock"},
	"storage": {"/opt/var/run/routerforge-storage.sock", "/opt/var/run/dns-monitor-storage.sock"},
	"network": {"/opt/var/run/routerforge-network.sock", "/opt/var/run/dns-monitor-network.sock"},
}

func activeModuleSocket(moduleID string) (string, bool) {
	candidates, ok := moduleSockets[moduleID]
	if !ok || len(candidates) == 0 {
		return "", false
	}
	for _, socket := range candidates {
		if _, err := os.Stat(socket); err == nil {
			return socket, true
		}
	}
	return candidates[0], true
}

func moduleMethodAllowed(moduleID, method string) bool {
	if method == http.MethodHead {
		return true
	}
	if moduleID == "dns" {
		switch method {
		case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete:
			return true
		default:
			return false
		}
	}
	return method == http.MethodGet
}

func moduleTargetPath(rest string) (string, bool) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "/v1/health", true
	}
	if strings.Contains(rest, "..") {
		return "", false
	}
	clean := path.Clean("/" + rest)
	if clean == "/." || !strings.HasPrefix(clean, "/") {
		return "", false
	}
	return "/v1" + clean, true
}

func moduleTransport(socket string) *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socket)
		},
	}
}

func proxyModuleAPI(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/modules/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		http.NotFound(w, r)
		return
	}

	moduleID := strings.ToLower(strings.TrimSpace(parts[0]))
	if moduleID == "profiling" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeModuleJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error":        "profiling status is read-only",
				"mutation_api": false,
			})
			return
		}
		if len(parts) == 1 || parts[1] == "" || parts[1] == "status" {
			writeModuleJSON(w, http.StatusOK, profilingStatusSnapshot())
			return
		}
		http.NotFound(w, r)
		return
	}

	if !moduleMethodAllowed(moduleID, r.Method) {
		allow := "GET, HEAD"
		if moduleID == "dns" {
			allow = "GET, HEAD, POST, PATCH, DELETE"
		}
		w.Header().Set("Allow", allow)
		writeModuleJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error":        "method is not allowed by this RouterForge module",
			"mutation_api": moduleID == "dns",
		})
		return
	}

	socket, ok := activeModuleSocket(moduleID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	suffix := ""
	if len(parts) == 2 {
		suffix = parts[1]
	}
	targetPath, ok := moduleTargetPath(suffix)
	if !ok {
		http.NotFound(w, r)
		return
	}

	transport := moduleTransport(socket)
	defer transport.CloseIdleConnections()

	upstream := &url.URL{Scheme: "http", Host: "unix"}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Transport = transport
	proxy.FlushInterval = -1
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = targetPath
		req.URL.RawPath = ""
		req.Host = "unix"
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if strings.HasPrefix(targetPath, "/v1/ui/") {
			resp.Header.Set("Cache-Control", "no-cache")
		} else {
			resp.Header.Set("Cache-Control", "no-store")
		}
		return nil
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		moduleUnavailable(rw, moduleID, fmt.Sprintf("%s: %v", socket, err))
	}
	proxy.ServeHTTP(w, r)
}

func readModuleRaw(ctx context.Context, moduleID, targetPath string, timeout time.Duration) ([]byte, error) {
	socket, ok := activeModuleSocket(moduleID)
	if !ok {
		return nil, fmt.Errorf("unknown module %q", moduleID)
	}
	if strings.Contains(targetPath, "..") || !strings.HasPrefix(targetPath, "/v1/") {
		return nil, fmt.Errorf("invalid module target path")
	}
	transport := moduleTransport(socket)
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix"+targetPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("module %s returned http %d", moduleID, resp.StatusCode)
	}
	return body, nil
}

func moduleUnavailable(w http.ResponseWriter, moduleID, detail string) {
	writeModuleJSON(w, http.StatusServiceUnavailable, map[string]any{
		"module":       moduleID,
		"installed":    false,
		"running":      false,
		"mutation_api": moduleID == "dns",
		"error":        "RouterForge module is not available",
		"detail":       detail,
	})
}

func writeModuleJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
