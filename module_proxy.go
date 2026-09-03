package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path"
	"strings"
	"time"
)

var moduleSockets = map[string]string{
	"system":  "/opt/var/run/dns-monitor-system.sock",
	"thermal": "/opt/var/run/dns-monitor-thermal.sock",
	"storage": "/opt/var/run/dns-monitor-storage.sock",
	"network": "/opt/var/run/dns-monitor-network.sock",
}

func proxyModuleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeModuleJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error":        "modules are read-only",
			"mutation_api": false,
		})
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/modules/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	moduleID := strings.ToLower(parts[0])
	if moduleID == "profiling" {
		if len(parts) == 1 || parts[1] == "" || parts[1] == "status" {
			writeModuleJSON(w, http.StatusOK, profilingStatusSnapshot())
			return
		}
		http.NotFound(w, r)
		return
	}

	socket, ok := moduleSockets[moduleID]
	if !ok {
		http.NotFound(w, r)
		return
	}

	target := "/v1/health"
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		if strings.Contains(parts[1], "..") {
			http.NotFound(w, r)
			return
		}
		clean := path.Clean("/" + parts[1])
		target = "/v1" + clean
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socket)
		},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{Transport: transport, Timeout: 6 * time.Second}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://unix"+target, nil)
	if err != nil {
		moduleUnavailable(w, moduleID, err.Error())
		return
	}
	request.URL.RawQuery = r.URL.RawQuery
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		moduleUnavailable(w, moduleID, err.Error())
		return
	}
	defer response.Body.Close()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func moduleUnavailable(w http.ResponseWriter, moduleID, detail string) {
	writeModuleJSON(w, http.StatusServiceUnavailable, map[string]any{
		"module":       moduleID,
		"installed":    false,
		"running":      false,
		"mutation_api": false,
		"error":        "module is not available",
		"detail":       detail,
	})
}

func writeModuleJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
