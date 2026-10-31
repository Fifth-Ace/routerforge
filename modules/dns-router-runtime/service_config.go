package dnsrouter

import (
	"fmt"
	"net"
	"strconv"
)

// ServiceConfig selects the independent DNS service. Unlike the legacy shadow
// listener it has an explicit default and never creates per-domain ndmc rows.
type ServiceConfig struct {
	Listen          string   `json:"listen"`
	DefaultUpstream string   `json:"default_upstream"`
	DefaultGroup    string   `json:"default_group,omitempty"`
	CacheBytes      int      `json:"cache_bytes"`
	AllowedNetworks []string `json:"allowed_networks"`
}

func validateServiceConfig(c Config, upstreams map[string]Upstream, groups map[string]DNSGroup) error {
	s := c.Service
	if s == nil {
		return nil
	}
	host, port, err := net.SplitHostPort(s.Listen)
	ip := net.ParseIP(host)
	n, pe := strconv.Atoi(port)
	if err != nil || pe != nil || n < 1025 || n > 65535 || ip == nil || (!ip.IsLoopback() && !ip.IsPrivate()) || ip.IsUnspecified() || ip.IsMulticast() {
		return fmt.Errorf("service requires an explicit private/loopback IP and high port")
	}
	if _, ok := upstreams[s.DefaultUpstream]; !ok {
		return fmt.Errorf("service default upstream is missing")
	}
	if s.DefaultGroup != "" {
		g, ok := groups[s.DefaultGroup]
		if !ok || len(g.Servers) == 0 || g.Servers[0] != s.DefaultUpstream {
			return fmt.Errorf("service default group is invalid")
		}
	}
	if s.CacheBytes < 0 || s.CacheBytes > 8<<20 {
		return fmt.Errorf("service cache must be between 0 and 8388608 bytes")
	}
	if len(s.AllowedNetworks) < 1 || len(s.AllowedNetworks) > 16 {
		return fmt.Errorf("service needs 1..16 allowed client networks")
	}
	for _, raw := range s.AllowedNetworks {
		ip, network, e := net.ParseCIDR(raw)
		if e != nil {
			return fmt.Errorf("invalid service client network")
		}
		ones, bits := network.Mask.Size()
		if (!ip.IsLoopback() && !ip.IsPrivate()) || (!network.IP.IsLoopback() && !network.IP.IsPrivate()) || ones == 0 || (bits == 32 && ones < 8) || (bits == 128 && ones < 7) {
			return fmt.Errorf("service clients must be private or loopback networks")
		}
	}
	for _, u := range upstreams {
		if u.Transport == "native" {
			continue
		}
		uhost, _, _ := net.SplitHostPort(u.Address)
		if uhost == host {
			return fmt.Errorf("service upstream %s points to this router; recursion prohibited", u.ID)
		}
	}
	return nil
}
