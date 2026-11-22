package lite

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/AdguardTeam/dnsproxy/upstream"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
	"github.com/quic-go/quic-go"
)

// P36 E2E: real network listeners and genuine mutual-TLS handshakes on Linux CI.
// All sockets use loopback. Certificates exist only in memory; no router
// settings, system CA stores or /opt/etc credentials are modified.
func p36E2ECertificates(t *testing.T) (serverPEM, serverKeyPEM, caPEM []byte, client tls.Certificate) {
	t.Helper()
	newKey := func() *ecdsa.PrivateKey {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	now := time.Now()
	caKey := newKey()
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1001), Subject: pkix.Name{CommonName: "P36 E2E ephemeral CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	issue := func(serial int64, server bool) ([]byte, []byte) {
		key := newKey()
		cert := &x509.Certificate{
			SerialNumber: big.NewInt(serial), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
			KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true,
		}
		if server {
			cert.Subject = pkix.Name{CommonName: "localhost"}
			cert.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
			cert.DNSNames = []string{"localhost"}
			cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		} else {
			cert.Subject = pkix.Name{CommonName: "P36 E2E client"}
			cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		}
		der, e := x509.CreateCertificate(rand.Reader, cert, caCert, &key.PublicKey, caKey)
		if e != nil {
			t.Fatal(e)
		}
		privateDER, e := x509.MarshalPKCS8PrivateKey(key)
		if e != nil {
			t.Fatal(e)
		}
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
			pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	}
	serverPEM, serverKeyPEM = issue(1002, true)
	cPEM, cKeyPEM := issue(1003, false)
	client, err = tls.X509KeyPair(cPEM, cKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return serverPEM, serverKeyPEM, caPEM, client
}

func p36E2EReserve(t *testing.T, network string) string {
	t.Helper()
	if network == "udp" {
		l, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		a := l.LocalAddr().String()
		if err = l.Close(); err != nil {
			t.Fatal(err)
		}
		return a
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	a := l.Addr().String()
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
	return a
}

func p36E2EClientTLS(ca []byte, client *tls.Certificate, doq bool) *tls.Config {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		panic("P36 E2E bad generated CA")
	}
	c := &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS13, ServerName: "localhost"}
	if client != nil {
		c.Certificates = []tls.Certificate{*client}
	}
	if doq {
		c.NextProtos = []string{"doq"}
	}
	return c
}

func p36E2EAnswer(t *testing.T, a *dns.Msg) {
	t.Helper()
	if a == nil || a.Rcode != dns.RcodeSuccess || len(a.Answer) != 1 || !strings.Contains(a.Answer[0].String(), "P36-E2E") {
		t.Fatalf("unexpected authenticated P36 DNS answer: %v", a)
	}
}

func p36E2EDoH(ctx context.Context, addr string, tlsConfig *tls.Config) (*dns.Msg, error) {
	q := new(dns.Msg)
	q.SetQuestion("outside.example.", dns.TypeTXT)
	raw, e := q.Pack()
	if e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+addr+"/dns-query", bytes.NewReader(raw))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/dns-message")
	tr := &http.Transport{TLSClientConfig: tlsConfig}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	resp, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH HTTP %d", resp.StatusCode)
	}
	body, e := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if e != nil {
		return nil, e
	}
	ans := new(dns.Msg)
	if e = ans.Unpack(body); e != nil {
		return nil, e
	}
	return ans, nil
}

func p36E2EDoQ(ctx context.Context, addr string, tlsConfig *tls.Config) (*dns.Msg, error) {
	conn, e := quic.DialAddr(ctx, addr, tlsConfig, &quic.Config{Allow0RTT: false})
	if e != nil {
		return nil, e
	}
	defer conn.CloseWithError(0, "P36 E2E done")
	stream, e := conn.OpenStreamSync(ctx)
	if e != nil {
		return nil, e
	}
	if e = stream.SetDeadline(time.Now().Add(5 * time.Second)); e != nil {
		return nil, e
	}
	q := new(dns.Msg)
	q.SetQuestion("outside.example.", dns.TypeTXT)
	q.Id = 0 // RFC9250 requires Message ID zero.
	raw, e := q.Pack()
	if e != nil {
		return nil, e
	}
	wire := make([]byte, len(raw)+2)
	binary.BigEndian.PutUint16(wire[:2], uint16(len(raw)))
	copy(wire[2:], raw)
	if _, e = stream.Write(wire); e != nil {
		return nil, e
	}
	if e = stream.Close(); e != nil {
		return nil, e
	}
	header := make([]byte, 2)
	if _, e = io.ReadFull(stream, header); e != nil {
		return nil, e
	}
	n := int(binary.BigEndian.Uint16(header))
	if n < 12 || n > 65535 {
		return nil, fmt.Errorf("DoQ invalid response size %d", n)
	}
	body := make([]byte, n)
	if _, e = io.ReadFull(stream, body); e != nil {
		return nil, e
	}
	ans := new(dns.Msg)
	if e = ans.Unpack(body); e != nil {
		return nil, e
	}
	if ans.Id != 0 {
		return nil, fmt.Errorf("DoQ invalid message ID %d", ans.Id)
	}
	return ans, nil
}

func TestP36E2ERealInboundMutualTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("network mTLS E2E runs in normal CI")
	}
	serverPEM, keyPEM, caPEM, clientCert := p36E2ECertificates(t)
	tlsServer, e := p36MutualTLSPEM(serverPEM, keyPEM, caPEM)
	if e != nil {
		t.Fatalf("P36 production TLS config: %v", e)
	}
	if tlsServer.MinVersion != tls.VersionTLS13 || tlsServer.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("P36 TLS security contract downgraded: %+v", tlsServer)
	}
	dot := p36E2EReserve(t, "tcp")
	doh := p36E2EReserve(t, "tcp")
	doq := p36E2EReserve(t, "udp")
	c := &dr.SecureDNSConfig{Enabled: true, DoTListen: dot, DoHListen: doh, DoQListen: doq}
	var upstreamCalls atomic.Int32
	backend := dnsFixture(t, "P36-E2E", &upstreamCalls)
	service := fixtureConfig(t)
	service.Service.CacheBytes = 0
	factory := func(_ dr.Upstream) (upstream.Upstream, error) {
		return upstream.AddressToUpstream(backend, &upstream.Options{Timeout: time.Second})
	}
	engine, e := newEngineWithTopology(service, factory, func() (string, []string, error) { return "", nil, nil })
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(engine.closeServers)
	config := &proxy.Config{
		UpstreamConfig: &proxy.UpstreamConfig{Upstreams: []upstream.Upstream{&routingUpstream{engine}}},
		RequestHandler: proxy.HandlerFunc(engine.handle),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if e = p36ConfigureListeners(config, c, tlsServer); e != nil {
		t.Fatal(e)
	}
	if len(config.TLSListenAddr) != 1 || len(config.QUICListenAddr) != 1 || config.HTTPConfig == nil ||
		len(config.HTTPConfig.ListenAddresses) != 1 || config.HTTPConfig.InsecureEnabled || config.HTTPConfig.HTTP3Enabled {
		t.Fatal("P36 secure listener configuration differs from production")
	}
	// This test reaches all three actual DNSProxy listener implementations,
	// with the very same P36 TLS policy, DNS Lite ACL/policy and trace handler.
	p, e := proxy.New(config)
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.Shutdown(ctx); err != nil {
			t.Errorf("P36 shutdown: %v", err)
		}
	})
	yes := p36E2EClientTLS(caPEM, &clientCert, false)
	no := p36E2EClientTLS(caPEM, nil, false)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	t.Run("DoT", func(t *testing.T) {
		q := new(dns.Msg)
		q.SetQuestion("outside.example.", dns.TypeTXT)
		cli := &dns.Client{Net: "tcp-tls", TLSConfig: yes, Timeout: 5 * time.Second}
		ans, _, err := cli.Exchange(q, dot)
		if err != nil {
			t.Fatal(err)
		}
		p36E2EAnswer(t, ans)
		cli.TLSConfig = no
		if a, _, err := cli.Exchange(q, dot); err == nil {
			t.Fatalf("DoT without client cert accepted: %v", a)
		}
	})
	t.Run("DoH", func(t *testing.T) {
		ans, err := p36E2EDoH(ctx, doh, yes)
		if err != nil {
			t.Fatal(err)
		}
		p36E2EAnswer(t, ans)
		if a, err := p36E2EDoH(ctx, doh, no); err == nil {
			t.Fatalf("DoH without client cert accepted: %v", a)
		}
	})
	t.Run("DoQ", func(t *testing.T) {
		ans, err := p36E2EDoQ(ctx, doq, p36E2EClientTLS(caPEM, &clientCert, true))
		if err != nil {
			t.Fatal(err)
		}
		p36E2EAnswer(t, ans)
		if a, err := p36E2EDoQ(ctx, doq, p36E2EClientTLS(caPEM, nil, true)); err == nil {
			t.Fatalf("DoQ without client cert accepted: %v", a)
		}
	})
	if upstreamCalls.Load() != 3 {
		t.Fatalf("P36 routed calls=%d; want 3", upstreamCalls.Load())
	}
	if got := engine.Traces("outside.example"); len(got) != 3 {
		t.Fatalf("P36 expected three routed trace entries, got %d", len(got))
	}
}
