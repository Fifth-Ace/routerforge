package lite

// P30/P31 only affect an explicitly enabled independent DNS Router service.
// No stock Keenetic DNS, NAT64 routing, DHCP, or firewall settings are touched.
import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/miekg/dns"
)

func (e *Engine) upstreamRequestP30(original *dns.Msg) *dns.Msg {
	q := original.Copy()
	if e.cfg.Service == nil || e.cfg.Service.Privacy == nil || !e.cfg.Service.Privacy.Enabled {
		return q
	}
	p := e.cfg.Service.Privacy
	if p.DNSSECMode == "request_do" || p.DNSSECMode == "require_ad" {
		if q.IsEdns0() == nil {
			q.SetEdns0(1232, true)
		} else {
			q.IsEdns0().SetDo()
		}
	}
	if p.ECSMode == "strip" || p.ECSMode == "fixed" {
		opt := q.IsEdns0()
		if opt != nil {
			kept := make([]dns.EDNS0, 0, len(opt.Option))
			for _, item := range opt.Option {
				if item.Option() != dns.EDNS0SUBNET {
					kept = append(kept, item)
				}
			}
			opt.Option = kept
		}
		if p.ECSMode == "fixed" {
			if opt == nil {
				q.SetEdns0(1232, false)
				opt = q.IsEdns0()
			}
			ip := net.ParseIP(p.ECSAddress)
			if ip != nil {
				subnet := &dns.EDNS0_SUBNET{Code: dns.EDNS0SUBNET}
				if x := ip.To4(); x != nil {
					subnet.Family = 1
					subnet.SourceNetmask = 24
					subnet.Address = x.Mask(net.CIDRMask(24, 32))
				} else {
					subnet.Family = 2
					subnet.SourceNetmask = 56
					subnet.Address = ip.To16().Mask(net.CIDRMask(56, 128))
				}
				opt.Option = append(opt.Option, subnet)
			}
		}
	}
	return q
}

func (e *Engine) upstreamDNSSECCheckP30(q, a *dns.Msg) error {
	if e.cfg.Service != nil && e.cfg.Service.Privacy != nil && e.cfg.Service.Privacy.Enabled && e.cfg.Service.Privacy.DNSSECMode == "require_ad" && !a.AuthenticatedData {
		// This is an authenticated-upstream assertion, not local chain
		// validation. Strict AD deliberately rejects unsigned domains too.
		return errors.New("trusted DNSSEC upstream did not assert authenticated data (AD)")
	}
	return nil
}

func isPrivateReverseP31(q *dns.Msg) bool {
	if q == nil || len(q.Question) != 1 || q.Question[0].Qtype != dns.TypePTR {
		return false
	}
	name := strings.TrimSuffix(strings.ToLower(q.Question[0].Name), ".")
	if strings.HasSuffix(name, ".in-addr.arpa") {
		raw := strings.TrimSuffix(name, ".in-addr.arpa")
		labels := strings.Split(raw, ".")
		if len(labels) != 4 {
			return false
		}
		parts := make([]string, 4)
		for i := 0; i < 4; i++ {
			v, e := strconv.Atoi(labels[3-i])
			if e != nil || v < 0 || v > 255 {
				return false
			}
			parts[i] = strconv.Itoa(v)
		}
		ip, e := netip.ParseAddr(strings.Join(parts, "."))
		return e == nil && (ip.IsPrivate() || ip.IsLoopback())
	}
	if strings.HasSuffix(name, ".ip6.arpa") {
		raw := strings.TrimSuffix(name, ".ip6.arpa")
		labels := strings.Split(raw, ".")
		if len(labels) != 32 {
			return false
		}
		var hexdigits strings.Builder
		for i := 31; i >= 0; i-- {
			if len(labels[i]) != 1 || !strings.ContainsAny(labels[i], "0123456789abcdef") {
				return false
			}
			hexdigits.WriteString(labels[i])
		}
		rawhex := hexdigits.String()
		groups := make([]string, 8)
		for i := 0; i < 8; i++ {
			groups[i] = rawhex[i*4 : (i+1)*4]
		}
		ip, e := netip.ParseAddr(strings.Join(groups, ":"))
		return e == nil && (ip.IsPrivate() || ip.IsLoopback())
	}
	return false
}
func (e *Engine) customReverseP31(q *dns.Msg) bool {
	return e.cfg.Service != nil && e.cfg.Service.ReverseDNS != nil && e.cfg.Service.ReverseDNS.Enabled && isPrivateReverseP31(q)
}
func (e *Engine) synthesizeDNS64P31(ctx context.Context, q, res *dns.Msg) *dns.Msg {
	if e.cfg.Service == nil || e.cfg.Service.DNS64 == nil || !e.cfg.Service.DNS64.Enabled || q == nil || res == nil || len(q.Question) != 1 {
		return res
	}
	if q.Question[0].Qtype != dns.TypeAAAA || q.Question[0].Qclass != dns.ClassINET || res.Rcode != dns.RcodeSuccess || res.AuthenticatedData {
		return res
	}
	if edns := q.IsEdns0(); edns != nil && edns.Do() && !q.CheckingDisabled {
		return res
	}
	for _, rr := range res.Answer {
		if rr.Header().Rrtype == dns.TypeAAAA {
			return res
		}
	}
	prefix, err := netip.ParsePrefix(e.cfg.Service.DNS64.Prefix)
	if err != nil || !prefix.Addr().Is6() || prefix.Bits() != 96 {
		return res
	}
	aq := q.Copy()
	aq.Question[0].Qtype = dns.TypeA
	route, er := e.policy.EvaluateClientDNSQuestion(strings.TrimSuffix(strings.ToLower(q.Question[0].Name), "."), "")
	if current, ok := ctx.Value(clientRouteKey{}).(clientRoute); ok {
		route = current.Route
		er = nil
	}
	if er != nil {
		return res
	}
	a, er := e.exchangeSelectedP28(ctx, aq, route.Decision)
	if er != nil || a == nil || a.Rcode != dns.RcodeSuccess {
		return res
	}
	out := res.Copy()
	base := prefix.Addr().As16()
	for _, rr := range a.Answer {
		record, ok := rr.(*dns.A)
		if !ok {
			continue
		}
		v4 := record.A.To4()
		if v4 == nil {
			continue
		}
		bits := base
		copy(bits[12:], v4)
		address := make(net.IP, 16)
		copy(address, bits[:])
		out.Answer = append(out.Answer, &dns.AAAA{Hdr: dns.RR_Header{Name: record.Hdr.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: record.Hdr.Ttl}, AAAA: address})
	}
	if len(out.Answer) == len(res.Answer) {
		return res
	}
	out.AuthenticatedData = false
	return out
}
