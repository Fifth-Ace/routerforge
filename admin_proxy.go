package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const adminModuleSocket = "/opt/var/run/dns-monitor-admin.sock"

func proxyAdminAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":        "admin module is read-only",
			"mutation_api": false,
		})
		return
	}

	suffix := strings.TrimPrefix(r.URL.Path, "/api/admin")
	if suffix == "" || suffix == "/" {
		suffix = "/summary"
	}
	targetPath := "/v1" + suffix

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", adminModuleSocket)
		},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   6 * time.Second,
	}

	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://unix"+targetPath, nil)
	if err != nil {
		adminUnavailable(w, err.Error())
		return
	}
	request.URL.RawQuery = r.URL.RawQuery
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		adminUnavailable(w, err.Error())
		return
	}
	defer response.Body.Close()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func adminUnavailable(w http.ResponseWriter, detail string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"installed":    false,
		"running":      false,
		"mutation_api": false,
		"error":        "dns-monitor-admin is not available",
		"detail":       detail,
	})
}
