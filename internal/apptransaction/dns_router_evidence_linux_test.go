//go:build linux

package apptransaction

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func dnsEvidenceFixture(on, gate bool) string {
	rev := ""
	consent := ""
	if on {
		rev = "sha256:" + strings.Repeat("a", 64)
	}
	if gate {
		consent = "explicit-enable-v1"
	}
	return fmt.Sprintf(`{"mode":"independent-dns","active":%t,"maintenance":false,"interception":%t,"last_error":"","applied_revision":%q,"desired":{"version":1,"service_enabled":%t,"interception_enabled":%t,"applied_revision":%q,"gate_consent":%q},"connection":{"state":"disconnected"}}`, on, gate, rev, on, gate, rev, consent)
}
func TestDNSRouterEvidenceValidation(t *testing.T) {
	on := dnsEvidenceFixture(true, true)
	off := dnsEvidenceFixture(false, false)
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{on, true}, {off, true}, {dnsEvidenceFixture(true, false), true},
		{strings.Replace(on, `"active":true`, `"active":false`, 1), false},
		{strings.Replace(on, `"maintenance":false`, `"maintenance":true`, 1), false},
		{strings.Replace(on, `"last_error":""`, `"last_error":"failed"`, 1), false},
		{strings.Replace(on, `"state":"disconnected"`, `"state":"connected"`, 1), false},
		{strings.Replace(on, `"state":"disconnected"`, `"state":"recovery_required"`, 1), false},
		{strings.Replace(on, `"service_enabled":true`, `"service_enabled":null`, 1), false},
		{strings.Replace(off, `"active":false,`, ``, 1), false},
		{strings.Replace(off, `"active":false`, `"active":false,"active":false`, 1), false},
		{strings.Replace(on, `"version":1`, `"version":1,"version":1`, 1), false},
		{strings.Replace(on, `"gate_consent":"explicit-enable-v1"`, `"gate_consent":""`, 1), false},
		{strings.Replace(on, strings.Repeat("a", 64), strings.Repeat("b", 64), 1), false},
		{on + ` {}`, false},
	} {
		_, err := decodeDNSRouterEvidence([]byte(tc.body))
		if (err == nil) != tc.valid {
			t.Fatalf("valid=%t err=%v body=%s", tc.valid, err, tc.body)
		}
	}
	for _, state := range []string{"STARTING", "DEGRADED", "READY"} {
		err := validateDNSRouterStartup([]byte(fmt.Sprintf(`{"component":"dns-router","version":"v1","state":%q}`, state)), "v1")
		if (err == nil) != (state == "READY") {
			t.Fatal(state, err)
		}
	}
}
func TestDNSRouterEvidenceNativePeer(t *testing.T) {
	d := t.TempDir()
	sock := filepath.Join(d, "dns.sock")
	l, e := net.Listen("unix", sock)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if e = os.Chmod(sock, 0600); e != nil {
		t.Fatal(e)
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	var drift atomic.Bool
	var count atomic.Int32
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("mutation attempted")
		}
		switch r.URL.Path {
		case "/v1/startup":
			fmt.Fprint(w, `{"component":"dns-router","version":"v1","state":"READY"}`)
		case "/v1/service/status":
			n := count.Add(1)
			fmt.Fprint(w, dnsEvidenceFixture(!(drift.Load() && n%2 == 0), false))
		default:
			t.Error("unknown endpoint")
			w.WriteHeader(404)
		}
	})}
	defer srv.Close()
	go srv.Serve(l)
	uid := uint32(os.Geteuid())
	out, e := probeDNSRouterEvidence(context.Background(), sock, exe, uid, "v1")
	if e != nil || !out.ServiceEnabled || out.PeerPID != os.Getpid() {
		t.Fatal(out, e)
	}
	if _, e = probeDNSRouterEvidence(context.Background(), sock, exe, uid, "v2"); e == nil {
		t.Fatal("stale runtime accepted")
	}
	drift.Store(true)
	if _, e = probeDNSRouterEvidence(context.Background(), sock, exe, uid, "v1"); e == nil {
		t.Fatal("intent drift accepted")
	}
	after := out
	after.RuntimeVersion = "v2"
	if e = CompareDNSRouterIntent(out, after, "v2"); e != nil {
		t.Fatal(e)
	}
	after.ServiceEnabled = false
	if e = CompareDNSRouterIntent(out, after, "v2"); e == nil {
		t.Fatal("changed intent accepted")
	}
	if _, e = probeNativeEndpoint(context.Background(), sock, exe, uid, "/v1/service/control", func([]byte) error { return nil }); e == nil {
		t.Fatal("unapproved endpoint accepted")
	}
}
