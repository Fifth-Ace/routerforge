package lite

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

// P27 integration exercises the actual RouterForge DNS wire path, including
// policy evaluation and DNSProxy's Resolve/cache hand-off, without Internet.
type p27SinkholeUpstream struct{ calls *atomic.Int32 }

func (s *p27SinkholeUpstream) Address() string { return "p27-sinkhole-fixture" }
func (s *p27SinkholeUpstream) Close() error    { return nil }
func (s *p27SinkholeUpstream) Exchange(_ context.Context, q *dns.Msg) (*dns.Msg, error) {
	s.calls.Add(1)
	a := new(dns.Msg)
	a.SetReply(q)
	a.RecursionAvailable = true
	if q.Question[0].Qtype == dns.TypeA {
		a.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: q.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.IPv4(0, 0, 0, 0).To4()}}
	}
	return a, nil
}
func p27Ask(t *testing.T, listen, name string, kind uint16) *dns.Msg {
	t.Helper()
	q := new(dns.Msg)
	q.SetQuestion(name, kind)
	response, _, err := (&dns.Client{Net: "udp", Timeout: 2 * time.Second}).Exchange(q, listen)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
func TestP27WireProtectionOptIn(t *testing.T) {
	cfg := fixtureConfig(t)
	cfg.Service.CacheBytes = 0
	cfg.Service.Protection = &dr.QueryProtectionConfig{RefuseAny: true, BogusNXDomain: []string{"0.0.0.0"}}
	calls := new(atomic.Int32)
	factory := func(_ dr.Upstream) (upstream.Upstream, error) { return &p27SinkholeUpstream{calls}, nil }
	_ = startFixture(t, cfg, factory)
	// ANY never reaches the upstream, and a normal answer is converted to NXDOMAIN.
	any := p27Ask(t, cfg.Service.Listen, "p27-any.example.", dns.TypeANY)
	if any.Rcode != dns.RcodeNotImplemented {
		t.Fatalf("ANY response: %v", any)
	}
	if calls.Load() != 0 {
		t.Fatalf("ANY unexpectedly queried upstream: %d", calls.Load())
	}
	sink := p27Ask(t, cfg.Service.Listen, "p27-sinkhole.example.", dns.TypeA)
	if sink.Rcode != dns.RcodeNameError || len(sink.Answer) != 0 {
		t.Fatalf("Bogus NXDOMAIN response: %v", sink)
	}
	if calls.Load() != 1 {
		t.Fatalf("Bogus response should resolve once: %d", calls.Load())
	}
}
func TestP27WireRateLimitAndDefaultOff(t *testing.T) {
	cfg := fixtureConfig(t)
	cfg.Service.CacheBytes = 0
	cfg.Service.Protection = &dr.QueryProtectionConfig{RateLimitRPS: 2}
	calls := new(atomic.Int32)
	factory := func(_ dr.Upstream) (upstream.Upstream, error) { return &p27SinkholeUpstream{calls}, nil }
	_ = startFixture(t, cfg, factory)
	if result := p27Ask(t, cfg.Service.Listen, "p27-first.example.", dns.TypeA); result.Rcode != dns.RcodeSuccess {
		t.Fatal("first query refused", result)
	}
	if result := p27Ask(t, cfg.Service.Listen, "p27-second.example.", dns.TypeA); result.Rcode != dns.RcodeSuccess {
		t.Fatal("second query refused", result)
	}
	if result := p27Ask(t, cfg.Service.Listen, "p27-third.example.", dns.TypeA); result.Rcode != dns.RcodeRefused {
		t.Fatal("rate limit did not refuse third query", result)
	}
	if calls.Load() != 2 {
		t.Fatalf("refused query leaked to upstream: %d", calls.Load())
	}

	cfg = fixtureConfig(t)
	cfg.Service.CacheBytes = 0
	cfg.Service.Protection = nil // the old/simple mode remains unchanged
	offCalls := new(atomic.Int32)
	offFactory := func(_ dr.Upstream) (upstream.Upstream, error) { return &p27SinkholeUpstream{offCalls}, nil }
	_ = startFixture(t, cfg, offFactory)
	ordinary := p27Ask(t, cfg.Service.Listen, "p27-unprotected.example.", dns.TypeA)
	if ordinary.Rcode != dns.RcodeSuccess || len(ordinary.Answer) != 1 || offCalls.Load() != 1 {
		t.Fatalf("disabled protection altered DNS response: %v calls=%d", ordinary, offCalls.Load())
	}
}
