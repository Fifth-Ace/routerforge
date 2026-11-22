package lite

// P24.1: A/AAAA response ordering only. Never mutate DNSProxy's cached result,
// CNAME, non-address records, TTLs, local answers or negative DNS replies.
// Availability probes and per-domain policy configuration belong to later stages.

import (
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/Fifth-Ace/routerforge/modules/dns-router-lite/internal/ipintel"
	"github.com/miekg/dns"
)

func responseIP(rr dns.RR, kind uint16) (netip.Addr, bool) {
	switch v := rr.(type) {
	case *dns.A:
		if kind != dns.TypeA {
			return netip.Addr{}, false
		}
		return netip.AddrFromSlice(net.IP(v.A).To4())
	case *dns.AAAA:
		if kind != dns.TypeAAAA {
			return netip.Addr{}, false
		}
		return netip.AddrFromSlice(net.IP(v.AAAA).To16())
	default:
		return netip.Addr{}, false
	}
}

func responseSigned(q, a *dns.Msg, owner string, kind uint16) bool {
	if a.AuthenticatedData {
		return true
	}
	if opt := q.IsEdns0(); opt != nil && opt.Do() {
		return true
	}
	for _, rr := range a.Answer {
		sig, ok := rr.(*dns.RRSIG)
		if ok && strings.EqualFold(sig.Hdr.Name, owner) && sig.TypeCovered == kind {
			return true
		}
	}
	return false
}

// reorderDNSAnswer works on consecutive, homogeneous answer RRsets only.  It
// never reorders glue, NS, additional records, CNAMEs, or multiple RR types.
// The returned message is a deep copy when an eligible group exists.  The
// caller must not mutate the original message after it is returned by Resolve.
func reorderDNSAnswer(p *ipintel.Planner, q, a *dns.Msg, mode ipintel.Mode, policyKey, verifiedMAC string, metrics map[netip.Addr]ipintel.Health) *dns.Msg {
	if p == nil || q == nil || a == nil || len(q.Question) != 1 ||
		a.Rcode != dns.RcodeSuccess || a.Truncated || !a.Response ||
		!mode.Valid() || mode == "" || mode == ipintel.Original {
		return a
	}
	question := q.Question[0]
	if question.Qclass != dns.ClassINET || (question.Qtype != dns.TypeA && question.Qtype != dns.TypeAAAA) {
		return a
	}
	if len(a.Answer) < 2 {
		return a
	}
	// Until P24.2 supplies real samples, health-only modes are inert.
	if len(metrics) == 0 && (mode == ipintel.Fastest || mode == ipintel.HealthAware || mode == ipintel.HealthyOnly) {
		return a
	}
	// Do not remove members from an authenticated/signed RRset.  Merely
	// reordering the records is safe; a filtered RRset would invalidate RRSIG.
	out := a.Copy()
	changed := false
	for i := 0; i < len(out.Answer); {
		hdr := out.Answer[i].Header()
		if hdr.Rrtype != question.Qtype || hdr.Class != dns.ClassINET {
			i++
			continue
		}
		_, valid := responseIP(out.Answer[i], question.Qtype)
		if !valid {
			i++
			continue
		}
		j := i + 1
		for j < len(out.Answer) {
			next := out.Answer[j].Header()
			if next.Rrtype != hdr.Rrtype || next.Class != hdr.Class || !strings.EqualFold(next.Name, hdr.Name) {
				break
			}
			if _, ok := responseIP(out.Answer[j], question.Qtype); !ok {
				break
			}
			j++
		}
		if j-i < 2 {
			i = j
			continue
		}
		ips := make([]netip.Addr, 0, j-i)
		records := make(map[netip.Addr][]dns.RR, j-i)
		for k := i; k < j; k++ {
			ip, _ := responseIP(out.Answer[k], question.Qtype)
			ips = append(ips, ip)
			records[ip] = append(records[ip], out.Answer[k])
		}
		selectedMode := mode
		if mode == ipintel.HealthyOnly && responseSigned(q, out, hdr.Name, question.Qtype) {
			selectedMode = ipintel.HealthAware
		}
		key := fmt.Sprintf("%s|%d|%s|%s", strings.ToLower(question.Name), question.Qtype, strings.ToLower(hdr.Name), policyKey)
		reordered := p.Order(selectedMode, key, verifiedMAC, ips, metrics)
		replacement := make([]dns.RR, 0, len(reordered))
		for _, ip := range reordered {
			matches := records[ip]
			if len(matches) == 0 {
				return a
			} // fail closed on unexpected planner output
			replacement = append(replacement, matches[0])
			records[ip] = matches[1:]
		}
		if len(replacement) == 0 {
			return a
		}
		if len(replacement) == j-i {
			copy(out.Answer[i:j], replacement)
		} else {
			// Only HealthyOnly can deliberately reduce an unsigned RRset.
			if selectedMode != ipintel.HealthyOnly {
				return a
			}
			updated := make([]dns.RR, 0, len(out.Answer)-(j-i)+len(replacement))
			updated = append(updated, out.Answer[:i]...)
			updated = append(updated, replacement...)
			updated = append(updated, out.Answer[j:]...)
			out.Answer = updated
		}
		changed = true
		i += len(replacement)
	}
	if !changed {
		return a
	}
	return out
}
