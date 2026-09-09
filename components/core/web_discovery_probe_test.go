package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func rfListenerForTestURL(t *testing.T, raw string) rfWebListener {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	_, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscanf(portText, "%d", &port); err != nil {
		t.Fatal(err)
	}
	return rfWebListener{
		Address: "127.0.0.1",
		Port:    port,
		Owners: []rfWebListenerOwner{{
			PID:        123,
			Process:    "demo-panel",
			Executable: "/opt/bin/demo-panel",
			Package:    "demo-panel",
		}},
	}
}

func TestRFDetectWebSurfaceHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html><head><title>Demo Panel</title></head><body>ok</body></html>"))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	surface, ok := rfDetectWebSurface(ctx, rfListenerForTestURL(t, server.URL))
	if !ok {
		t.Fatal("expected web surface")
	}
	if surface.Scheme != "http" || surface.StatusCode != http.StatusOK || surface.Title != "Demo Panel" {
		t.Fatalf("surface=%#v", surface)
	}
}

func TestRFDetectWebSurfaceRejectsJSONAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if surface, ok := rfDetectWebSurface(ctx, rfListenerForTestURL(t, server.URL)); ok {
		t.Fatalf("unexpected web surface: %#v", surface)
	}
}

func TestRFDetectWebSurfaceAcceptsSameEndpointRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><title>Login</title></html>"))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	surface, ok := rfDetectWebSurface(ctx, rfListenerForTestURL(t, server.URL))
	if !ok {
		t.Fatal("expected redirect web surface")
	}
	if surface.StatusCode != http.StatusFound || surface.Path != "/login" || surface.Redirect == "" {
		t.Fatalf("surface=%#v", surface)
	}
}

func TestRFDetectWebSurfaceRejectsGenericForbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html><head><title>403 Forbidden</title></head><body>Forbidden</body></html>"))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if surface, ok := rfDetectWebSurface(ctx, rfListenerForTestURL(t, server.URL)); ok {
		t.Fatalf("generic forbidden page must not become a Web UI: %#v", surface)
	}
}

func TestRFDetectWebSurfaceAcceptsForbiddenLoginForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<html><head><title>Admin Login</title></head><body><form><input name="user"><input type="password"></form></body></html>`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	surface, ok := rfDetectWebSurface(ctx, rfListenerForTestURL(t, server.URL))
	if !ok {
		t.Fatal("forbidden login form should remain a Web UI candidate")
	}
	if surface.StatusCode != http.StatusForbidden || surface.Title != "Admin Login" {
		t.Fatalf("surface=%#v", surface)
	}
}

func TestRFDetectWebSurfaceHTTPSWithSelfSignedCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>TLS Panel</title></head><body></body></html>"))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	surface, ok := rfDetectWebSurface(ctx, rfListenerForTestURL(t, server.URL))
	if !ok {
		t.Fatal("expected TLS web surface")
	}
	if surface.Scheme != "https" || !surface.TLSVerificationSkipped || surface.Title != "TLS Panel" {
		t.Fatalf("surface=%#v", surface)
	}
}

func TestRFShouldActiveProbeWebListenerBlocksKnownNonWebPortsBeforeDial(t *testing.T) {
	oldProbe := rfProbeWebListenerFunc
	t.Cleanup(func() { rfProbeWebListenerFunc = oldProbe })

	calls := 0
	rfProbeWebListenerFunc = func(context.Context, rfWebListener, string, string) rfWebProbeObservation {
		calls++
		return rfWebProbeObservation{LikelyUI: true}
	}

	listeners := []rfWebListener{
		{Address: "127.0.0.1", Port: 23, Owners: []rfWebListenerOwner{{Process: "telnetd", Executable: "/usr/sbin/telnetd"}}},
		{Address: "127.0.0.1", Port: 139, Owners: []rfWebListenerOwner{{Process: "smbd", Executable: "/usr/sbin/smbd"}}},
		{Address: "127.0.0.1", Port: 445, Owners: []rfWebListenerOwner{{Process: "tsmb", Executable: "/usr/sbin/tsmb"}}},
	}
	if surfaces := rfDetectLocalWebSurfaces(context.Background(), listeners); len(surfaces) != 0 {
		t.Fatalf("non-Web ports must not be surfaced: %#v", surfaces)
	}
	if calls != 0 {
		t.Fatalf("probe called %d times for denied ports; want zero", calls)
	}
}

func TestRFShouldActiveProbeWebListenerPreservesKnownRouterWebPorts(t *testing.T) {
	for _, port := range []int{2222, 2233} {
		listener := rfWebListener{Address: "127.0.0.1", Port: port}
		if !rfShouldActiveProbeWebListener(listener) {
			t.Fatalf("known RouterForge-adjacent Web port %d must remain probeable", port)
		}
	}
}

func TestRFShouldActiveProbeWebListenerAllowsOptOwnedHighPort(t *testing.T) {
	listener := rfWebListener{
		Address: "127.0.0.1",
		Port:    32000,
		Owners: []rfWebListenerOwner{{
			Process:    "custom-panel",
			Executable: "/opt/bin/custom-panel",
			Package:    "custom-panel",
		}},
	}
	if !rfShouldActiveProbeWebListener(listener) {
		t.Fatal("/opt package high-port Web UI should be probeable")
	}
}

func TestRFShouldActiveProbeWebListenerRejectsUnknownHighPort(t *testing.T) {
	listener := rfWebListener{Address: "127.0.0.1", Port: 32001}
	if rfShouldActiveProbeWebListener(listener) {
		t.Fatal("unknown high port without owner metadata must not be actively probed")
	}
}

func TestRFWebDiscoveryProbeHostWildcards(t *testing.T) {
	cases := map[string]string{
		"0.0.0.0":   "127.0.0.1",
		"::":        "::1",
		"127.0.0.1": "127.0.0.1",
	}
	for input, want := range cases {
		got, ok := rfWebDiscoveryProbeHost(input)
		if !ok || got != want {
			t.Fatalf("host %q => %q %v; want %q", input, got, ok, want)
		}
	}
}

func TestRFSafeWebDiscoveryRedirectRejectsCrossOrigin(t *testing.T) {
	base, _ := url.Parse("http://127.0.0.1:8080/")
	if got := rfSafeWebDiscoveryRedirect(base, "http://192.168.1.1:8080/login"); got != "" {
		t.Fatalf("cross-origin redirect accepted: %q", got)
	}
}
