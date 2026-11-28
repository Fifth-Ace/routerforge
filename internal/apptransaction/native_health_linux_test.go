//go:build linux

package apptransaction

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeHealthPayload(t *testing.T) {
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`{"ok":true,"module":"dns","version":"v1","data_ready":true,"phase":"ready"}`, true},
		{`{"ok":true,"module":"dns","version":"v1","data_ready":false,"phase":"ready"}`, false},
		{`{"ok":true,"module":"dns","version":"v2","data_ready":true,"phase":"ready"}`, false},
		{`{"ok":true,"ok":true,"module":"dns","version":"v1","data_ready":true,"phase":"ready"}`, false},
		{`{"ok":true,"module":"dns","version":"v1","data_ready":true,"phase":"ready"} {}`, false},
		{`{"ok":true,"module":"admin","version":"v1","data_ready":true,"phase":"ready"}`, false},
	} {
		if err := validateNativeHealth([]byte(tc.body), "dns", "v1"); (err == nil) != tc.ok {
			t.Fatalf("%s: %v", tc.body, err)
		}
	}
	for _, n := range []string{"routerforge-core", "routerforge-profiling", "routerforge-dns-router", "../admin"} {
		if _, err := ProbeRunningModuleHealth(context.Background(), n, "v1"); err == nil {
			t.Fatal("unsupported accepted", n)
		}
	}
	if len(nativeHealthModules) != 8 {
		t.Fatal("unexpected module scope")
	}
}

func TestNativeHealthUnixPeer(t *testing.T) {
	d := t.TempDir()
	socket := filepath.Join(d, "health.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err = os.Chmod(socket, 0600); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/health" {
			t.Error("unexpected request")
		}
		fmt.Fprint(w, `{"ok":true,"module":"admin","version":"v1"}`)
	})}
	defer srv.Close()
	go srv.Serve(l)
	uid := uint32(os.Geteuid())
	out, err := probeNativeHealth(context.Background(), socket, exe, uid, "admin", "v1")
	if err != nil || out.PeerPID != os.Getpid() {
		t.Fatal(out, err)
	}
	if _, err = probeNativeHealth(context.Background(), socket, exe, uid, "admin", "v2"); err == nil {
		t.Fatal("old runtime accepted")
	}
	other := filepath.Join(d, "other")
	if err = os.WriteFile(other, []byte("not executable"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = probeNativeHealth(context.Background(), socket, other, uid, "admin", "v1"); err == nil {
		t.Fatal("wrong peer executable accepted")
	}
	alias := filepath.Join(d, "alias")
	if err = os.Symlink(socket, alias); err != nil {
		t.Fatal(err)
	}
	if _, err = probeNativeHealth(context.Background(), alias, exe, uid, "admin", "v1"); err == nil {
		t.Fatal("symlink accepted")
	}
	if err = os.Chmod(socket, 0666); err != nil {
		t.Fatal(err)
	}
	if _, err = probeNativeHealth(context.Background(), socket, exe, uid, "admin", "v1"); err == nil {
		t.Fatal("public socket accepted")
	}
}
