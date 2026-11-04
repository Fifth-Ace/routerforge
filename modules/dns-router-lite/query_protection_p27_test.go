package lite

import (
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

func TestP27RateLimitPerIPWindowAndConcurrency(t *testing.T) {
	p := newQueryProtectionP27(&dr.QueryProtectionConfig{RateLimitRPS: 2})
	t0 := time.Unix(12345, 0)
	a := netip.MustParseAddr("192.168.10.23")
	b := netip.MustParseAddr("192.168.10.24")
	if p.evaluate(a, dns.TypeA, t0) != 0 || p.evaluate(a, dns.TypeAAAA, t0) != 0 {
		t.Fatal("first two queries refused")
	}
	if p.evaluate(a, dns.TypeA, t0) != dns.RcodeRefused {
		t.Fatal("third query not refused")
	}
	if p.evaluate(b, dns.TypeA, t0) != 0 {
		t.Fatal("another client penalized")
	}
	if p.evaluate(a, dns.TypeA, t0.Add(time.Second)) != 0 {
		t.Fatal("window did not recover")
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); p.evaluate(b, dns.TypeA, t0.Add(2*time.Second)) }()
	}
	wg.Wait()
	if len(p.windows) > 1024 {
		t.Fatal("memory cap violated")
	}
}

func TestP27RefuseAnyAndLegacyDefault(t *testing.T) {
	t0 := time.Now()
	a := netip.MustParseAddr("192.168.10.5")
	var legacy *queryProtectionP27
	if legacy.evaluate(a, dns.TypeANY, t0) != 0 {
		t.Fatal("legacy RefuseAny changed")
	}
	p := newQueryProtectionP27(&dr.QueryProtectionConfig{RefuseAny: true})
	if got := p.evaluate(a, dns.TypeANY, t0); got != dns.RcodeNotImplemented {
		t.Fatal("ANY must be NOTIMP, got", got)
	}
	if got := p.evaluate(a, dns.TypeA, t0); got != 0 {
		t.Fatal("normal A incorrectly blocked")
	}
}

func TestP27BogusNXDomainAndUnchangedResponse(t *testing.T) {
	q := new(dns.Msg)
	q.SetQuestion("www.example.org.", dns.TypeA)
	good := new(dns.Msg)
	good.SetReply(q)
	good.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: q.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.IPv4zero}}
	p := newQueryProtectionP27(&dr.QueryProtectionConfig{BogusNXDomain: []string{"0.0.0.0"}})
	changed, yes := p.convertBogusNXDomainP27(q, good)
	if !yes || changed.Rcode != dns.RcodeNameError || len(changed.Answer) != 0 {
		t.Fatal("not converted to NXDOMAIN")
	}
	if good.Rcode != dns.RcodeSuccess || len(good.Answer) != 1 {
		t.Fatal("original response mutated")
	}
	other := new(dns.Msg)
	other.SetReply(q)
	other.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: q.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET}, A: net.ParseIP("8.8.8.8").To4()}}
	if got, yes := p.convertBogusNXDomainP27(q, other); yes || got != other {
		t.Fatal("non-bogus response changed")
	}
	p = nil
	if got, yes := p.convertBogusNXDomainP27(q, good); yes || got != good {
		t.Fatal("disabled protection changed response")
	}
}

func TestP27RateLimitBoundedStateAndNoCounterEviction(t *testing.T) {
	p := newQueryProtectionP27(&dr.QueryProtectionConfig{RateLimitRPS: 2})
	start := time.Unix(4321, 0)
	for i := 0; i < 1024; i++ {
		addr := netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)})
		if code := p.evaluate(addr, dns.TypeA, start); code != 0 {
			t.Fatalf("initial client %d denied: %d", i, code)
		}
	}
	extra := netip.MustParseAddr("192.168.10.55")
	if code := p.evaluate(extra, dns.TypeA, start); code != dns.RcodeRefused {
		t.Fatalf("new client should fail closed at capacity: %d", code)
	}
	existing := netip.AddrFrom4([4]byte{10, 0, 0, 0})
	if p.evaluate(existing, dns.TypeA, start) != 0 {
		t.Fatal("original client was evicted")
	}
	if p.evaluate(existing, dns.TypeA, start) != dns.RcodeRefused {
		t.Fatal("original rate limit reset on churn")
	}
	if len(p.windows) != 1024 {
		t.Fatal("state is unbounded", len(p.windows))
	}
	if p.evaluate(extra, dns.TypeA, start.Add(time.Second)) != 0 {
		t.Fatal("expired buckets not cleared")
	}
}
