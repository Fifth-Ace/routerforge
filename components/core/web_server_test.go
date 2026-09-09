package main

import (
	"net/http"
	"testing"
	"time"
)

func TestNewCoreHTTPServerHardening(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := newCoreHTTPServer("127.0.0.1:0", handler)

	if server.Addr != "127.0.0.1:0" {
		t.Fatalf("Addr=%q", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("Handler must be preserved")
	}
	if got, want := server.ReadHeaderTimeout, 5*time.Second; got != want {
		t.Fatalf("ReadHeaderTimeout=%v, want %v", got, want)
	}
	if got, want := server.ReadTimeout, 15*time.Second; got != want {
		t.Fatalf("ReadTimeout=%v, want %v", got, want)
	}
	if got, want := server.IdleTimeout, 60*time.Second; got != want {
		t.Fatalf("IdleTimeout=%v, want %v", got, want)
	}
	if got, want := server.MaxHeaderBytes, 32<<10; got != want {
		t.Fatalf("MaxHeaderBytes=%d, want %d", got, want)
	}
	if server.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout=%v, want 0 for SSE", server.WriteTimeout)
	}
}

func TestCoreHTTPServerTimeoutOrdering(t *testing.T) {
	server := newCoreHTTPServer("127.0.0.1:0", http.NotFoundHandler())

	if server.ReadHeaderTimeout <= 0 {
		t.Fatal("ReadHeaderTimeout must be positive")
	}
	if server.ReadTimeout <= server.ReadHeaderTimeout {
		t.Fatalf(
			"ReadTimeout=%v must exceed ReadHeaderTimeout=%v",
			server.ReadTimeout,
			server.ReadHeaderTimeout,
		)
	}
}
