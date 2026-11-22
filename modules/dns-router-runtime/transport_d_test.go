package dnsrouter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func dOptions(dial DialContextFunc, ca *x509.CertPool) ShadowOptions {
	return ShadowOptions{Listen: "127.0.0.1:55353", Timeout: time.Second, Workers: 8, Dial: dial, RootCAs: ca}
}
func dRoots(cert *x509.Certificate) *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(cert)
	return p
}
func TestDPR2DValidationAndChain(t *testing.T) {
	oversized := Config{SchemaVersion: 1, Upstreams: make([]Upstream, MaxUpstreams+1)}
	if _, err := Compile(oversized); err == nil {
		t.Fatal("unbounded upstream count accepted")
	}
	cfg := shadowConfig("udp")
	cfg.Upstreams = append(cfg.Upstreams, Upstream{ID: "reserve", Transport: "tcp", Address: "192.0.2.45:5300"})
	cfg.Upstreams[0].Fallbacks = []string{"reserve"}
	e, err := Compile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := e.UpstreamChain("pool")
	if err != nil || len(chain) != 2 || chain[1].ID != "reserve" {
		t.Fatalf("unexpected chain %v %v", chain, err)
	}
	cfg.Upstreams[1].Fallbacks = []string{"pool"}
	if _, err := Compile(cfg); err == nil {
		t.Fatal("recursive fallback accepted")
	}
	cfg.Upstreams[1].Fallbacks = nil
	cfg.Upstreams[0].Fallbacks = []string{"missing"}
	if _, err := Compile(cfg); err == nil {
		t.Fatal("missing fallback accepted")
	}
	cfg.Upstreams[0].Fallbacks = nil
	for _, bad := range []Upstream{
		{ID: "pool", Transport: "dot", Address: "192.0.2.44:853"},
		{ID: "pool", Transport: "doh", Address: "192.0.2.44:443", ServerName: "Example.COM"},
		{ID: "pool", Transport: "doh", Address: "192.0.2.44:443", ServerName: "example.com", Path: "//attacker"},
		{ID: "pool", Transport: "doh", Address: "192.0.2.44:443", ServerName: "example.com", Path: "/dns-query?evil=1"},
		{ID: "pool", Transport: "udp", Address: "192.0.2.44:53", ServerName: "example.com"},
	} {
		cfg.Upstreams[0] = bad
		if _, err := Compile(cfg); err == nil {
			t.Fatalf("accepted unsafe upstream %+v", bad)
		}
	}
}
func TestDPR2DDoTVerifiedTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	// Accept both the trusted and untrusted client. The second connection
	// must reach a real TLS server, rather than merely timing out on accept.
	tlsLn := tls.NewListener(l, srv.TLS.Clone())
	defer tlsLn.Close()
	accepted := make(chan struct{}, 2)
	go func() {
		for i := 0; i < 2; i++ {
			c, e := tlsLn.Accept()
			if e != nil {
				return
			}
			accepted <- struct{}{}
			func() {
				defer c.Close()
				var hdr [2]byte
				if _, e = io.ReadFull(c, hdr[:]); e != nil {
					return
				}
				q := make([]byte, int(binary.BigEndian.Uint16(hdr[:])))
				if _, e = io.ReadFull(c, q); e != nil {
					return
				}
				ans := successReply(q)
				binary.BigEndian.PutUint16(hdr[:], uint16(len(ans)))
				_, _ = c.Write(append(hdr[:], ans...))
			}()
		}
	}()
	cfg := shadowConfig("dot")
	cfg.Upstreams[0].Address = "192.0.2.44:853"
	cfg.Upstreams[0].ServerName = "example.com"
	roots := dRoots(srv.Certificate())
	dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, l.Addr().String())
	}
	f, err := NewShadowForwarder(cfg, dOptions(dial, roots))
	if err != nil {
		t.Fatal(err)
	}
	q := queryBytes("example.com", 99)
	ans := f.resolve(context.Background(), q, "udp")
	if !sameDNSQuestion(q, ans) || ans[3]&15 != 0 {
		t.Fatalf("DoT did not return verified answer %x", ans)
	}
	// The same upstream with an untrusted root must fail closed.
	fBad, err := NewShadowForwarder(cfg, dOptions(dial, nil))
	if err != nil {
		t.Fatal(err)
	}
	bad := fBad.resolve(context.Background(), q, "udp")
	if len(bad) < 12 || bad[3]&15 != 2 {
		t.Fatalf("unverified DoT accepted: %x", bad)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-accepted:
		case <-time.After(2 * time.Second):
			t.Fatal("DoT test did not attempt both TLS handshakes")
		}
	}
}
func TestDPR2DDoHVerifiedTLS(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "POST" || r.URL.Path != "/dns-query" || r.Header.Get("Content-Type") != "application/dns-message" {
			w.WriteHeader(400)
			return
		}
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 2048))
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(successReply(raw))
	}))
	defer srv.Close()
	cfg := shadowConfig("doh")
	cfg.Upstreams[0].Address = "192.0.2.44:443"
	cfg.Upstreams[0].ServerName = "example.com"
	cfg.Upstreams[0].Path = "/dns-query"
	dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	f, err := NewShadowForwarder(cfg, dOptions(dial, dRoots(srv.Certificate())))
	if err != nil {
		t.Fatal(err)
	}
	q := queryBytes("www.example.com", 110)
	ans := f.resolve(context.Background(), q, "tcp")
	if !sameDNSQuestion(q, ans) || ans[3]&15 != 0 || calls.Load() != 1 {
		t.Fatalf("DoH failed: %x calls=%d", ans, calls.Load())
	}
	fBad, err := NewShadowForwarder(cfg, dOptions(dial, nil))
	if err != nil {
		t.Fatal(err)
	}
	bad := fBad.resolve(context.Background(), q, "tcp")
	if len(bad) < 12 || bad[3]&15 != 2 {
		t.Fatalf("unverified DoH accepted: %x", bad)
	}
}
func TestDPR2DExplicitFailoverAndNXDOMAIN(t *testing.T) {
	cfg := shadowConfig("udp")
	cfg.Upstreams[0].Fallbacks = []string{"backup"}
	cfg.Upstreams = append(cfg.Upstreams, Upstream{ID: "backup", Transport: "udp", Address: "198.51.100.10:53"})
	backup, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, e := backup.ReadFrom(buf)
			if e != nil {
				return
			}
			_, _ = backup.WriteTo(successReply(buf[:n]), addr)
		}
	}()
	var backupCalls atomic.Int32
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == cfg.Upstreams[0].Address {
			return nil, io.EOF
		}
		backupCalls.Add(1)
		return (&net.Dialer{}).DialContext(ctx, network, backup.LocalAddr().String())
	}
	f, err := NewShadowForwarder(cfg, dOptions(dial, nil))
	if err != nil {
		t.Fatal(err)
	}
	q := queryBytes("example.com", 111)
	ans := f.resolve(context.Background(), q, "udp")
	if !sameDNSQuestion(q, ans) || f.Stats().Failovers != 1 || backupCalls.Load() != 1 {
		t.Fatalf("explicit failover failed %+v", f.Stats())
	}
	// A legitimate negative DNS reply must not trigger backup.
	neg, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer neg.Close()
	go func() {
		buf := make([]byte, 1024)
		n, a, e := neg.ReadFrom(buf)
		if e == nil {
			r := successReply(buf[:n])
			r[3] |= 3
			_, _ = neg.WriteTo(r, a)
		}
	}()
	f2, err := NewShadowForwarder(cfg, dOptions(func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == cfg.Upstreams[0].Address {
			return (&net.Dialer{}).DialContext(ctx, network, neg.LocalAddr().String())
		}
		backupCalls.Add(1)
		return (&net.Dialer{}).DialContext(ctx, network, backup.LocalAddr().String())
	}, nil))
	if err != nil {
		t.Fatal(err)
	}
	n := f2.resolve(context.Background(), q, "udp")
	if len(n) < 12 || n[3]&15 != 3 || f2.Stats().Failovers != 0 {
		t.Fatalf("NXDOMAIN leaked to fallback %x", n)
	}
}

func TestDPR2DDoHRejectsRedirectAndWrongMedia(t *testing.T) {
	for _, mode := range []string{"redirect", "wrong-content-type", "bad-dns"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch mode {
				case "redirect":
					http.Redirect(w, r, "https://evil.example/dns-query", http.StatusFound)
				case "wrong-content-type":
					w.Header().Set("Content-Type", "text/html")
					_, _ = w.Write([]byte("html"))
				default:
					w.Header().Set("Content-Type", "application/dns-message")
					_, _ = w.Write([]byte("invalid"))
				}
			}))
			defer srv.Close()
			cfg := shadowConfig("doh")
			cfg.Upstreams[0].ServerName = "example.com"
			dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
			}
			f, err := NewShadowForwarder(cfg, dOptions(dial, dRoots(srv.Certificate())))
			if err != nil {
				t.Fatal(err)
			}
			r := f.resolve(context.Background(), queryBytes("example.com", 105), "udp")
			if len(r) < 12 || r[3]&15 != 2 || hits.Load() != 1 || f.Stats().Failures != 1 {
				t.Fatalf("invalid DoH accepted: mode=%s, resp=%x, calls=%d", mode, r, hits.Load())
			}
		})
	}
}
