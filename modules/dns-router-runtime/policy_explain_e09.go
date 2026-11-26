package dnsrouter

import (
	"errors"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/platform/dnsaudit"
)

func diagnosticRule(r Rule) dnsaudit.Rule {
	return dnsaudit.Rule{ID: r.ID, Domain: r.Domain, Match: r.Match, Priority: r.Priority, Upstream: r.Upstream, GroupID: r.GroupID, ListID: r.ListID}
}

// Use the already compiled trie, only nodes on the queried suffix path.
func (e *Engine) diagnosticRank(domain string) dnsaudit.Ranking {
	rules := []dnsaudit.Rule{}
	if !isProtected(domain) {
		n := e.root
		labels := strings.Split(domain, ".")
		for i := len(labels) - 1; i >= 0; i-- {
			n = n.child[labels[i]]
			if n == nil {
				break
			}
			for _, r := range n.rules {
				rules = append(rules, diagnosticRule(r))
			}
		}
	}
	return dnsaudit.Rank(domain, rules)
}

type DiagnosticExplainE09 struct {
	Decision         ClientDecision   `json:"decision"`
	Shared           dnsaudit.Ranking `json:"shared"`
	Device           dnsaudit.Ranking `json:"device"`
	AllowedUpstreams []string         `json:"allowed_upstreams"`
	SimulatedMAC     bool             `json:"simulated_mac"`
	Scope            string           `json:"scope"`
}

func rankAgrees(r dnsaudit.Ranking, d Decision) bool {
	if !d.Matched {
		return r.Total == 0
	}
	return len(r.Rows) > 0 && r.Rows[0].ID == d.RuleID
}

// ExplainE09 calls the same native decision and chain resolver as forwarding.
// A hypothetical MAC does not assert actual client identity. No DNS packets,
// health selection, local record lookup, ACL or cache lookup are performed.
func (e *Engine) ExplainE09(name, mac string) (DiagnosticExplainE09, error) {
	out := DiagnosticExplainE09{Shared: dnsaudit.Ranking{Rows: []dnsaudit.Candidate{}}, Device: dnsaudit.Ranking{Rows: []dnsaudit.Candidate{}}, AllowedUpstreams: []string{}, SimulatedMAC: mac != "", Scope: "POLICY_ONLY"}
	if e == nil || e.root == nil {
		return out, errors.New("POLICY_UNAVAILABLE")
	}
	d, err := normalizedDomain(name)
	if err != nil {
		return out, errors.New("INVALID_DOMAIN")
	}
	if mac != "" {
		canonical, err := CanonicalDeviceMAC(mac)
		if err != nil || canonical != mac {
			return out, errors.New("INVALID_MAC")
		}
	}
	route, err := e.EvaluateClientDNSQuestion(d, mac)
	if err != nil {
		return out, errors.New("POLICY_UNAVAILABLE")
	}
	out.Decision = route
	if route.Layer == "local_namespace" {
		return out, nil
	}
	common, err := e.EvaluateDNSQuestion(d)
	if err != nil {
		return out, errors.New("POLICY_UNAVAILABLE")
	}
	out.Shared = e.diagnosticRank(d)
	if !rankAgrees(out.Shared, common) {
		return out, errors.New("EXPLANATION_MISMATCH")
	}
	if mac != "" {
		if dev := e.devices.rules[mac]; dev != nil {
			dd, err := dev.EvaluateDNSQuestion(d)
			if err != nil {
				return out, errors.New("POLICY_UNAVAILABLE")
			}
			out.Device = dev.diagnosticRank(d)
			if !rankAgrees(out.Device, dd) {
				return out, errors.New("EXPLANATION_MISMATCH")
			}
		}
	}
	if route.Source == "policy" {
		chain, err := e.UpstreamChainForDecision(route.Decision)
		if err != nil {
			return out, errors.New("POLICY_CHAIN_UNAVAILABLE")
		}
		for _, u := range chain {
			out.AllowedUpstreams = append(out.AllowedUpstreams, u.ID)
		}
	}
	return out, nil
}

type ListAuditE09 struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Kind              string `json:"kind"`
	Rules             int    `json:"rules"`
	AutoUpdateMinutes int    `json:"auto_update_minutes"`
	UpdatedAt         string `json:"updated_at,omitempty"`
	AgeSeconds        *int64 `json:"age_seconds"`
}
type PolicyAuditE09 struct {
	Rules           int                `json:"rules"`
	RuleLimit       int                `json:"rule_limit"`
	Upstreams       int                `json:"upstreams"`
	Groups          int                `json:"groups"`
	Lists           []ListAuditE09     `json:"lists"`
	Conflicts       dnsaudit.Conflicts `json:"conflicts"`
	DNSSECMode      string             `json:"dnssec_mode"`
	LocalValidation bool               `json:"local_dnssec_validation"`
	DNS64Enabled    bool               `json:"dns64_enabled"`
	DNS64Prefix     string             `json:"dns64_prefix,omitempty"`
	NAT64Declared   bool               `json:"nat64_declared"`
	CacheBytes      int                `json:"cache_bytes"`
	CacheMinTTL     *uint32            `json:"cache_min_ttl"`
	CacheMaxTTL     *uint32            `json:"cache_max_ttl"`
	Optimistic      bool               `json:"optimistic_cache_configured"`
	PolicyCache     bool               `json:"policy_cache_configured"`
}

func AuditPolicyE09(c Config, now time.Time) PolicyAuditE09 {
	out := PolicyAuditE09{Rules: len(c.Rules), RuleLimit: MaxRules, Upstreams: len(c.Upstreams), Groups: len(c.Groups), Lists: []ListAuditE09{}, DNSSECMode: "off"}
	counts := map[string]int{}
	rules := make([]dnsaudit.Rule, 0, len(c.Rules))
	for _, r := range c.Rules {
		counts[r.ListID]++
		rules = append(rules, diagnosticRule(r))
	}
	out.Conflicts = dnsaudit.Audit(rules)
	for _, l := range c.Lists {
		row := ListAuditE09{ID: l.ID, Name: l.Name, Kind: l.Kind, Rules: counts[l.ID], AutoUpdateMinutes: l.AutoUpdateMinutes, UpdatedAt: l.UpdatedAt}
		if t, err := time.Parse(time.RFC3339, l.UpdatedAt); err == nil && !t.After(now) {
			age := int64(now.Sub(t) / time.Second)
			row.AgeSeconds = &age
		}
		out.Lists = append(out.Lists, row)
	}
	if s := c.Service; s != nil {
		out.CacheBytes = s.CacheBytes
		if p := s.Privacy; p != nil && p.Enabled && p.DNSSECMode != "" {
			out.DNSSECMode = p.DNSSECMode
		}
		if p := s.DNS64; p != nil && p.Enabled {
			out.DNS64Enabled = true
			out.DNS64Prefix = p.Prefix
			out.NAT64Declared = p.NAT64Confirmed
		}
		if p := s.Performance; p != nil && p.Enabled {
			min, max := p.CacheMinTTL, p.CacheMaxTTL
			out.CacheMinTTL = &min
			out.CacheMaxTTL = &max
			out.Optimistic = p.OptimisticCache
			out.PolicyCache = p.DevicePolicyCache
		}
	}
	return out
}

// SavedDNSFeaturesE09 is deliberately small and contains no destinations,
// URLs, domains, digests, MACs or credentials. The /status endpoint reports
// saved features, never a claim about the active resolver or NAT64 reachability.
type SavedDNSFeaturesE09 struct {
	Lists         int    `json:"lists"`
	DNSSECMode    string `json:"dnssec_mode"`
	DNS64Enabled  bool   `json:"dns64_enabled"`
	NAT64Declared bool   `json:"nat64_declared"`
}

func SavedFeaturesE09(c Config) SavedDNSFeaturesE09 {
	out := SavedDNSFeaturesE09{Lists: len(c.Lists), DNSSECMode: "off"}
	if s := c.Service; s != nil {
		if p := s.Privacy; p != nil && p.Enabled && (p.DNSSECMode == "request_do" || p.DNSSECMode == "require_ad") {
			out.DNSSECMode = p.DNSSECMode
		}
		if p := s.DNS64; p != nil && p.Enabled {
			out.DNS64Enabled = true
			out.NAT64Declared = p.NAT64Confirmed
		}
	}
	return out
}
