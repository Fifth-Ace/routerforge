// Package lite is the DEV independent DNS service. DNS transports and the
// bounded DNS cache use AdGuard DNSProxy; policy/list ownership stays in RF.
package lite

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/AdguardTeam/dnsproxy/upstream"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

type Trace struct {
	Client    string    `json:"client,omitempty"`
	Time      time.Time `json:"time"`
	Name      string    `json:"name"`
	Type      uint16    `json:"type"`
	Rule      string    `json:"rule,omitempty"`
	Assigned  string    `json:"assigned"`
	Used      string    `json:"used,omitempty"`
	Transport string    `json:"transport,omitempty"`
	Cache     bool      `json:"cache"`
	Success   bool      `json:"success"`
	RCode     int       `json:"rcode"`
	LatencyMS int64     `json:"latency_ms"`
}

type requestClientKey struct{}

type Factory func(dr.Upstream) (upstream.Upstream, error)

type Engine struct {
	proxy     *proxy.Proxy
	policy    *dr.Engine
	cfg       dr.Config
	servers   map[string]upstream.Upstream
	local     upstream.Upstream
	allowed   []*net.IPNet
	mu        sync.Mutex
	traces    []Trace
	requests  atomic.Uint64
	denied    atomic.Uint64
	cacheHits atomic.Uint64
	failures  atomic.Uint64
}

func transportFactory(roots *x509.CertPool) Factory {
	return func(u dr.Upstream) (upstream.Upstream, error) {
		if u.Transport == "native" {
			return nil, fmt.Errorf("DNS %s is a legacy native selector; import its settings or replace it with a direct DNS server", u.ID)
		}
		host, port, err := net.SplitHostPort(u.Address)
		if err != nil {
			return nil, err
		}
		opts := &upstream.Options{Timeout: 3 * time.Second, RootCAs: roots, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		address := u.Address
		switch u.Transport {
		case "tcp":
			address = "tcp://" + address
		case "dot", "doh", "doq":
			bootstrap, err := netip.ParseAddr(host)
			if err != nil {
				return nil, err
			}
			opts.Bootstrap = upstream.StaticResolver{bootstrap}
			scheme := map[string]string{"dot": "tls", "doh": "https", "doq": "quic"}[u.Transport]
			endpoint := &url.URL{Scheme: scheme, Host: net.JoinHostPort(u.ServerName, port)}
			if u.Transport == "doh" {
				endpoint.Path = u.Path
				if endpoint.Path == "" {
					endpoint.Path = "/dns-query"
				}
			}
			address = endpoint.String()
		}
		return upstream.AddressToUpstream(address, opts)
	}
}

func NewEngine(c dr.Config, factory Factory) (*Engine, error) {
	policy, err := dr.Compile(c)
	if err != nil {
		return nil, err
	}
	if c.Service == nil {
		return nil, errors.New("configure DNS service and its default server first")
	}
	if factory == nil {
		factory = transportFactory(nil)
	}
	e := &Engine{policy: policy, cfg: c, servers: map[string]upstream.Upstream{}}
	// Only servers reachable by a rule/default chain are constructed. Unused
	// legacy native catalog entries do not prevent an independent service start.
	needed, err := usedUpstreams(c)
	if err != nil {
		return nil, err
	}
	for _, u := range c.Upstreams {
		if !needed[u.ID] {
			continue
		}
		s, er := factory(u)
		if er != nil {
			e.closeServers()
			return nil, fmt.Errorf("upstream %s: %w", u.ID, er)
		}
		e.servers[u.ID] = s
	}
	for _, raw := range c.Service.AllowedNetworks {
		_, n, _ := net.ParseCIDR(raw)
		e.allowed = append(e.allowed, n)
	}
	// The stock proxy retains local DHCP names. This path is used exclusively
	// for local namespaces and never as a public/default fallback.
	e.local, err = upstream.AddressToUpstream("127.0.0.1:53", &upstream.Options{Timeout: 2 * time.Second})
	if err != nil {
		e.closeServers()
		return nil, err
	}
	udp, err := net.ResolveUDPAddr("udp", c.Service.Listen)
	if err != nil {
		e.closeServers()
		return nil, err
	}
	tcp, err := net.ResolveTCPAddr("tcp", c.Service.Listen)
	if err != nil {
		e.closeServers()
		return nil, err
	}
	e.proxy, err = proxy.New(&proxy.Config{
		UDPListenAddr: []*net.UDPAddr{udp}, TCPListenAddr: []*net.TCPAddr{tcp},
		UpstreamConfig: &proxy.UpstreamConfig{Upstreams: []upstream.Upstream{&routingUpstream{e}}},
		CacheEnabled:   c.Service.CacheBytes > 0, CacheSizeBytes: c.Service.CacheBytes,
		MaxGoroutines: 32, RequestHandler: proxy.HandlerFunc(e.handle),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		e.closeServers()
		return nil, err
	}
	return e, nil
}

func usedUpstreams(c dr.Config) (map[string]bool, error) {
	engine, e := dr.Compile(c)
	if e != nil {
		return nil, e
	}
	needed := map[string]bool{}
	decisions := []dr.Decision{}
	if c.Service != nil {
		decisions = append(decisions, dr.Decision{Matched: true, Source: "policy", Upstream: c.Service.DefaultUpstream, GroupID: c.Service.DefaultGroup})
	}
	for _, r := range c.Rules {
		decisions = append(decisions, dr.Decision{Matched: true, Source: "policy", Upstream: r.Upstream, GroupID: r.GroupID})
	}
	for _, d := range decisions {
		chain, e := engine.UpstreamChainForDecision(d)
		if e != nil {
			return nil, e
		}
		for _, u := range chain {
			needed[u.ID] = true
		}
	}
	return needed, nil
}

func (e *Engine) Start(ctx context.Context) error { return e.proxy.Start(ctx) }
func (e *Engine) Close(ctx context.Context) error { return e.proxy.Shutdown(ctx) }
func (e *Engine) closeServers() {
	for _, s := range e.servers {
		_ = s.Close()
	}
	if e.local != nil {
		_ = e.local.Close()
	}
}
func (e *Engine) addTrace(t Trace) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.traces) == 128 {
		copy(e.traces, e.traces[1:])
		e.traces = e.traces[:127]
	}
	e.traces = append(e.traces, t)
}
func (e *Engine) Traces(name string) []Trace {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := []Trace{}
	for _, t := range e.traces {
		if name == "" || strings.TrimSuffix(strings.ToLower(name), ".") == t.Name {
			out = append(out, t)
		}
	}
	return out
}
func localName(name string) bool {
	if !strings.Contains(name, ".") {
		return true
	}
	for _, s := range []string{"localhost", "local", "lan", "home.arpa", "in-addr.arpa", "ip6.arpa", "my.keenetic.net"} {
		if name == s || strings.HasSuffix(name, "."+s) {
			return true
		}
	}
	return false
}
func (e *Engine) handle(ctx context.Context, p *proxy.Proxy, d *proxy.DNSContext) error {
	allowed := false
	for _, n := range e.allowed {
		if n.Contains(net.IP(d.Addr.Addr().AsSlice())) {
			allowed = true
			break
		}
	}
	if !allowed || len(d.Req.Question) != 1 {
		e.requests.Add(1)
		e.denied.Add(1)
		e.failures.Add(1)
		if len(d.Req.Question) == 1 {
			e.addTrace(Trace{Time: time.Now().UTC(), Client: d.Addr.Addr().String(), Name: strings.TrimSuffix(strings.ToLower(d.Req.Question[0].Name), "."), Type: d.Req.Question[0].Qtype, Assigned: "client-acl", Used: "refused", Success: false, RCode: dns.RcodeRefused})
		}
		d.Res = new(dns.Msg)
		d.Res.SetRcode(d.Req, dns.RcodeRefused)
		return nil
	}
	e.requests.Add(1)
	ctx = context.WithValue(ctx, requestClientKey{}, d.Addr.Addr().String())
	name := strings.TrimSuffix(strings.ToLower(d.Req.Question[0].Name), ".")
	if localName(name) {
		start := time.Now()
		var err error
		d.Res, err = e.local.Exchange(ctx, d.Req)
		t := Trace{Time: time.Now().UTC(), Client: d.Addr.Addr().String(), Name: name, Type: d.Req.Question[0].Qtype, Assigned: "system-local", Used: "system-local", Success: err == nil, LatencyMS: time.Since(start).Milliseconds()}
		if err != nil {
			d.Res = new(dns.Msg)
			d.Res.SetRcode(d.Req, dns.RcodeServerFailure)
			e.failures.Add(1)
		}
		t.RCode = d.Res.Rcode
		e.addTrace(t)
		return nil
	}
	err := p.Resolve(ctx, d)
	cached := false
	if stats := d.QueryStatistics(); stats != nil {
		for _, s := range stats.Main() {
			cached = cached || s.IsCached
		}
	}
	if cached && d.Res != nil {
		e.cacheHits.Add(1)
		e.addTrace(Trace{Time: time.Now().UTC(), Client: d.Addr.Addr().String(), Name: name, Type: d.Req.Question[0].Qtype, Cache: true, Assigned: "cache", Success: err == nil, RCode: d.Res.Rcode})
	}
	return err
}

type routingUpstream struct{ e *Engine }

func (r *routingUpstream) Address() string { return "routerforge-policy" }
func (r *routingUpstream) Close() error    { r.e.closeServers(); return nil }
func (r *routingUpstream) Exchange(ctx context.Context, q *dns.Msg) (*dns.Msg, error) {
	e := r.e
	name := strings.TrimSuffix(strings.ToLower(q.Question[0].Name), ".")
	d, err := e.policy.EvaluateDNSQuestion(name)
	if err != nil {
		d = dr.Decision{}
	} // DNS wire names need not be hostnames.
	if !d.Matched {
		d = dr.Decision{Matched: true, Source: "policy", Upstream: e.cfg.Service.DefaultUpstream, GroupID: e.cfg.Service.DefaultGroup}
	}
	chain, err := e.policy.UpstreamChainForDecision(d)
	if err != nil {
		return nil, err
	}
	var last error
	for _, u := range chain {
		start := time.Now()
		s := e.servers[u.ID]
		if s == nil {
			return nil, errors.New("selected server is unavailable")
		}
		answer, er := s.Exchange(ctx, q)
		if er == nil && (answer == nil || !answer.Response || answer.Id != q.Id || len(answer.Question) != 1 || !strings.EqualFold(answer.Question[0].Name, q.Question[0].Name) || answer.Question[0].Qtype != q.Question[0].Qtype || answer.Question[0].Qclass != q.Question[0].Qclass) {
			er = errors.New("DNS response does not match the question")
		}
		client, _ := ctx.Value(requestClientKey{}).(string)
		t := Trace{Time: time.Now().UTC(), Client: client, Name: name, Type: q.Question[0].Qtype, Rule: d.RuleID, Assigned: d.Upstream, Used: u.ID, Transport: u.Transport, Success: er == nil, LatencyMS: time.Since(start).Milliseconds(), RCode: -1}
		if answer != nil {
			t.RCode = answer.Rcode
		}
		e.addTrace(t)
		if er == nil {
			return answer, nil
		} // Valid negative answers never leak to a fallback.
		last = er
	}
	e.failures.Add(1)
	return nil, fmt.Errorf("explicit DNS chain failed: %w", last)
}
