package dnsrouter

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func p85Query(domain string) []byte {
	q := []byte{0x85, 0x31, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	for _, part := range strings.Split(domain, ".") {
		q = append(q, byte(len(part)))
		q = append(q, []byte(part)...)
	}
	return append(q, 0, 0, 1, 0, 1)
}

func p85FakeStubby(t *testing.T, address [4]byte) (string, func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, client, err := conn.ReadFrom(buf)
			if err != nil {
				select {
				case <-stop:
					return
				default:
				}
				continue
			}
			if n < 17 {
				continue
			}
			resp := append([]byte(nil), buf[:n]...)
			resp[2], resp[3] = 0x81, 0x80
			resp[6], resp[7] = 0, 1
			resp = append(resp, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 30, 0, 4)
			resp = append(resp, address[:]...)
			_, _ = conn.WriteTo(resp, client)
		}
	}()
	return conn.LocalAddr().String(), func() { close(stop); _ = conn.Close(); wg.Wait() }
}

func p85Config(t *testing.T, directory, addr string) Config {
	t.Helper()
	content := "dns_server = " + addr + " . # 9.9.9.9\n"
	if err := os.WriteFile(filepath.Join(directory, "ndnproxymain.conf"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return Config{SchemaVersion: 1,
		Upstreams: []Upstream{{ID: "native-p85", Transport: "native", Address: "native://System/./9.9.9.9"}},
		Rules:     []Rule{{ID: "r-p85", Domain: "example.org", Match: "suffix", Upstream: "native-p85"}},
	}
}

func p85ExpectA(t *testing.T, resp []byte, want [4]byte) {
	t.Helper()
	if len(resp) < 16 || resp[3]&15 != 0 || resp[7] != 1 {
		t.Fatalf("expected successful A response, got %x", resp)
	}
	if string(resp[len(resp)-4:]) != string(want[:]) {
		t.Fatalf("unexpected A: %x", resp[len(resp)-4:])
	}
}

func TestP85RealNativeForwardDynamicPortAndFailClosed(t *testing.T) {
	dir := t.TempDir()
	oldAddr, oldStop := p85FakeStubby(t, [4]byte{10, 11, 12, 13})
	cfg := p85Config(t, dir, oldAddr)
	f, err := NewShadowForwarder(cfg, ShadowOptions{Listen: "127.0.0.1:55377", NativeBridge: true, NativeDNSDir: dir, Timeout: 250 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	query := p85Query("example.org")
	p85ExpectA(t, f.resolve(context.Background(), query, "udp"), [4]byte{10, 11, 12, 13})
	oldStop()
	// The native config still points to a dead UDP listener: do not bypass to
	// another policy and never silently use the system default resolver.
	dead := f.resolve(context.Background(), query, "udp")
	if len(dead) < 4 || dead[3]&15 != 2 {
		t.Fatalf("dead endpoint must SERVFAIL: %x", dead)
	}
	newAddr, newStop := p85FakeStubby(t, [4]byte{20, 21, 22, 23})
	defer newStop()
	_ = p85Config(t, dir, newAddr)
	p85ExpectA(t, f.resolve(context.Background(), query, "udp"), [4]byte{20, 21, 22, 23})
	stats := f.Stats()
	if stats.Requests != 3 || stats.Forwarded != 2 || stats.Failures != 1 || stats.Upstreams["native-p85"].Attempts != 3 || stats.Upstreams["native-p85"].Successes != 2 || stats.Upstreams["native-p85"].Failures != 1 {
		t.Fatalf("runtime counters drifted: %+v", stats)
	}
	if err := os.Remove(filepath.Join(dir, "ndnproxymain.conf")); err != nil {
		t.Fatal(err)
	}
	missing := f.resolve(context.Background(), query, "udp")
	if len(missing) < 4 || missing[3]&15 != 2 {
		t.Fatalf("absent inventory must SERVFAIL: %x", missing)
	}
}

func TestP85RejectNativeSelfListenerAndUnauthorizedMode(t *testing.T) {
	dir := t.TempDir()
	cfg := p85Config(t, dir, "127.0.0.1:55379")
	q := p85Query("example.org")
	f, err := NewShadowForwarder(cfg, ShadowOptions{Listen: "127.0.0.1:55379", NativeBridge: true, NativeDNSDir: dir, Timeout: 250 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	blocked := f.resolve(context.Background(), q, "udp")
	if len(blocked) < 4 || blocked[3]&15 != 2 {
		t.Fatalf("self-loop must fail closed: %x", blocked)
	}
	if _, err := NewShadowForwarder(cfg, ShadowOptions{Listen: "127.0.0.1:55379"}); err == nil {
		t.Fatal("native without capability accepted")
	}
}

func TestP85ReadinessDoesNotClaimRuntimeOrEncryption(t *testing.T) {
	dir := t.TempDir()
	cfg := p85Config(t, dir, "127.0.0.1:55379")
	inv := ReadOnlyNativeDNSInventoryP82(dir)
	s := BuildNativeBridgeReadinessP85(cfg, inv, true, true, "127.0.0.1:55379")
	if !s.ReadOnly || !s.CapabilityEnabled || !s.ShadowActive || s.ActivationAllowed || s.TransportEncryptionProven || s.PolicyEgressVerified || s.ChangesApplied || len(s.Bindings) != 1 || s.Bindings[0].Ready {
		t.Fatalf("readiness overstated: %+v", s)
	}
	if s.Bindings[0].Address != "" {
		t.Fatal("self listener exposed in preview")
	}
	s = BuildNativeBridgeReadinessP85(cfg, inv, false, false, "127.0.0.1:55388")
	if s.CapabilityEnabled || s.ShadowActive || !s.Bindings[0].Ready {
		t.Fatalf("unexpected passive preview: %+v", s)
	}
}

func TestP85RealShadowLifecycleManualStopRestart(t *testing.T) {
	dir := t.TempDir()
	addr, stop := p85FakeStubby(t, [4]byte{1, 2, 3, 4})
	defer stop()
	cfg := p85Config(t, dir, addr)
	// Use a real ephemeral port but release reservation before lifecycle start.
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	if port < 1025 {
		t.Skip("non-high port")
	}
	listen := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	store := NewConfigStore(filepath.Join(t.TempDir(), "config.json"))
	_, revision, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	revision, err = store.Save(cfg, revision)
	if err != nil {
		t.Fatal(err)
	}
	manager := NewLifecycleManager(store, ShadowOptions{Listen: listen, NativeBridge: true, NativeDNSDir: dir, Timeout: 500 * time.Millisecond})
	defer manager.Close()
	for i := 0; i < 2; i++ {
		if err := manager.Start(revision); err != nil {
			t.Fatal(err)
		}
		if !manager.Status().Active {
			t.Fatal("shadow not active")
		}
		if err := manager.Stop(); err != nil {
			t.Fatal(err)
		}
		if manager.Status().Active {
			t.Fatal("shadow still active")
		}
	}
	// Explicit stop must relinquish the listener, not leave :port owned.
	tcp, err := net.Listen("tcp4", listen)
	if err != nil {
		t.Fatalf("listener leaked: %v", err)
	}
	_ = tcp.Close()
}

func TestP85RuntimeAvoidsProtectedNamespace(t *testing.T) {
	dir := t.TempDir()
	addr, stop := p85FakeStubby(t, [4]byte{1, 2, 3, 4})
	defer stop()
	cfg := p85Config(t, dir, addr)
	f, err := NewShadowForwarder(cfg, ShadowOptions{Listen: "127.0.0.1:55377", NativeBridge: true, NativeDNSDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	query := p85Query("router.lan")
	resp := f.resolve(context.Background(), query, "udp")
	if len(resp) < 4 || binary.BigEndian.Uint16(resp[2:4])&15 != 5 {
		t.Fatalf("protected domain must REFUSE: %x", resp)
	}
}

func TestP85StatusAPIReadOnlyAndMethodGate(t *testing.T) {
	store := NewConfigStore(filepath.Join(t.TempDir(), "config.json"))
	_, rev, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Save(Config{SchemaVersion: 1, Upstreams: []Upstream{}, Rules: []Rule{}}, rev)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAPI(store, "").Handler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/keenetic/native-bridge/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET status HTTP %d: %s", response.Code, response.Body.String())
	}
	var status NativeBridgeReadinessP85
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.ReadOnly || status.ActivationAllowed || status.CapabilityEnabled || status.ShadowActive || status.ChangesApplied || status.PolicyEgressVerified || status.TransportEncryptionProven {
		t.Fatalf("unsafe API status: %+v", status)
	}
	forbidden := httptest.NewRecorder()
	handler.ServeHTTP(forbidden, httptest.NewRequest(http.MethodPost, "/v1/keenetic/native-bridge/status", nil))
	if forbidden.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST was not blocked: %d", forbidden.Code)
	}
}
