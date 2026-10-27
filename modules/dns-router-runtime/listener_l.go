package dnsrouter

// DPR2-L: revision-pinned, declarative listener preferences and read-only
// Keenetic zone preview. This file NEVER invokes ndmc or changes host DNS.

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
)

type ListenerConfig struct {
	Mode           string   `json:"mode"`   // loopback or lan
	Listen         string   `json:"listen"` // explicit IP and unprivileged high port
	AllowedClients []string `json:"allowed_clients,omitempty"`
}

func ValidateListenerConfigL(v *ListenerConfig) error {
	if v == nil {
		return nil
	} // legacy configuration: CLI loopback default
	if v.Mode != "loopback" && v.Mode != "lan" {
		return errors.New("listener mode must be loopback or lan")
	}
	host, port, err := net.SplitHostPort(v.Listen)
	if err != nil {
		return fmt.Errorf("listener address: %w", err)
	}
	ip := net.ParseIP(host)
	n, err := strconv.Atoi(port)
	if err != nil || n < 1025 || n > 65535 {
		return errors.New("listener must use a high port")
	}
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.String() != host {
		return errors.New("listener requires a canonical literal unicast IP")
	}
	if v.Mode == "loopback" {
		if !ip.IsLoopback() || len(v.AllowedClients) != 0 {
			return errors.New("loopback listener must be loopback with no ACL")
		}
		return nil
	}
	if ip.To4() == nil || ip.IsLoopback() || !ip.IsPrivate() {
		return errors.New("LAN listener requires canonical private IPv4 address")
	}
	if len(v.AllowedClients) < 1 || len(v.AllowedClients) > 8 {
		return errors.New("LAN listener needs 1..8 exact clients")
	}
	seen := map[string]bool{}
	for _, item := range v.AllowedClients {
		a := net.ParseIP(item)
		if a == nil || a.To4() == nil || a.String() != item || (!a.IsPrivate() && !a.IsLoopback()) || seen[item] {
			return errors.New("LAN ACL contains non-canonical, duplicate or non-private IPv4")
		}
		seen[item] = true
	}
	return nil
}

// ResolveListenerOptionsL applies a saved listener preference to the lifecycle.
// The management daemon's explicit --shadow-lan-experimental flag remains the
// non-persistent permission gate for ANY LAN activation. Saved config alone
// never opens a socket or grants permission to activate.
func ResolveListenerOptionsL(cfg Config, capability ShadowOptions) (ShadowOptions, error) {
	opts := capability
	if cfg.Listener == nil {
		host, _, err := net.SplitHostPort(opts.Listen)
		if err == nil && net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback() && len(opts.AllowedClients) == 0 {
			opts.AllowLAN = false // management-only permission; no implicit LAN activation
		}
		return opts, nil
	}
	if err := ValidateListenerConfigL(cfg.Listener); err != nil {
		return opts, err
	}
	opts.Listen = cfg.Listener.Listen
	if cfg.Listener.Mode == "lan" {
		if !capability.AllowLAN {
			return opts, errors.New("LAN runtime capability not enabled: start daemon with --serve --shadow-lan-experimental")
		}
		opts.AllowLAN = true
		opts.AllowedClients = append([]string(nil), cfg.Listener.AllowedClients...)
	} else {
		opts.AllowLAN = false
		opts.AllowedClients = nil
	}
	return opts, nil
}

type DomainSplitZoneL struct {
	Domain string `json:"domain"`
	Add    string `json:"add"`
	Remove string `json:"remove"`
}

type DomainSplitPlanL struct {
	Mode                        string             `json:"mode"`
	Ready                       bool               `json:"ready"`
	Listener                    string             `json:"listener,omitempty"`
	Zones                       []DomainSplitZoneL `json:"zones"`
	Warnings                    []string           `json:"warnings"`
	ChangesApplied              bool               `json:"changes_applied"`
	RequiresExplicitTransaction bool               `json:"requires_explicit_transaction"`
}

// BuildDomainSplitPlanL reports exact domain-scoped ndmc operations without
// executing them. Kept separate from the resolver policy: exact and
// subdomains-only rules cannot be advertised as domain-wide DNS routes.
func BuildDomainSplitPlanL(cfg Config) DomainSplitPlanL {
	result := DomainSplitPlanL{Mode: "preview-only", Zones: []DomainSplitZoneL{}, Warnings: []string{}, RequiresExplicitTransaction: true}
	if _, err := Compile(cfg); err != nil {
		result.Warnings = append(result.Warnings, "invalid saved configuration: "+err.Error())
		return result
	}
	if cfg.Listener == nil || cfg.Listener.Mode != "lan" {
		result.Warnings = append(result.Warnings, "configure explicit private LAN listener first")
		return result
	}
	result.Listener = cfg.Listener.Listen
	listenerIP, _, _ := net.SplitHostPort(cfg.Listener.Listen)
	for _, u := range cfg.Upstreams {
		host, port, err := net.SplitHostPort(u.Address)
		if err == nil && host == listenerIP && port == "53" {
			result.Warnings = append(result.Warnings, "DNS recursion risk: upstream "+u.ID+" points to the same router DNS Proxy on port 53")
			return result
		}
	}
	// ndnproxy may source queries from the LAN IP or local loopback; both are
	// required in a preview plan rather than guessing the daemon's source.
	host, _, _ := net.SplitHostPort(cfg.Listener.Listen)
	acl := map[string]bool{}
	for _, v := range cfg.Listener.AllowedClients {
		acl[v] = true
	}
	if !acl[host] || !acl["127.0.0.1"] {
		result.Warnings = append(result.Warnings, "include exact LAN address and 127.0.0.1 in source ACL for confirmed DSTest integration")
		return result
	}
	domains := map[string]bool{}
	unsupported := 0
	for _, r := range cfg.Rules {
		if r.Match != "suffix" {
			unsupported++
			continue
		}
		domain, _ := normalizedDomain(r.Domain)
		domains[domain] = true
	}
	if unsupported > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d exact/subdomain-only rules are not automatically mapped into DNS Proxy zones", unsupported))
	}
	ordered := make([]string, 0, len(domains))
	for d := range domains {
		ordered = append(ordered, d)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if len(ordered[i]) == len(ordered[j]) {
			return ordered[i] < ordered[j]
		}
		return len(ordered[i]) < len(ordered[j])
	})
	for _, d := range ordered {
		if !strings.Contains(d, ".") {
			result.Warnings = append(result.Warnings, "top-level-only zone "+d+" cannot be routed safely")
			return result
		}
		subsumed := false
		for _, z := range result.Zones {
			if d == z.Domain || strings.HasSuffix(d, "."+z.Domain) {
				subsumed = true
				break
			}
		}
		if subsumed {
			continue
		}
		result.Zones = append(result.Zones, DomainSplitZoneL{Domain: d, Add: "ip name-server " + cfg.Listener.Listen + " " + d, Remove: "no ip name-server " + cfg.Listener.Listen + " " + d})
	}
	if len(result.Zones) == 0 {
		result.Warnings = append(result.Warnings, "no suffix rules to map")
	}
	if len(result.Zones) > 32 {
		result.Warnings = append(result.Warnings, "more than 32 mapped zones are not allowed for managed activation")
	}
	result.Ready = len(result.Zones) > 0 && len(result.Zones) <= 32
	// Even Ready=true permits preview ONLY; no router changes at this stage.
	return result
}
