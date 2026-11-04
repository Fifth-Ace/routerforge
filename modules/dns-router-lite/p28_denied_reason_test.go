package lite

import (
	"sync/atomic"
	"testing"

	"github.com/AdguardTeam/dnsproxy/upstream"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

// P28 verifies that service diagnostics never confuse client network denials
// with deliberate P27 query-protection responses. All endpoints are loopback.
func TestP28DeniedReasonCounters(t *testing.T) {
	t.Run("client_network", func(t *testing.T) {
		cfg := fixtureConfig(t)
		cfg.Service.CacheBytes = 0
		cfg.Service.AllowedNetworks = []string{"10.0.0.0/8"}
		var calls atomic.Int32
		factory := func(_ dr.Upstream) (upstream.Upstream, error) { return &p27SinkholeUpstream{calls: &calls}, nil }
		engine := startFixture(t, cfg, factory)
		got := p27Ask(t, cfg.Service.Listen, "p28-client.example.", dns.TypeA)
		if got.Rcode != dns.RcodeRefused || engine.denied.Load() != 1 || engine.aclDenied.Load() != 1 || engine.protectionDenied.Load() != 0 || calls.Load() != 0 {
			t.Fatalf("ACL classification failed: rcode=%d denied=%d acl=%d protection=%d upstream=%d", got.Rcode, engine.denied.Load(), engine.aclDenied.Load(), engine.protectionDenied.Load(), calls.Load())
		}
		m := &Manager{engine: engine, applied: cfg}
		status := m.statusLocked()
		if status["acl_denied_requests"] != uint64(1) || status["protection_denied_requests"] != uint64(0) {
			t.Fatalf("invalid API counters: %+v", status)
		}
	})
	t.Run("refuse_any", func(t *testing.T) {
		cfg := fixtureConfig(t)
		cfg.Service.CacheBytes = 0
		cfg.Service.Protection = &dr.QueryProtectionConfig{RefuseAny: true}
		var calls atomic.Int32
		factory := func(_ dr.Upstream) (upstream.Upstream, error) { return &p27SinkholeUpstream{calls: &calls}, nil }
		engine := startFixture(t, cfg, factory)
		got := p27Ask(t, cfg.Service.Listen, "p28-any.example.", dns.TypeANY)
		if got.Rcode != dns.RcodeNotImplemented || engine.requests.Load() != 1 || engine.denied.Load() != 1 || engine.aclDenied.Load() != 0 || engine.protectionDenied.Load() != 1 || calls.Load() != 0 {
			t.Fatalf("P27 classification failed: rcode=%d requests=%d denied=%d acl=%d protection=%d upstream=%d", got.Rcode, engine.requests.Load(), engine.denied.Load(), engine.aclDenied.Load(), engine.protectionDenied.Load(), calls.Load())
		}
		traces := engine.Traces("p28-any.example")
		if len(traces) != 1 || traces[0].Used != "refuse-any" || traces[0].RCode != dns.RcodeNotImplemented {
			t.Fatalf("incorrect P27 trace: %+v", traces)
		}
		m := &Manager{engine: engine, applied: cfg}
		status := m.statusLocked()
		if status["acl_denied_requests"] != uint64(0) || status["protection_denied_requests"] != uint64(1) {
			t.Fatalf("invalid API counters: %+v", status)
		}
	})
	t.Run("rate_limit", func(t *testing.T) {
		cfg := fixtureConfig(t)
		cfg.Service.CacheBytes = 0
		cfg.Service.Protection = &dr.QueryProtectionConfig{RateLimitRPS: 1}
		var calls atomic.Int32
		factory := func(_ dr.Upstream) (upstream.Upstream, error) { return &p27SinkholeUpstream{calls: &calls}, nil }
		engine := startFixture(t, cfg, factory)
		first := p27Ask(t, cfg.Service.Listen, "p28-one.example.", dns.TypeA)
		second := p27Ask(t, cfg.Service.Listen, "p28-two.example.", dns.TypeA)
		if first.Rcode != dns.RcodeSuccess || second.Rcode != dns.RcodeRefused || engine.requests.Load() != 2 || engine.denied.Load() != 1 || engine.aclDenied.Load() != 0 || engine.protectionDenied.Load() != 1 || calls.Load() != 1 {
			t.Fatalf("rate-limit classification failed: rcodes=%d/%d requests=%d denied=%d acl=%d protection=%d upstream=%d", first.Rcode, second.Rcode, engine.requests.Load(), engine.denied.Load(), engine.aclDenied.Load(), engine.protectionDenied.Load(), calls.Load())
		}
	})
}
