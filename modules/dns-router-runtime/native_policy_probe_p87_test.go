package dnsrouter

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestP87PolicyNamesRoutesAndIsolation(t *testing.T) {
	raw := []byte(`policy, name = Policy0, description = 123:
          mark: ffffaaa
        table4: 4096
        route4:
          route:
          destination: 192.168.1.0/24
        table6: 4096
        route6:

policy, name = Policy1, description = nfqws:
          mark: ffffaab
        table4: 4098
        route4:
          route:
          destination: 0.0.0.0/0
          route:
          destination: 192.168.10.0/24
        table6: 4098
        route6:
`)
	p, err := parseNDMCPoliciesP87(raw)
	if err != nil || len(p) != 2 {
		t.Fatalf("policy parser failed: %+v, %v", p, err)
	}
	if p[0].Name != "Policy0" || p[0].Description != "123" || p[0].Table4 != 4096 || p[0].IPv4RouteStatus != "default_absent" {
		t.Fatalf("Policy0 misclassified: %+v", p[0])
	}
	if p[1].Name != "Policy1" || p[1].Description != "nfqws" || p[1].Table4 != 4098 || p[1].IPv4RouteStatus != "default_present" {
		t.Fatalf("Policy1 misclassified: %+v", p[1])
	}
	catalog := ObserveNativePoliciesP87(context.Background(), func(context.Context) ([]byte, error) { return raw, nil })
	if !catalog.Available || !catalog.ReadOnly || !catalog.NDMCReadAttempted || catalog.DNSProbePerformed || catalog.EgressVerified || catalog.EncryptionProven || catalog.ChangesApplied {
		t.Fatalf("passive report overstated capability: %+v", catalog)
	}
}

func TestP87PolicyReaderFailuresAreFailClosed(t *testing.T) {
	for _, data := range [][]byte{
		nil, []byte{0xff}, []byte(strings.Repeat("X", maxNDMCPolicyBytesP87+1)),
		[]byte("policy, name = Policy0, description = a:\npolicy, name = Policy0, description = b:\n"),
	} {
		if _, err := parseNDMCPoliciesP87(data); err == nil {
			t.Fatalf("must reject %d bytes", len(data))
		}
	}
	cat := ObserveNativePoliciesP87(context.Background(), func(context.Context) ([]byte, error) { return nil, errors.New("no ndmc") })
	if cat.Available || len(cat.Policies) != 0 || len(cat.Warnings) == 0 || cat.DNSProbePerformed {
		t.Fatal("failure should remain unknown", cat)
	}
	w := new(cappedWriterP87)
	if _, err := w.Write(make([]byte, maxNDMCPolicyBytesP87+1)); err == nil {
		t.Fatal("unbounded output accepted")
	}
}

func p87FixtureInventory(t *testing.T, addr string) NativeDNSInventoryP82 {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ndnproxymain.conf"), []byte("dns_server = "+addr+" . # 9.9.9.9\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return ReadOnlyNativeDNSInventoryP82(dir)
}

func p87DNSListener(t *testing.T, rcode int) (string, func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		n, peer, e := conn.ReadFrom(buf)
		if e != nil {
			return
		}
		if n < 20 {
			return
		}
		resp := append([]byte(nil), buf[:n]...)
		resp[2], resp[3] = 0x81, byte(0x80|rcode)
		if rcode == 0 {
			binary.BigEndian.PutUint16(resp[6:8], 1)
			resp = append(resp, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 30, 0, 4, 10, 10, 10, 10)
		}
		_, _ = conn.WriteTo(resp, peer)
	}()
	return conn.LocalAddr().String(), func() { _ = conn.Close(); wg.Wait() }
}

func TestP87ActiveProbeOnlyWhenExplicitAndNative(t *testing.T) {
	addr, stop := p87DNSListener(t, 0)
	defer stop()
	inv := p87FixtureInventory(t, addr)
	result := ProbeNativeDNSP87(context.Background(), inv, "System", "9.9.9.9")
	if !result.Attempted || !result.DNSResponded || !result.DNSSuccess || result.RCode != 0 || result.ChangesApplied || result.EgressVerified || result.EncryptionProven {
		t.Fatal(result)
	}
	rejected := ProbeNativeDNSP87(context.Background(), inv, "Policy9999", "9.9.9.9")
	if rejected.Attempted || rejected.DNSSuccess {
		t.Fatal(rejected)
	}
	rejected = ProbeNativeDNSP87(context.Background(), inv, "System", "bad/hint")
	if rejected.Attempted {
		t.Fatal(rejected)
	}
}

func TestP87ProbeSERVFAILIsNotSuccess(t *testing.T) {
	addr, stop := p87DNSListener(t, 2)
	defer stop()
	result := ProbeNativeDNSP87(context.Background(), p87FixtureInventory(t, addr), "System", "9.9.9.9")
	if !result.Attempted || !result.DNSResponded || result.DNSSuccess || result.RCode != 2 {
		t.Fatal(result)
	}
}

func TestP87NewAPIMethodGates(t *testing.T) {
	store := NewConfigStore(filepath.Join(t.TempDir(), "config.json"))
	h := NewAPI(store, "").Handler()
	for _, route := range []string{"/v1/keenetic/native-policies", "/v1/keenetic/native-dns/probe"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, route, nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s: %d", route, w.Code)
		}
	}
	// No implicit live probing on a read of the passive catalog.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/keenetic/native-dns/probe?instance=%2Fetc&hint=9.9.9.9", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid selector accepted: %d", w.Code)
	}
}
