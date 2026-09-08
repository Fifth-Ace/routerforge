package main

import (
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidModuleMode(t *testing.T) {
	for _, id := range []string{"system", "thermal", "storage", "network", "all"} {
		if !validModuleMode(id) {
			t.Fatalf("validModuleMode(%q)=false", id)
		}
	}
	for _, id := range []string{"", "dns", "monitoring", "ALL"} {
		if validModuleMode(id) {
			t.Fatalf("validModuleMode(%q)=true", id)
		}
	}
}

func TestMonitoringModuleIDsStable(t *testing.T) {
	want := []string{"system", "thermal", "storage", "network"}
	if !reflect.DeepEqual(monitoringModuleIDs, want) {
		t.Fatalf("monitoringModuleIDs=%v, want %v", monitoringModuleIDs, want)
	}
}

func TestMonitoringSocketPath(t *testing.T) {
	base := filepath.Join("tmp", "routerforge-run")
	want := map[string]string{
		"system":  "routerforge-system.sock",
		"thermal": "routerforge-thermal.sock",
		"storage": "routerforge-storage.sock",
		"network": "routerforge-network.sock",
	}
	if !reflect.DeepEqual(monitoringSocketNames, want) {
		t.Fatalf("monitoringSocketNames=%v, want %v", monitoringSocketNames, want)
	}
	for _, id := range monitoringModuleIDs {
		wantPath := filepath.Join(base, want[id])
		if got := monitoringSocketPath(base, id); got != wantPath {
			t.Fatalf("monitoringSocketPath(%q,%q)=%q, want %q", base, id, got, wantPath)
		}
	}
	if got := monitoringSocketPath(base, "dns"); got != "" {
		t.Fatalf("monitoringSocketPath accepted unknown module: %q", got)
	}
}

func TestUnixSocketRespondingDetectsActiveSocket(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "active.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	active, err := unixSocketResponding(socket)
	if err != nil {
		t.Fatalf("unixSocketResponding: %v", err)
	}
	if !active {
		t.Fatal("active Unix socket was not detected")
	}
}

func TestServeAllModulesRefusesActiveLegacySocket(t *testing.T) {
	dir := t.TempDir()
	socket := monitoringSocketPath(dir, monitoringModuleIDs[0])
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	err = serveAllModules(dir)
	if err == nil {
		t.Fatal("serveAllModules unexpectedly accepted active legacy socket")
	}
	if !strings.Contains(err.Error(), "monitoring socket already active") {
		t.Fatalf("unexpected error: %v", err)
	}
}
