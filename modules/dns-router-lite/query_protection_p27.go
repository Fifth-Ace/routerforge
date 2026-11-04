package lite

// P27: opt-in query protection. Rate limiting works for the real external
// client IP at the RouterForge request handler, regardless of proxy middlewares.
// The state is bounded and entirely in memory; it never changes Gate or LAN.
import (
	"net/netip"
	"sync"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

type p27Window struct {
	start time.Time
	count int
}

type queryProtectionP27 struct {
	rateLimit int
	refuseAny bool
	bogus     []netip.Prefix
	mu        sync.Mutex
	windows   map[netip.Addr]p27Window
}

func newQueryProtectionP27(c *dr.QueryProtectionConfig) *queryProtectionP27 {
	if c == nil {
		return nil
	}
	p := &queryProtectionP27{rateLimit: c.RateLimitRPS, refuseAny: c.RefuseAny, windows: make(map[netip.Addr]p27Window)}
	for _, raw := range c.BogusNXDomain {
		if ip, e := netip.ParseAddr(raw); e == nil {
			p.bogus = append(p.bogus, netip.PrefixFrom(ip, ip.BitLen()))
			continue
		}
		if prefix, e := netip.ParsePrefix(raw); e == nil {
			p.bogus = append(p.bogus, prefix.Masked())
		}
	}
	return p
}

// evaluate returns 0 for pass, otherwise a DNS RCODE. Local service-health
// probes bypass this filter at the caller, and rate-limited requests never
// reach upstream or the cache. Fixed one-second per-source-IP accounting.
func (p *queryProtectionP27) evaluate(ip netip.Addr, kind uint16, now time.Time) int {
	if p == nil {
		return 0
	}
	if p.refuseAny && kind == dns.TypeANY {
		return dns.RcodeNotImplemented
	}
	if p.rateLimit <= 0 {
		return 0
	}
	ip = ip.Unmap()
	p.mu.Lock()
	defer p.mu.Unlock()
	w := p.windows[ip]
	if w.start.IsZero() || now.Sub(w.start) >= time.Second || now.Before(w.start) {
		w = p27Window{start: now}
	}
	w.count++
	// Bound memory AND resist identity churn. Do not evict an active client:
	// otherwise thousands of spoofed/new IPs could reset rate counters.
	if _, exists := p.windows[ip]; !exists && len(p.windows) >= 1024 {
		for addr, record := range p.windows {
			if now.Before(record.start) || now.Sub(record.start) >= time.Second {
				delete(p.windows, addr)
			}
		}
		if len(p.windows) >= 1024 {
			return dns.RcodeRefused
		}
	}
	p.windows[ip] = w
	if w.count > p.rateLimit {
		return dns.RcodeRefused
	}
	return 0
}

// convertBogusNXDomainP27 transforms only successful public-upstream A/AAAA
// responses. It is called BEFORE proxy caching, so the cached result matches
// the answer sent to the client. A new response avoids mutating shared records.
func (p *queryProtectionP27) convertBogusNXDomainP27(req, resp *dns.Msg) (*dns.Msg, bool) {
	if p == nil || len(p.bogus) == 0 || req == nil || resp == nil || resp.Rcode != dns.RcodeSuccess {
		return resp, false
	}
	for _, rr := range resp.Answer {
		var addr netip.Addr
		var ok bool
		switch v := rr.(type) {
		case *dns.A:
			addr, ok = netip.AddrFromSlice(v.A)
		case *dns.AAAA:
			addr, ok = netip.AddrFromSlice(v.AAAA)
		}
		if !ok {
			continue
		}
		addr = addr.Unmap()
		for _, prefix := range p.bogus {
			if prefix.Contains(addr) {
				out := new(dns.Msg)
				out.SetRcode(req, dns.RcodeNameError)
				out.RecursionAvailable = resp.RecursionAvailable
				out.AuthenticatedData = false
				return out, true
			}
		}
	}
	return resp, false
}
