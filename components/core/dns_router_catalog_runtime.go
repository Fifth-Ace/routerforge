package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"
)

var dnsRouterCatalogSocketPath = "/opt/var/run/routerforge-dns-router.sock"

// dnsRouterCatalogEngineActive reports the applied DNS engine state, not the
// existence of the always-on management supervisor. This is read-only.
func dnsRouterCatalogEngineActive() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 550*time.Millisecond)
	defer cancel()
	dialer := &net.Dialer{}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", dnsRouterCatalogSocketPath)
	}, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 550 * time.Millisecond}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://dns-router/v1/service/status", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var result struct {
		Mode   string `json:"mode"`
		Active bool   `json:"active"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 4096))
	if dec.Decode(&result) != nil {
		return false
	}
	return result.Mode == "independent-dns" && result.Active
}
