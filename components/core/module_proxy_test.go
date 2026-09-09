package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestModuleTargetPathPreservesTrailingSlash(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "health", in: "", want: "/v1/health", ok: true},
		{name: "api leaf", in: "info", want: "/v1/info", ok: true},
		{name: "ui index", in: "ui/index.html", want: "/v1/ui/index.html", ok: true},
		{name: "ui directory", in: "ui/", want: "/v1/ui/", ok: true},
		{name: "nested asset directory", in: "ui/assets/", want: "/v1/ui/assets/", ok: true},
		{name: "reject traversal", in: "ui/../secret", want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := moduleTargetPath(tt.in)
			if ok != tt.ok {
				t.Fatalf("ok=%v, want %v (path=%q)", ok, tt.ok, got)
			}
			if got != tt.want {
				t.Fatalf("path=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestModuleTargetPathUIRedirectRegression(t *testing.T) {
	got, ok := moduleTargetPath("ui/")
	if !ok {
		t.Fatal("ui directory rejected")
	}
	if got == "/v1/ui" {
		t.Fatal("trailing slash was lost; this reintroduces the nested Core shell/404 redirect bug")
	}
}

func configureModuleProxyTest(t *testing.T, socket string, installed map[string]string) {
	t.Helper()

	oldSockets := moduleSockets
	oldInstalledPackages := moduleInstalledPackages
	moduleSockets = map[string][]string{
		"dns": {socket},
	}
	moduleInstalledPackages = func() map[string]string {
		return installed
	}

	t.Cleanup(func() {
		moduleSockets = oldSockets
		moduleInstalledPackages = oldInstalledPackages
	})
}

func TestModuleUnavailableReportsInstalledDuringRestart(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "missing.sock")
	configureModuleProxyTest(t, socket, map[string]string{
		"routerforge-dns": "0.4.18-beta",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/modules/dns/health", nil)
	proxyModuleAPI(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content-type=%q, want application/json", got)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["installed"] != true {
		t.Fatalf("installed=%v, want true", payload["installed"])
	}
	if payload["running"] != false {
		t.Fatalf("running=%v, want false", payload["running"])
	}
}

func TestModuleUnavailableReportsAbsentModule(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "missing.sock")
	configureModuleProxyTest(t, socket, map[string]string{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/modules/dns/health", nil)
	proxyModuleAPI(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["installed"] != false {
		t.Fatalf("installed=%v, want false", payload["installed"])
	}
	if payload["running"] != false {
		t.Fatalf("running=%v, want false", payload["running"])
	}
}

func TestModuleUIUnavailableReturnsReconnectHTML(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "missing.sock")
	configureModuleProxyTest(t, socket, map[string]string{
		"routerforge-dns": "0.4.18-beta",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/modules/dns/ui/index.html", nil)
	proxyModuleAPI(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content-type=%q, want text/html", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "location.reload()") {
		t.Fatal("reconnect page does not reload after module health recovers")
	}
	if !strings.Contains(body, "/health") {
		t.Fatal("reconnect page does not poll module health")
	}
	if strings.Contains(body, `"error":"RouterForge module is not available"`) {
		t.Fatal("raw module JSON leaked into iframe reconnect response")
	}
}

func TestModuleUIProxyStillServesLiveUnixSocket(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "module.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ui/index.html" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><title>module-ok</title>"))
	})}
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = server.Close()
	})

	configureModuleProxyTest(t, socket, map[string]string{
		"routerforge-dns": "0.4.18-beta",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/modules/dns/ui/index.html", nil)
	proxyModuleAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "module-ok") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestModuleMutationBodyLimitsMatchDownstreamContracts(t *testing.T) {
	if got, want := moduleMutationBodyLimit("dns"), int64(64<<10); got != want {
		t.Fatalf("dns limit=%d, want %d", got, want)
	}
	if got, want := moduleMutationBodyLimit("admin"), int64(8<<10); got != want {
		t.Fatalf("admin limit=%d, want %d", got, want)
	}
	if got := moduleMutationBodyLimit("monitoring"); got != 0 {
		t.Fatalf("monitoring limit=%d, want 0", got)
	}
}

func TestModuleMutationBodyLimitRejectsKnownLength(t *testing.T) {
	body := strings.Repeat("x", int(dnsModuleMutationBodyLimit)+1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/modules/dns/resolvers", strings.NewReader(body))

	proxyModuleAPI(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want %d body=%s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
	if got := rec.Header().Get("Connection"); !strings.EqualFold(got, "close") {
		t.Fatalf("Connection=%q, want close", got)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := int64(payload["max_body_bytes"].(float64)); got != dnsModuleMutationBodyLimit {
		t.Fatalf("max_body_bytes=%d, want %d", got, dnsModuleMutationBodyLimit)
	}
}

func TestModuleMutationBodyLimitRejectsUnknownLength(t *testing.T) {
	body := strings.Repeat("x", int(dnsModuleMutationBodyLimit)+1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/modules/dns/resolvers", strings.NewReader(body))
	req.ContentLength = -1
	req.TransferEncoding = []string{"chunked"}

	proxyModuleAPI(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want %d body=%s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
}

func TestAdminMutationBodyLimitRejectsAboveDownstreamLimit(t *testing.T) {
	body := strings.Repeat("x", int(adminModuleMutationBodyLimit)+1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/modules/admin/processes/123/signal", strings.NewReader(body))

	bounded, ok := boundedModuleMutationRequest(rec, req, "admin")
	if ok || bounded != nil {
		t.Fatal("oversized Admin mutation request was accepted")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want %d body=%s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
}

func TestModuleMutationBodyLimitAcceptsDNSBoundaryAndForwardsBody(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "dns.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	type observation struct {
		method string
		path   string
		body   string
		err    error
	}
	observed := make(chan observation, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, readErr := io.ReadAll(r.Body)
		observed <- observation{method: r.Method, path: r.URL.Path, body: string(body), err: readErr}
		w.WriteHeader(http.StatusNoContent)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	configureModuleProxyTest(t, socket, map[string]string{"routerforge-dns": "test"})
	payload := strings.Repeat("x", int(dnsModuleMutationBodyLimit))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/modules/dns/resolvers", strings.NewReader(payload))

	proxyModuleAPI(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d, want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	select {
	case got := <-observed:
		if got.err != nil {
			t.Fatalf("upstream read: %v", got.err)
		}
		if got.method != http.MethodPost || got.path != "/v1/resolvers" {
			t.Fatalf("upstream method/path=%s %s", got.method, got.path)
		}
		if got.body != payload {
			t.Fatalf("upstream body length=%d, want %d", len(got.body), len(payload))
		}
	case <-time.After(time.Second):
		t.Fatal("upstream body observation timed out")
	}
}

func TestModuleMutationBodyLimitLeavesGETUnchanged(t *testing.T) {
	body := strings.Repeat("x", int(dnsModuleMutationBodyLimit)+1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/modules/dns/info", strings.NewReader(body))

	bounded, ok := boundedModuleMutationRequest(rec, req, "dns")
	if !ok {
		t.Fatalf("GET was rejected: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if bounded != req {
		t.Fatal("GET request should pass through without body buffering")
	}
}
