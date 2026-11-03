package dnsrouter

// P24.3: explicit selection of IP Intelligence domains or managed domain lists.
// Empty scope preserves P24.1/P24.2 global behavior for existing opt-in users.
import (
	"fmt"
	"strings"
)

func validateIPIntelligenceScope(c Config) error {
	if c.Service == nil || c.Service.IPIntelligence == nil {
		return nil
	}
	cfg := c.Service.IPIntelligence
	if len(cfg.Domains) > 128 || len(cfg.ListIDs) > 64 {
		return fmt.Errorf("IP Intelligence scope too large")
	}
	switch cfg.Scope {
	case "", "all":
		if len(cfg.Domains) != 0 || len(cfg.ListIDs) != 0 {
			return fmt.Errorf("global IP scope must not have filters")
		}
	case "domains":
		if len(cfg.Domains) == 0 || len(cfg.ListIDs) != 0 {
			return fmt.Errorf("domain scope requires domains only")
		}
		seen := make(map[string]bool, len(cfg.Domains))
		for _, domain := range cfg.Domains {
			canonical, err := normalizedDomain(domain)
			if err != nil || domain != canonical || isProtected(canonical) {
				return fmt.Errorf("invalid IP scope domain %q", domain)
			}
			if seen[domain] {
				return fmt.Errorf("duplicate IP scope domain %q", domain)
			}
			seen[domain] = true
		}
	case "lists":
		if len(cfg.ListIDs) == 0 || len(cfg.Domains) != 0 {
			return fmt.Errorf("list scope requires list IDs only")
		}
		known := make(map[string]bool, len(c.Lists))
		for _, list := range c.Lists {
			known[list.ID] = true
		}
		seen := make(map[string]bool, len(cfg.ListIDs))
		for _, id := range cfg.ListIDs {
			if !known[id] || seen[id] || strings.TrimSpace(id) != id {
				return fmt.Errorf("missing/duplicate IP scope list %q", id)
			}
			seen[id] = true
		}
	default:
		return fmt.Errorf("unsupported IP Intelligence scope %q", cfg.Scope)
	}
	return nil
}
