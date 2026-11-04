package dnsrouter

import (
	"fmt"
	"net/netip"
)

// QueryProtectionConfig is opt-in; nil/zero means exact legacy behaviour.
// BogusNXDomain contains public resolver sinkhole IPs or small CIDR ranges.
// No protection option touches Keenetic, firewall or DNS Gate.
type QueryProtectionConfig struct {
	RateLimitRPS  int      `json:"rate_limit_rps,omitempty"`
	RefuseAny     bool     `json:"refuse_any,omitempty"`
	BogusNXDomain []string `json:"bogus_nxdomain,omitempty"`
}

func validateQueryProtection(c *QueryProtectionConfig) error {
	if c == nil {
		return nil
	}
	if c.RateLimitRPS < 0 || c.RateLimitRPS > 2000 {
		return fmt.Errorf("DNS request rate limit must be 0..2000 queries per second per client")
	}
	if len(c.BogusNXDomain) > 16 {
		return fmt.Errorf("Bogus NXDOMAIN accepts at most 16 IPs/CIDRs")
	}
	seen := map[string]struct{}{}
	for _, raw := range c.BogusNXDomain {
		var p netip.Prefix
		if ip, err := netip.ParseAddr(raw); err == nil {
			p = netip.PrefixFrom(ip, ip.BitLen())
		} else {
			var err error
			p, err = netip.ParsePrefix(raw)
			if err != nil {
				return fmt.Errorf("invalid Bogus NXDOMAIN IP/CIDR %q", raw)
			}
		}
		if !p.IsValid() || p.Addr().Is4In6() || p.Masked() != p || p.Addr().IsMulticast() {
			return fmt.Errorf("Bogus NXDOMAIN requires canonical unicast IP/prefix %q", raw)
		}
		// Fail closed on accidentally broad networks, especially /0.
		if p.Addr().Is4() && p.Bits() < 24 {
			return fmt.Errorf("IPv4 Bogus NXDOMAIN prefix must be /24 or narrower: %q", raw)
		}
		if p.Addr().Is6() && p.Bits() < 64 {
			return fmt.Errorf("IPv6 Bogus NXDOMAIN prefix must be /64 or narrower: %q", raw)
		}
		key := p.String()
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate Bogus NXDOMAIN address %q", raw)
		}
		seen[key] = struct{}{}
	}
	return nil
}
