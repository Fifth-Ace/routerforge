package dnsrouter

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// ServiceConfig selects the independent DNS service. Unlike the legacy shadow
// listener it has an explicit default and never creates per-domain ndmc rows.
type ServiceConfig struct {
	Listen            string                   `json:"listen"`
	DefaultUpstream   string                   `json:"default_upstream"`
	DefaultGroup      string                   `json:"default_group,omitempty"`
	CacheBytes        int                      `json:"cache_bytes"`
	AllowedNetworks   []string                 `json:"allowed_networks"`
	IPIntelligence    *IPIntelligenceConfig    `json:"ip_intelligence,omitempty"`
	Protection        *QueryProtectionConfig   `json:"protection,omitempty"`
	UpstreamSelection *UpstreamSelectionConfig `json:"upstream_selection,omitempty"`
	Performance       *DNSPerformanceConfig    `json:"performance,omitempty"`
	Privacy           *DNSPrivacyConfig        `json:"privacy,omitempty"`
	DNS64             *DNS64Config             `json:"dns64,omitempty"`
	ReverseDNS        *DNSReverseConfig        `json:"reverse_dns,omitempty"`
}

// Optional P28-P31 settings. Nil/disabled preserves the shipped Beta 3 path.
type UpstreamSelectionConfig struct {
	Enabled   bool   `json:"enabled"`
	Mode      string `json:"mode"` // sequential, load_balance, parallel, fastest
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}
type DNSPerformanceConfig struct {
	Enabled         bool   `json:"enabled"`
	CacheMinTTL     uint32 `json:"cache_min_ttl"`
	CacheMaxTTL     uint32 `json:"cache_max_ttl"`
	OptimisticCache bool   `json:"optimistic_cache"`
	PendingRequests string `json:"pending_requests"` // default, on, off
	MaxConcurrent   int    `json:"max_concurrent"`
}
type DNSPrivacyConfig struct {
	Enabled    bool   `json:"enabled"`
	ECSMode    string `json:"ecs_mode"` // inherit, strip, fixed
	ECSAddress string `json:"ecs_address,omitempty"`
	DNSSECMode string `json:"dnssec_mode"` // off, request_do, require_ad
}
type DNS64Config struct {
	Enabled        bool   `json:"enabled"`
	Prefix         string `json:"prefix"`
	NAT64Confirmed bool   `json:"nat64_confirmed"`
}
type DNSReverseConfig struct {
	Enabled  bool   `json:"enabled"`
	Upstream string `json:"upstream"`
}

func validateP28P31(s *ServiceConfig, upstreams map[string]Upstream) error {
	if s.UpstreamSelection != nil && s.UpstreamSelection.Enabled {
		p := s.UpstreamSelection
		switch p.Mode {
		case "sequential", "load_balance", "parallel", "fastest":
		default:
			return fmt.Errorf("invalid P28 upstream mode %q", p.Mode)
		}
		if p.TimeoutMS != 0 && (p.TimeoutMS < 250 || p.TimeoutMS > 5000) {
			return fmt.Errorf("P28 timeout must be 250..5000 ms")
		}
	}
	if s.Performance != nil && s.Performance.Enabled {
		p := s.Performance
		if p.CacheMinTTL > 3600 || p.CacheMaxTTL > 86400 || (p.CacheMaxTTL != 0 && p.CacheMinTTL > p.CacheMaxTTL) {
			return fmt.Errorf("invalid P29 cache TTL limits")
		}
		if p.OptimisticCache && s.CacheBytes == 0 {
			return fmt.Errorf("P29 optimistic cache requires enabled cache")
		}
		switch p.PendingRequests {
		case "", "default", "on", "off":
		default:
			return fmt.Errorf("invalid P29 duplicate request mode")
		}
		if p.MaxConcurrent != 0 && (p.MaxConcurrent < 8 || p.MaxConcurrent > 64) {
			return fmt.Errorf("P29 concurrency must be 8..64")
		}
	}
	if s.Privacy != nil && s.Privacy.Enabled {
		p := s.Privacy
		switch p.ECSMode {
		case "", "inherit", "strip", "fixed":
		default:
			return fmt.Errorf("invalid P30 ECS mode")
		}
		if p.ECSMode == "fixed" {
			addr, err := netip.ParseAddr(p.ECSAddress)
			if err != nil || !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
				return fmt.Errorf("P30 fixed ECS address must be public unicast")
			}
		} else if p.ECSAddress != "" {
			return fmt.Errorf("P30 ECS address set without fixed mode")
		}
		switch p.DNSSECMode {
		case "", "off", "request_do", "require_ad":
		default:
			return fmt.Errorf("invalid P30 DNSSEC mode")
		}
		if p.DNSSECMode == "require_ad" {
			// AD can only be trusted across an authenticated encrypted connection.
			// Apply a strict rule to the catalog so no policy or fallback can downgrade.
			for _, u := range upstreams {
				if u.Transport != "native" && u.Transport != "dot" && u.Transport != "doh" && u.Transport != "doq" {
					return fmt.Errorf("P30 DNSSEC AD requires encrypted upstreams: %s", u.ID)
				}
			}
		}
	}
	if s.DNS64 != nil && s.DNS64.Enabled {
		p := s.DNS64
		prefix, err := netip.ParsePrefix(p.Prefix)
		if err != nil || !prefix.Addr().Is6() || prefix.Bits() != 96 || !prefix.IsValid() || !p.NAT64Confirmed {
			return fmt.Errorf("P31 DNS64 requires an explicit IPv6 /96 prefix and confirmed NAT64 infrastructure")
		}
	}
	if s.ReverseDNS != nil && s.ReverseDNS.Enabled {
		u, ok := upstreams[s.ReverseDNS.Upstream]
		if !ok || u.Transport == "native" {
			return fmt.Errorf("P31 reverse DNS requires an explicit direct upstream")
		}
	}
	return nil
}

// IPIntelligenceConfig is optional and disabled by default.  It never starts
// health probes and is only evaluated for explicit enabled policies.
type IPIntelligenceConfig struct {
	Enabled bool            `json:"enabled"`
	Mode    string          `json:"mode"`
	Health  *IPHealthConfig `json:"health,omitempty"`
	Scope   string          `json:"scope,omitempty"` // empty/all, domains, lists
	Domains []string        `json:"domains,omitempty"`
	ListIDs []string        `json:"list_ids,omitempty"`
}

// IPHealthConfig is opt-in. Only direct public IPv4/IPv6 destinations are
// eligible; no hostnames are resolved by the probe scheduler.
type IPHealthConfig struct {
	Enabled           bool   `json:"enabled"`
	Protocol          string `json:"protocol"` // tcp or tls
	Port              int    `json:"port"`
	IntervalSeconds   int    `json:"interval_seconds"`
	TimeoutMS         int    `json:"timeout_ms"`
	FailureThreshold  int    `json:"failure_threshold"`
	RecoveryThreshold int    `json:"recovery_threshold,omitempty"`
	MinDownSeconds    int    `json:"min_down_seconds,omitempty"`
	HTTPPath          string `json:"http_path,omitempty"`
}

func validateIPHealth(h *IPHealthConfig) error {
	if h == nil {
		return nil
	}
	if h.Protocol != "" && h.Protocol != "tcp" && h.Protocol != "tls" && h.Protocol != "http" && h.Protocol != "https" {
		return fmt.Errorf("IP health supports tcp, tls, http or https")
	}
	if h.Port != 0 && (h.Port < 80 || h.Port > 65535) {
		return fmt.Errorf("IP health port must be 80..65535")
	}
	if h.IntervalSeconds != 0 && (h.IntervalSeconds < 30 || h.IntervalSeconds > 3600) {
		return fmt.Errorf("IP health interval must be 30..3600 seconds")
	}
	if h.TimeoutMS != 0 && (h.TimeoutMS < 250 || h.TimeoutMS > 3000) {
		return fmt.Errorf("IP health timeout must be 250..3000 milliseconds")
	}
	if h.FailureThreshold != 0 && (h.FailureThreshold < 1 || h.FailureThreshold > 5) {
		return fmt.Errorf("IP health failure threshold must be 1..5")
	}
	if h.RecoveryThreshold != 0 && (h.RecoveryThreshold < 1 || h.RecoveryThreshold > 5) {
		return fmt.Errorf("IP recovery threshold must be 1..5")
	}
	if h.MinDownSeconds < 0 || h.MinDownSeconds > 3600 {
		return fmt.Errorf("IP minimum down hold must be 0..3600 seconds")
	}
	if h.HTTPPath != "" {
		if len(h.HTTPPath) > 128 || h.HTTPPath[0] != '/' || strings.Contains(h.HTTPPath, "..") || strings.ContainsAny(h.HTTPPath, "?#%\\\r\n") {
			return fmt.Errorf("unsafe HTTP health path")
		}
		for _, ch := range h.HTTPPath {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || strings.ContainsRune("/_-.", ch)) {
				return fmt.Errorf("unsafe HTTP health path")
			}
		}
	}
	return nil
}

func validateIPIntelligence(cfg *IPIntelligenceConfig) error {
	if cfg == nil {
		return nil
	}
	if cfg.Health != nil && cfg.Health.Enabled && !cfg.Enabled {
		return fmt.Errorf("IP health requires enabled IP Intelligence")
	}
	if err := validateIPHealth(cfg.Health); err != nil {
		return err
	}
	switch cfg.Mode {
	case "", "original", "round_robin", "random", "fastest", "health_aware", "adaptive", "sticky", "weighted", "healthy_only":
		return nil
	default:
		return fmt.Errorf("unsupported IP Intelligence mode %q", cfg.Mode)
	}
}

func validateServiceConfig(c Config, upstreams map[string]Upstream, groups map[string]DNSGroup) error {
	s := c.Service
	if s == nil {
		return nil
	}
	if err := validateIPIntelligence(s.IPIntelligence); err != nil {
		return err
	}
	if err := validateIPIntelligenceScope(c); err != nil {
		return err
	}
	if err := validateQueryProtection(s.Protection); err != nil {
		return err
	}
	if err := validateP28P31(s, upstreams); err != nil {
		return err
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
