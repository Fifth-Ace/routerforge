package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateDNSMutationDesiredRejectsInvalidEnvelope(t *testing.T) {
	if err := validateDNSMutationDesired(nil, map[string]bool{"DoT": true}); err == nil {
		t.Fatal("nil desired state must be rejected")
	}
	state := &dnsConfigState{Logical: map[string]*dnsLogicalResolver{}}
	if err := validateDNSMutationDesired(state, nil); err == nil {
		t.Fatal("empty changed set must be rejected")
	}
	if err := validateDNSMutationDesired(state, map[string]bool{"Bogus": true}); err == nil {
		t.Fatal("unknown protocol must be rejected")
	}
	if err := validateDNSMutationDesired(state, map[string]bool{"DNS": true}); err != nil {
		t.Fatalf("valid DNS mutation envelope rejected: %v", err)
	}
}

func TestDNSControlRuntimeProbeCallback(t *testing.T) {
	want := errors.New("probe failed")
	manager := newDNSControlManager(nil, "", func(context.Context) error {
		return want
	})
	if err := manager.runRuntimeProbe(context.Background()); !errors.Is(err, want) {
		t.Fatalf("runtime probe error = %v, want %v", err, want)
	}
	manager = newDNSControlManager(nil, "")
	if err := manager.runRuntimeProbe(context.Background()); err != nil {
		t.Fatalf("nil runtime probe must be a no-op: %v", err)
	}
}

func TestProbeDNSModuleRuntimeHealthContract(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-socket runtime probe is validated on Linux CI")
	}

	dir := t.TempDir()
	socket := filepath.Join(dir, "dns.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	defer os.Remove(socket)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"module":"dns","mutation_api":true}`))
	})
	server := &http.Server{Handler: mux}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()

	if err := probeDNSModuleRuntime(context.Background(), socket); err != nil {
		t.Fatalf("healthy DNS module probe failed: %v", err)
	}
}
