package lite

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

func fixtureConfig(t *testing.T) dr.Config {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := l.Addr().String()
	l.Close()
	c := dr.Config{SchemaVersion: 1, Upstreams: []dr.Upstream{{ID: "default", Transport: "udp", Address: "192.0.2.1:53"}, {ID: "chosen", Transport: "udp", Address: "192.0.2.2:53"}, {ID: "other", Transport: "doh", Address: "192.0.2.3:443", ServerName: "example.com", Path: "/dns-query"}}, Service: &dr.ServiceConfig{Listen: addr, DefaultUpstream: "default", CacheBytes: 65536, AllowedNetworks: []string{"127.0.0.0/8"}}}
	for i := 0; i < 171; i++ {
		c.Rules = append(c.Rules, dr.Rule{ID: fmt.Sprintf("r%d", i), Domain: fmt.Sprintf("domain%d.example", i), Match: "suffix", Upstream: "chosen"})
	}
	c.Rules = append(c.Rules, dr.Rule{ID: "exact", Domain: "api.domain0.example", Match: "exact", Upstream: "other"})
	return c
}
func dnsFixture(t *testing.T, marker string, count *atomic.Int32) string {
	t.Helper()
	tcp, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	udp, e := net.ListenPacket("udp", tcp.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	h := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		count.Add(1)
		m := new(dns.Msg)
		m.SetReply(r)
		if strings.Contains(r.Question[0].Name, "nx.") {
			m.Rcode = dns.RcodeNameError
		} else if strings.Contains(r.Question[0].Name, "sf.") {
			m.Rcode = dns.RcodeServerFailure
		} else if strings.Contains(r.Question[0].Name, "tc.") && w.RemoteAddr().Network() == "udp" {
			m.Truncated = true
		} else {
			rr, _ := dns.NewRR(r.Question[0].Name + " 60 IN TXT \"" + marker + "\"")
			m.Answer = []dns.RR{rr}
		}
		_ = w.WriteMsg(m)
	})
	a := &dns.Server{Listener: tcp, Handler: h}
	b := &dns.Server{PacketConn: udp, Handler: h}
	go a.ActivateAndServe()
	go b.ActivateAndServe()
	t.Cleanup(func() { _ = a.Shutdown(); _ = b.Shutdown() })
	return tcp.Addr().String()
}
func startFixture(t *testing.T, c dr.Config, f Factory) *Engine {
	t.Helper()
	e, err := NewEngine(c, f)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close(context.Background()) })
	return e
}
func ask(t *testing.T, addr, network, name string) *dns.Msg {
	t.Helper()
	q := new(dns.Msg)
	q.SetQuestion(name+".", dns.TypeTXT)
	a, _, e := (&dns.Client{Net: network, Timeout: 4 * time.Second}).Exchange(q, addr)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func TestIndependentDNS171RulesDefaultExactTCPAndCache(t *testing.T) {
	var dc, cc atomic.Int32
	d := dnsFixture(t, "DEFAULT", &dc)
	chosen := dnsFixture(t, "CHOSEN", &cc)
	c := fixtureConfig(t)
	f := func(u dr.Upstream) (upstream.Upstream, error) {
		dest := d
		if u.ID == "chosen" {
			dest = chosen
		}
		return upstream.AddressToUpstream(dest, &upstream.Options{Timeout: time.Second})
	}
	e := startFixture(t, c, f)
	for _, network := range []string{"udp", "tcp"} {
		for _, name := range []string{"n.domain0.example", "n.domain170.example", "_sip._tcp.domain0.example", "outside.example"} {
			a := ask(t, c.Service.Listen, network, name)
			want := "CHOSEN"
			if name == "outside.example" {
				want = "DEFAULT"
			}
			if len(a.Answer) != 1 || !strings.Contains(a.Answer[0].String(), want) {
				t.Fatalf("%s %s: %v", network, name, a)
			}
		}
	}
	if e.cacheHits.Load() == 0 {
		t.Fatal("cache was not used")
	}
	trace := e.Traces("n.domain170.example")
	if len(trace) == 0 || trace[0].Assigned != "chosen" || trace[0].Used != "chosen" {
		t.Fatal(trace)
	}
	a := ask(t, c.Service.Listen, "udp", "api.domain0.example")
	if !strings.Contains(a.Answer[0].String(), "DEFAULT") {
		t.Fatal("exact rule was ignored")
	}
	a = ask(t, c.Service.Listen, "udp", "sub.api.domain0.example")
	if !strings.Contains(a.Answer[0].String(), "CHOSEN") {
		t.Fatal("exact rule incorrectly captured subdomains")
	}
}
func TestIndependentDNSNegativeNoLeakAndTransportFallback(t *testing.T) {
	var dc, cc atomic.Int32
	d := dnsFixture(t, "BACKUP", &dc)
	chosen := dnsFixture(t, "PRIMARY", &cc)
	c := fixtureConfig(t)
	c.Service.CacheBytes = 0
	c.Upstreams[1].Fallbacks = []string{"default"}
	f := func(u dr.Upstream) (upstream.Upstream, error) {
		dest := d
		if u.ID == "chosen" {
			dest = chosen
		}
		return upstream.AddressToUpstream(dest, &upstream.Options{Timeout: time.Second})
	}
	e := startFixture(t, c, f)
	for _, name := range []string{"nx.domain0.example", "sf.domain0.example"} {
		a := ask(t, c.Service.Listen, "udp", name)
		if a.Rcode != dns.RcodeNameError && a.Rcode != dns.RcodeServerFailure {
			t.Fatal(a)
		}
	}
	if dc.Load() != 0 {
		t.Fatal("negative DNS answer leaked to backup")
	}
	a := ask(t, c.Service.Listen, "tcp", "tc.domain0.example")
	if a.Truncated || len(a.Answer) == 0 {
		t.Fatal("UDP truncation was not recovered over TCP")
	}
	if len(e.Traces("tc.domain0.example")) != 1 {
		t.Fatal("trace attribution missing")
	}
}
func TestIndependentDNSACLAndTraceBound(t *testing.T) {
	var n atomic.Int32
	dest := dnsFixture(t, "OK", &n)
	c := fixtureConfig(t)
	c.Service.CacheBytes = 0
	c.Service.AllowedNetworks = []string{"10.0.0.0/8"}
	f := func(dr.Upstream) (upstream.Upstream, error) {
		return upstream.AddressToUpstream(dest, &upstream.Options{Timeout: time.Second})
	}
	engine := startFixture(t, c, f)
	a := ask(t, c.Service.Listen, "udp", "outside.example")
	if a.Rcode != dns.RcodeRefused || n.Load() != 0 {
		t.Fatal("ACL was bypassed")
	}
	if engine.denied.Load() != 1 || engine.requests.Load() != 1 {
		t.Fatal("ACL denial did not update incoming request counters")
	}
	traces := engine.Traces("outside.example")
	if len(traces) != 1 || traces[0].Used != "refused" || traces[0].Client != "127.0.0.1" || traces[0].RCode != dns.RcodeRefused {
		t.Fatalf("ACL denial not visible in live diagnostics: %+v", traces)
	}
	e := &Engine{}
	for i := 0; i < 200; i++ {
		e.addTrace(Trace{Name: fmt.Sprint(i)})
	}
	if len(e.Traces("")) != 128 || len(e.Traces("0")) != 0 {
		t.Fatal("unbounded trace")
	}
}
func TestIndependentDNSDoHAndDoTPooledTLS(t *testing.T) {
	var count atomic.Int32
	handler := func(q *dns.Msg) *dns.Msg {
		count.Add(1)
		a := new(dns.Msg)
		a.SetReply(q)
		rr, _ := dns.NewRR(q.Question[0].Name + " 60 IN TXT \"TLS\"")
		a.Answer = []dns.RR{rr}
		return a
	}
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dns-query" {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		if r.Method == "GET" {
			raw, _ = base64.RawURLEncoding.DecodeString(r.URL.Query().Get("dns"))
		}
		q := new(dns.Msg)
		if q.Unpack(raw) != nil {
			w.WriteHeader(400)
			return
		}
		a, _ := handler(q).Pack()
		w.Header().Set("Content-Type", "application/dns-message")
		w.Write(a)
	}))
	defer ts.Close()
	pool := x509.NewCertPool()
	pool.AddCert(ts.Certificate())
	baseTLS := ts.TLS.Clone()
	baseTLS.NextProtos = []string{"dot"}
	l, err := tls.Listen("tcp", "127.0.0.1:0", baseTLS)
	if err != nil {
		t.Fatal(err)
	}
	dot := &dns.Server{Listener: l, Handler: dns.HandlerFunc(func(w dns.ResponseWriter, q *dns.Msg) { w.WriteMsg(handler(q)) })}
	go dot.ActivateAndServe()
	defer dot.Shutdown()
	c := fixtureConfig(t)
	c.Service.CacheBytes = 0
	c.Upstreams[0] = dr.Upstream{ID: "default", Transport: "dot", Address: "192.0.2.1:853", ServerName: "example.com"}
	f := func(u dr.Upstream) (upstream.Upstream, error) {
		if u.ID == "other" {
			u.Address = strings.TrimPrefix(ts.URL, "https://")
		} else {
			u.Transport = "dot"
			u.Address = l.Addr().String()
			u.ServerName = "example.com"
		}
		return transportFactory(pool)(u)
	}
	e := startFixture(t, c, f)
	for _, name := range []string{"outside.example", "api.domain0.example"} {
		a := ask(t, c.Service.Listen, "tcp", name)
		if a.Rcode != 0 || len(a.Answer) == 0 {
			t.Fatal(a)
		}
	}
	if count.Load() != 2 || len(e.Traces("api.domain0.example")) != 1 {
		t.Fatal("real TLS route not proven")
	}
}

type brokenUpstream struct {
	calls *atomic.Int32
	id    string
}

func (u *brokenUpstream) Address() string { return u.id }
func (u *brokenUpstream) Close() error    { return nil }
func (u *brokenUpstream) Exchange(context.Context, *dns.Msg) (*dns.Msg, error) {
	u.calls.Add(1)
	return nil, fmt.Errorf("offline")
}
func TestIndependentDNSExplicitFailover(t *testing.T) {
	var failed, backup atomic.Int32
	dest := dnsFixture(t, "BACKUP", &backup)
	c := fixtureConfig(t)
	c.Upstreams[1].Fallbacks = []string{"default"}
	f := func(u dr.Upstream) (upstream.Upstream, error) {
		if u.ID == "chosen" {
			return &brokenUpstream{&failed, u.ID}, nil
		}
		return upstream.AddressToUpstream(dest, &upstream.Options{Timeout: time.Second})
	}
	e := startFixture(t, c, f)
	a := ask(t, c.Service.Listen, "udp", "n.domain0.example")
	if a.Rcode != 0 || failed.Load() != 1 || backup.Load() != 1 {
		t.Fatal("explicit failover failed")
	}
	tr := e.Traces("n.domain0.example")
	if len(tr) != 2 || tr[1].Assigned != "chosen" || tr[1].Used != "default" {
		t.Fatal(tr)
	}
}

func TestP19ReadinessUsesRealDualStackDNSAndOwnAttribution(t *testing.T) {
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("BLOCKED: IPv6 loopback unavailable: %v", err)
	}
	listener.Close()
	var count atomic.Int32
	dest := dnsFixture(t, "READY", &count)
	c := fixtureConfig(t)
	e, err := newEngineWithTopology(c, func(dr.Upstream) (upstream.Upstream, error) {
		return upstream.AddressToUpstream(dest, &upstream.Options{})
	}, func() (string, []string, error) { return "192.168.10.1/24", []string{"fd12:3456::1/64"}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Close(context.Background())
	_, port, _ := net.SplitHostPort(c.Service.Listen)
	p := GatePlan{Port: port, Families: []GateFamily{{Family: "inet"}, {Family: "inet6"}}}
	if err = e.GateReady(context.Background(), c.Service.Listen, p); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 0 || e.requests.Load() != 0 {
		t.Fatalf("readiness must exercise four real transports without inflating client requests: %d / %d", count.Load(), e.requests.Load())
	}
	traces := e.readiness
	if len(e.Traces("")) != 0 {
		t.Fatal("internal readiness polluted client traces")
	}
	if len(traces) != 4 {
		t.Fatal("missing readiness attribution")
	}
	for _, tr := range traces {
		if !tr.Internal || tr.Cache || !tr.Success || tr.Used != "service-readiness" {
			t.Fatalf("bad readiness trace: %+v", tr)
		}
	}
}

func TestP19LocalNamespacesForwardOnlyToStockDNS(t *testing.T) {
	var localCount, publicCount atomic.Int32
	local := dnsFixture(t, "LOCAL", &localCount)
	public := dnsFixture(t, "PUBLIC", &publicCount)
	c := fixtureConfig(t)
	c.Service.CacheBytes = 0
	e, err := NewEngine(c, func(dr.Upstream) (upstream.Upstream, error) {
		return upstream.AddressToUpstream(public, &upstream.Options{Timeout: time.Second})
	})
	if err != nil {
		t.Fatal(err)
	}
	// Replace only the endpoint for stock DNS; the production local-name routing
	// and real UDP/TCP exchange remain intact. No host port 53 or ndmc is touched.
	_ = e.local.Close()
	e.local, err = upstream.AddressToUpstream(local, &upstream.Options{Timeout: time.Second})
	if err != nil {
		e.closeServers()
		t.Fatal(err)
	}
	if err = e.Start(context.Background()); err != nil {
		e.closeServers()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close(context.Background()) })
	names := []string{"router.lan", "printer.local", "router.HOME.ARPA", "localhost", "1.10.168.192.in-addr.arpa", "1.0.ip6.arpa", "my.keenetic.net"}
	for _, network := range []string{"udp", "tcp"} {
		for _, name := range names {
			answer := ask(t, c.Service.Listen, network, name)
			if len(answer.Answer) != 1 || !strings.Contains(answer.Answer[0].String(), "LOCAL") {
				t.Fatal("local question leaked", name, answer)
			}
			traces := e.Traces(strings.ToLower(name))
			if len(traces) == 0 || traces[0].Assigned != "system-local" || traces[0].Used != "system-local" {
				t.Fatal("local attribution lost", traces)
			}
		}
	}
	if publicCount.Load() != 0 || localCount.Load() != int32(2*len(names)) {
		t.Fatal("local namespace sent to public upstream")
	}
	answer := ask(t, c.Service.Listen, "udp", "public.example")
	if len(answer.Answer) != 1 || !strings.Contains(answer.Answer[0].String(), "PUBLIC") || publicCount.Load() != 1 {
		t.Fatal("public default forwarded back into stock DNS", answer)
	}
}
