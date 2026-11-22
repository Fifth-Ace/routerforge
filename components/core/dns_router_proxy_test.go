package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDNSRouterCoreModuleProxyContract(t *testing.T) {
	sockets := moduleSockets["dns-router"]
	if len(sockets) != 1 || sockets[0] != "/opt/var/run/routerforge-dns-router.sock" {
		t.Fatalf("missing DNS Router socket mapping: %v", sockets)
	}
	if modulePackageNames["dns-router"] != "routerforge-dns-router" {
		t.Fatal("package mismatch")
	}
	if !moduleMutationAPI("dns-router") || moduleMutationBodyLimit("dns-router") < 4<<20 {
		t.Fatal("mutation API contract")
	}
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut} {
		if !moduleMethodAllowed("dns-router", method) {
			t.Errorf("method denied: %s", method)
		}
	}
	if moduleMethodAllowed("dns-router", http.MethodDelete) || moduleMethodAllowed("dns-router", http.MethodPatch) {
		t.Fatal("unsafe method allowed")
	}
	for _, m := range []string{http.MethodPut, http.MethodPost} {
		r := httptest.NewRequest(m, "/api/modules/dns-router/config", strings.NewReader("{}"))
		if !dnsRouterMutationRequest(r) {
			t.Fatalf("mutation bypass: %s", m)
		}
	}
	if dnsRouterMutationRequest(httptest.NewRequest(http.MethodGet, "/api/modules/dns-router/status", nil)) {
		t.Fatal("GET incorrectly guarded")
	}
	if dnsRouterMutationRequest(httptest.NewRequest(http.MethodPut, "/api/modules/dns/evil", nil)) {
		t.Fatal("cross-module guard")
	}
	path, ok := moduleTargetPath("ui/index.html")
	if !ok || path != "/v1/ui/index.html" {
		t.Fatalf("UI routing: %q %t", path, ok)
	}
}
