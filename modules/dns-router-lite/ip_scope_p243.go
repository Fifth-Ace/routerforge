package lite

// P24.3 compiles bounded, read-only domain/list selectors at engine creation.
// Per-query checks never rescan 10k domain rules and never change DNS routing.
import (
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"strings"
)

type ipScopeMatcher struct {
	all        bool
	exact      map[string]struct{}
	suffix     map[string]struct{}
	subdomains map[string]struct{}
}

func newIPScopeMatcher(c dr.Config) *ipScopeMatcher {
	m := &ipScopeMatcher{exact: make(map[string]struct{}), suffix: make(map[string]struct{}), subdomains: make(map[string]struct{})}
	if c.Service == nil || c.Service.IPIntelligence == nil {
		m.all = true
		return m
	}
	cfg := c.Service.IPIntelligence
	if cfg.Scope == "" || cfg.Scope == "all" {
		m.all = true
		return m
	}
	if cfg.Scope == "domains" {
		for _, d := range cfg.Domains {
			m.suffix[strings.TrimSuffix(strings.ToLower(d), ".")] = struct{}{}
		}
		return m
	}
	if cfg.Scope == "lists" {
		selected := make(map[string]struct{}, len(cfg.ListIDs))
		for _, id := range cfg.ListIDs {
			selected[id] = struct{}{}
		}
		for _, r := range c.Rules {
			if _, ok := selected[r.ListID]; !ok || r.ListID == "" {
				continue
			}
			domain := strings.TrimSuffix(strings.ToLower(r.Domain), ".")
			switch r.Match {
			case "exact":
				m.exact[domain] = struct{}{}
			case "suffix":
				m.suffix[domain] = struct{}{}
			case "subdomains":
				m.subdomains[domain] = struct{}{}
			}
		}
	}
	return m
}

func (m *ipScopeMatcher) Matches(qname string) bool {
	if m == nil || m.all {
		return true
	}
	name := strings.TrimSuffix(strings.ToLower(qname), ".")
	if name == "" {
		return false
	}
	if _, ok := m.exact[name]; ok {
		return true
	}
	if _, ok := m.suffix[name]; ok {
		return true
	}
	for suffix := name; ; {
		dot := strings.IndexByte(suffix, '.')
		if dot < 0 {
			break
		}
		suffix = suffix[dot+1:]
		if _, ok := m.suffix[suffix]; ok {
			return true
		}
		if _, ok := m.subdomains[suffix]; ok {
			return true
		}
	}
	return false
}
