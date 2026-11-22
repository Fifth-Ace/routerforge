package lite

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

func TestP28P31LegacyAndOptInValidation(t *testing.T) {
	cfg := fixtureConfig(t)
	if _, err := dr.Compile(cfg); err != nil {
		t.Fatalf("legacy config changed: %v", err)
	}
	cfg.Service.UpstreamSelection = &dr.UpstreamSelectionConfig{Enabled: true, Mode: "parallel", TimeoutMS: 1500}
	cfg.Service.Performance = &dr.DNSPerformanceConfig{Enabled: true, CacheMinTTL: 20, CacheMaxTTL: 180, PendingRequests: "on", MaxConcurrent: 32}
	if _, err := dr.Compile(cfg); err != nil {
		t.Fatalf("opt-in P28/P29 refused: %v", err)
	}
	cfg.Service.Performance.CacheMaxTTL = 10
	if _, err := dr.Compile(cfg); err == nil {
		t.Fatal("must reject invalid TTL range")
	}
	cfg.Service.Performance.CacheMaxTTL = 180
	cfg.Service.DNS64 = &dr.DNS64Config{Enabled: true, Prefix: "64:ff9b::/96"}
	if _, err := dr.Compile(cfg); err == nil {
		t.Fatal("DNS64 must require confirmation of existing NAT64")
	}
	cfg.Service.DNS64.NAT64Confirmed = true
	if _, err := dr.Compile(cfg); err != nil {
		t.Fatalf("valid existing NAT64 declaration: %v", err)
	}
	cfg.Service.Privacy = &dr.DNSPrivacyConfig{Enabled: true, DNSSECMode: "require_ad"}
	if _, err := dr.Compile(cfg); err == nil {
		t.Fatal("cannot trust DNSSEC AD from plaintext DNS")
	}
}
func TestP28ExplicitChainAlgorithms(t *testing.T) {
	s := newUpstreamSelectionP28()
	chain := []dr.Upstream{{ID: "first"}, {ID: "second"}}
	original := s.order("group", "sequential", chain)
	if original[0].ID != "first" {
		t.Fatal(original)
	}
	a := s.order("group", "load_balance", chain)
	b := s.order("group", "load_balance", chain)
	if a[0].ID == b[0].ID {
		t.Fatalf("load balance did not rotate: %v, %v", a, b)
	}
	s.observe("first", 90*time.Millisecond, true)
	s.observe("second", 20*time.Millisecond, true)
	if got := s.order("group", "fastest", chain); got[0].ID != "second" {
		t.Fatalf("fastest should select measured server: %v", got)
	}
}
func TestP30ECSAndDNSSECOutboundCopy(t *testing.T) {
	e := &Engine{cfg: dr.Config{Service: &dr.ServiceConfig{Privacy: &dr.DNSPrivacyConfig{Enabled: true, ECSMode: "strip", DNSSECMode: "request_do"}}}}
	q := new(dns.Msg)
	q.SetQuestion("example.net.", dns.TypeA)
	q.SetEdns0(1232, false)
	q.IsEdns0().Option = append(q.IsEdns0().Option, &dns.EDNS0_SUBNET{Code: dns.EDNS0SUBNET, Family: 1, SourceNetmask: 24, Address: net.IPv4(203, 0, 113, 0)})
	out := e.upstreamRequestP30(q)
	if out == q || !out.IsEdns0().Do() {
		t.Fatal("DO bit missing or request not copied")
	}
	for _, opt := range out.IsEdns0().Option {
		if opt.Option() == dns.EDNS0SUBNET {
			t.Fatal("ECS not stripped")
		}
	}
	if len(q.IsEdns0().Option) != 1 || q.IsEdns0().Do() {
		t.Fatal("client query mutated")
	}
}
func TestP31OnlyPrivateReverse(t *testing.T) {
	for _, item := range []struct {
		name string
		want bool
	}{
		{"1.0.168.192.in-addr.arpa.", true},
		{"8.8.8.8.in-addr.arpa.", false},
		{"invalid.in-addr.arpa.", false},
	} {
		q := new(dns.Msg)
		q.SetQuestion(item.name, dns.TypePTR)
		if got := isPrivateReverseP31(q); got != item.want {
			t.Fatalf("%s private=%v want=%v", item.name, got, item.want)
		}
	}
}
func TestP31OptInDNS64Wire(t *testing.T) {
	cfg := fixtureConfig(t)
	cfg.Service.CacheBytes = 0
	cfg.Service.DNS64 = &dr.DNS64Config{Enabled: true, Prefix: "64:ff9b::/96", NAT64Confirmed: true}
	// An isolated fake DNS source, no Internet and no Keenetic changes.
	e := startFixture(t, cfg, func(_ dr.Upstream) (upstream.Upstream, error) {
		return &p27SinkholeUpstream{calls: new(atomic.Int32)}, nil
	})
	_ = e
	q := new(dns.Msg)
	q.SetQuestion("p31-dns64.example.", dns.TypeAAAA)
	result, _, err := (&dns.Client{Net: "udp", Timeout: 3 * time.Second}).Exchange(q, cfg.Service.Listen)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rr := range result.Answer {
		if a, ok := rr.(*dns.AAAA); ok && a.AAAA.To16() != nil {
			found = true
		}
	}
	if !found {
		t.Fatalf("DNS64 did not synthesize AAAA: %v", result)
	}
}
