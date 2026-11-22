package lite

// P33: conservative, opt-in cache for independent device-specific policies.
// DNSProxy's global cache remains OFF whenever device policies are active.
// This cache additionally keys by the observed client and applied route, never
// serves DNSSEC/ECS/EDNS-option requests, and never serves expired entries.
import (
	"net"
	"strings"
	"sync"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

type p33Entry struct {
	response *dns.Msg
	created  time.Time
	expiry   time.Time
	size     int
}

type policyCacheP33 struct {
	mu        sync.Mutex
	maxBytes  int
	bytes     int
	entries   map[string]p33Entry
	hits      uint64
	misses    uint64
	evictions uint64
}

func newPolicyCacheP33(requestedBytes int) *policyCacheP33 {
	if requestedBytes <= 0 {
		return nil
	}
	// Keep headroom for DNS message structs, Go map overhead, and allocations.
	budget := requestedBytes / 2
	if budget < 512 {
		return nil
	}
	return &policyCacheP33{maxBytes: budget, entries: make(map[string]p33Entry)}
}

func policyCacheKeyP33(route dr.ClientDecision, client, verifiedMAC string, req *dns.Msg) (string, bool) {
	if req == nil || len(req.Question) != 1 || req.CheckingDisabled || req.AuthenticatedData || req.Opcode != dns.OpcodeQuery {
		return "", false
	}
	q := req.Question[0]
	if q.Qclass != dns.ClassINET || (q.Qtype != dns.TypeA && q.Qtype != dns.TypeAAAA) {
		return "", false
	}
	if e := req.IsEdns0(); e != nil && (e.Do() || len(e.Option) != 0) {
		return "", false
	}
	ip := net.ParseIP(client)
	if ip == nil {
		return "", false
	}
	// Separate clients even if they happen to use the same upstream/group.
	// MAC is only used if verified by RouterForge's own L2 identity logic.
	parts := []string{ip.String(), verifiedMAC, route.Layer, route.Upstream, route.GroupID, route.RuleID, route.DeviceGroup,
		strings.ToLower(q.Name), string(rune(q.Qtype)), string(rune(q.Qclass))}
	if e := req.IsEdns0(); e != nil {
		parts = append(parts, "edns", string(rune(e.UDPSize())))
	} else {
		parts = append(parts, "no-edns")
	}
	if req.RecursionDesired {
		parts = append(parts, "rd")
	} else {
		parts = append(parts, "no-rd")
	}
	return strings.Join(parts, "\x00"), true
}

func cacheTTLForP33(resp *dns.Msg, minTTL, maxTTL uint32) uint32 {
	if resp == nil || resp.Rcode != dns.RcodeSuccess || resp.Truncated || resp.AuthenticatedData || len(resp.Answer) == 0 || len(resp.Ns) != 0 {
		return 0
	}
	ttl := uint32(^uint32(0))
	for _, rr := range resp.Answer {
		if rr == nil {
			return 0
		}
		hdr := rr.Header()
		if hdr.Class != dns.ClassINET || hdr.Ttl == 0 {
			return 0
		}
		if hdr.Ttl < ttl {
			ttl = hdr.Ttl
		}
	}
	// Conservative: don't keep replies that contain DNSSEC records.
	for _, rr := range resp.Answer {
		switch rr.Header().Rrtype {
		case dns.TypeRRSIG, dns.TypeDNSKEY, dns.TypeNSEC, dns.TypeNSEC3:
			return 0
		}
	}
	for _, rr := range resp.Extra {
		if rr.Header().Rrtype != dns.TypeOPT {
			return 0
		}
	}
	if minTTL > 0 && ttl < minTTL {
		ttl = minTTL
	}
	if maxTTL > 0 && ttl > maxTTL {
		ttl = maxTTL
	}
	// Bound even if upstream reports a giant TTL.
	if ttl > 86400 {
		ttl = 86400
	}
	return ttl
}

func (c *policyCacheP33) Get(key string, req *dns.Msg, now time.Time) (*dns.Msg, bool) {
	if c == nil || key == "" {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		c.misses++
		return nil, false
	}
	if !now.Before(entry.expiry) {
		c.bytes -= entry.size
		delete(c.entries, key)
		c.misses++
		return nil, false
	}
	out := entry.response.Copy()
	elapsed := uint32(now.Sub(entry.created) / time.Second)
	for _, section := range [][]dns.RR{out.Answer, out.Ns, out.Extra} {
		for _, rr := range section {
			if rr.Header().Rrtype == dns.TypeOPT {
				continue
			}
			if rr.Header().Ttl > elapsed {
				rr.Header().Ttl -= elapsed
			} else {
				rr.Header().Ttl = 0
			}
		}
	}
	out.Id = req.Id
	out.Question = append([]dns.Question(nil), req.Question...)
	c.hits++
	return out, true
}

func (c *policyCacheP33) Put(key string, response *dns.Msg, now time.Time, minTTL, maxTTL uint32) {
	if c == nil || key == "" {
		return
	}
	ttl := cacheTTLForP33(response, minTTL, maxTTL)
	if ttl == 0 {
		return
	}
	packed, err := response.Pack()
	if err != nil {
		return
	}
	// This includes a safety allowance for Go objects and map metadata.
	size := len(packed) + len(key) + 512
	if size > c.maxBytes/4 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if prev, ok := c.entries[key]; ok {
		c.bytes -= prev.size
		delete(c.entries, key)
	}
	// Oldest-first eviction is bounded by 1024 entries and done on writes.
	for c.bytes+size > c.maxBytes || len(c.entries) >= 1024 {
		var oldestKey string
		var oldest time.Time
		for k, v := range c.entries {
			if oldestKey == "" || v.created.Before(oldest) {
				oldestKey = k
				oldest = v.created
			}
		}
		if oldestKey == "" {
			return
		}
		c.bytes -= c.entries[oldestKey].size
		delete(c.entries, oldestKey)
		c.evictions++
	}
	stored := response.Copy()
	// Mirror explicit TTL overrides instead of serving zero-TTL cache hits.
	for _, rr := range stored.Answer {
		if minTTL > 0 && rr.Header().Ttl < minTTL {
			rr.Header().Ttl = minTTL
		}
		if maxTTL > 0 && rr.Header().Ttl > maxTTL {
			rr.Header().Ttl = maxTTL
		}
	}
	c.entries[key] = p33Entry{response: stored, created: now, expiry: now.Add(time.Duration(ttl) * time.Second), size: size}
	c.bytes += size
}
func (c *policyCacheP33) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]p33Entry)
	c.bytes = 0
}
func (c *policyCacheP33) Stats() map[string]any {
	if c == nil {
		return map[string]any{"enabled": false}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return map[string]any{"enabled": true, "entries": len(c.entries), "estimated_bytes": c.bytes, "budget_bytes": c.maxBytes, "hits": c.hits, "misses": c.misses, "evictions": c.evictions}
}
