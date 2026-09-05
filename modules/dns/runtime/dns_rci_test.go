package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDNSRCIRejectsApplicationErrorAtHTTP200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"error","message":"address conflict"}`))
	}))
	defer server.Close()
	client := newDNSRCIClient(server.URL)
	if _, err := client.postJSON(context.Background(), "/dns-proxy/tls/upstream", []map[string]any{{"address": "1.1.1.1"}}); err == nil {
		t.Fatal("expected NDMS application error")
	}
}

func TestDNSRCIPostUsesStructuredJSON(t *testing.T) {
	var body any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rci/" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	client := newDNSRCIClient(server.URL + "/rci")
	_, err := client.postJSON(context.Background(), "/dns-proxy/tls/upstream", []map[string]any{{"address": "1.1.1.1", "sni": "cloudflare-dns.com"}})
	if err != nil {
		t.Fatal(err)
	}
	command, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("request body = %#v", body)
	}
	dnsProxy, ok := command["dns-proxy"].(map[string]any)
	if !ok {
		t.Fatalf("missing dns-proxy command: %#v", command)
	}
	tls, ok := dnsProxy["tls"].(map[string]any)
	if !ok {
		t.Fatalf("missing tls command: %#v", dnsProxy)
	}
	if _, ok := tls["upstream"]; !ok {
		t.Fatalf("missing upstream leaf: %#v", tls)
	}
}

func TestDNSRCIRejectsNestedApplicationErrorAtHTTP200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ip":{"name-server":{"status":[{"status":"error","code":"123","message":"no input"}]}}}`))
	}))
	defer server.Close()
	client := newDNSRCIClient(server.URL)
	if _, err := client.postJSON(context.Background(), "/ip/name-server", []map[string]any{{"no": true}}); err == nil {
		t.Fatal("expected nested NDMS application error")
	}
}

func TestDNSRCIDeleteSettingUsesResourceAndQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/rci/ip/name-server" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("address") != "8.8.4.4" || q.Get("port") != "53" || q.Get("domain") != "" || q.Get("interface") != "OpkgTun15" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := newDNSRCIClient(server.URL + "/rci")
	_, err := client.deleteSetting(context.Background(), "/ip/name-server", map[string]any{
		"address":   "8.8.4.4",
		"port":      53,
		"domain":    "",
		"interface": "OpkgTun15",
	})
	if err != nil {
		t.Fatal(err)
	}
}
