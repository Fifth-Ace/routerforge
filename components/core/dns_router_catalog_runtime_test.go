package main

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestP195CatalogDNSRouterTrueEngineState(t *testing.T) {
	old := dnsRouterCatalogSocketPath
	dnsRouterCatalogSocketPath = filepath.Join(t.TempDir(), "dns.sock")
	defer func() { dnsRouterCatalogSocketPath = old }()
	ln, err := net.Listen("unix", dnsRouterCatalogSocketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var active atomic.Bool
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/service/status" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"mode":"independent-dns","active":%t}`, active.Load())
	})}
	defer server.Close()
	go server.Serve(ln)
	item := func() catalogItem {
		return catalogItem{ID: "dns-router", Managed: true, PackageAuthoritative: true, Detection: catalogDetection{Packages: []string{"routerforge-dns-router"}, Services: []string{"/opt/etc/init.d/S95routerforge-dns-router"}}}
	}
	installed := map[string]string{"routerforge-dns-router": "1~dev.test"}
	processes := map[string]bool{"routerforge-dns-router": true}
	for _, want := range []bool{false, true, false} {
		active.Store(want)
		entry := item()
		finalizeCatalogItem(&entry, installed, processes, func(string) bool { return true })
		if entry.ServiceRunning != want {
			t.Fatalf("DNS engine expected %t, got %t", want, entry.ServiceRunning)
		}
		if !entry.Installed {
			t.Fatal("DNS Router not detected as installed")
		}
	}
	server.Close()
	ln.Close()
	if dnsRouterCatalogEngineActive() {
		t.Fatal("closed socket reported running")
	}
}
