package dnsrouter

// DPR2-P6: explicit fail-closed DNS groups.
// A grouped rule can contact ONLY servers explicitly listed in its group.
// Never traverse legacy Upstream.Fallbacks or select an implicit system DNS.

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxDNSGroups = 64
const maxDNSGroupServers = 8

type DNSGroup struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Servers       []string `json:"servers"`                  // ordered, first is primary; remaining are explicit backups
	EncryptedOnly bool     `json:"encrypted_only,omitempty"` // P8: new groups default true in UI, legacy false preserved
}

func compileDNSGroups(cfg Config, upstreams map[string]Upstream) (map[string]DNSGroup, error) {
	if len(cfg.Groups) > maxDNSGroups {
		return nil, fmt.Errorf("too many DNS groups: %d > %d", len(cfg.Groups), maxDNSGroups)
	}
	groups := make(map[string]DNSGroup, len(cfg.Groups))
	for _, g := range cfg.Groups {
		if !validID(g.ID) || len(g.ID) > 40 {
			return nil, fmt.Errorf("invalid DNS group id %q", g.ID)
		}
		if _, exists := groups[g.ID]; exists {
			return nil, fmt.Errorf("duplicate DNS group %q", g.ID)
		}
		if !utf8.ValidString(g.Name) || strings.TrimSpace(g.Name) == "" || len([]rune(g.Name)) > 80 {
			return nil, fmt.Errorf("invalid DNS group name %q", g.ID)
		}
		if len(g.Servers) < 1 || len(g.Servers) > maxDNSGroupServers {
			return nil, fmt.Errorf("DNS group %q must contain 1..%d servers", g.ID, maxDNSGroupServers)
		}
		seen := make(map[string]bool, len(g.Servers))
		for _, id := range g.Servers {
			u, ok := upstreams[id]
			if !ok {
				return nil, fmt.Errorf("DNS group %q references unknown upstream %q", g.ID, id)
			}
			if g.EncryptedOnly && u.Transport != "dot" && u.Transport != "doh" && u.Transport != "doq" {
				return nil, fmt.Errorf("encrypted-only DNS group %q cannot contain plaintext upstream %q", g.ID, id)
			}
			if seen[id] {
				return nil, fmt.Errorf("DNS group %q repeats upstream %q", g.ID, id)
			}
			seen[id] = true
		}
		groups[g.ID] = g
	}
	return groups, nil
}

// UpstreamChainForDecision is the sole forwarding entry point for policy matches.
// In group mode it deliberately ignores each member's legacy fallback list.
func (e *Engine) UpstreamChainForDecision(d Decision) ([]Upstream, error) {
	if e == nil || !d.Matched || d.Source != "policy" {
		return nil, fmt.Errorf("no explicit DNS policy match")
	}
	if d.GroupID == "" {
		return e.UpstreamChain(d.Upstream)
	} // backward-compatible legacy rule
	g, ok := e.groups[d.GroupID]
	if !ok || len(g.Servers) == 0 || g.Servers[0] != d.Upstream {
		return nil, fmt.Errorf("invalid DNS group decision %q", d.GroupID)
	}
	result := make([]Upstream, 0, len(g.Servers))
	for _, id := range g.Servers {
		u, found := e.upstreams[id]
		if !found {
			return nil, fmt.Errorf("DNS group %q has missing upstream %q", d.GroupID, id)
		}
		result = append(result, u)
	}
	return result, nil
}
