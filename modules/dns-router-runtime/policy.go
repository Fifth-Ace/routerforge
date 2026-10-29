// Package dnsrouter implements deterministic, offline DNS-upstream selection.
// It neither listens on port 53 nor changes Keenetic, firewall, or network state.
package dnsrouter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
)

const SchemaVersion = 1
const MaxRules = 10000
const MaxUpstreams = 128

type Config struct {
	SchemaVersion int             `json:"schema_version"`
	Upstreams     []Upstream      `json:"upstreams"`
	Rules         []Rule          `json:"rules"`
	Lists         []DomainList    `json:"lists,omitempty"`
	Listener      *ListenerConfig `json:"listener,omitempty"`
}

type Upstream struct {
	ID         string   `json:"id"`
	Transport  string   `json:"transport"`             // udp, tcp, dot, doh
	Address    string   `json:"address"`               // explicit unicast IP:port to prevent bootstrap loops
	ServerName string   `json:"server_name,omitempty"` // required TLS SNI and certificate name for dot/doh
	Path       string   `json:"path,omitempty"`        // DoH endpoint, defaults to /dns-query
	Fallbacks  []string `json:"fallbacks,omitempty"`   // explicit same-rule failover, never default DNS
}

type Rule struct {
	ID       string `json:"id"`
	Domain   string `json:"domain"`
	Match    string `json:"match"` // exact, suffix, subdomains
	Upstream string `json:"upstream"`
	Priority int    `json:"priority"`
	ListID   string `json:"list_id,omitempty"`
}

type Decision struct {
	Domain    string `json:"domain"`
	Matched   bool   `json:"matched"`
	Source    string `json:"source"` // policy or system; no transparent fallback
	RuleID    string `json:"rule_id,omitempty"`
	Upstream  string `json:"upstream,omitempty"`
	Transport string `json:"transport,omitempty"`
	Address   string `json:"address,omitempty"`
	Reason    string `json:"reason"`
}

type node struct {
	child map[string]*node
	rules []Rule
}

type Engine struct {
	root      *node
	upstreams map[string]Upstream
	ruleCount int
}

var protected = []string{"localhost", "local", "lan", "home.arpa", "in-addr.arpa", "ip6.arpa"}

func normalizedDomain(raw string) (string, error) {
	s := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	if len(s) < 1 || len(s) > 253 {
		return "", fmt.Errorf("invalid DNS name length")
	}
	for _, label := range strings.Split(s, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid DNS label in %q", raw)
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
				return "", fmt.Errorf("invalid DNS character in %q; use IDNA ASCII", raw)
			}
		}
	}
	return s, nil
}

func isProtected(domain string) bool {
	for _, suffix := range protected {
		if domain == suffix || strings.HasSuffix(domain, "."+suffix) {
			return true
		}
	}
	return false
}

func validID(s string) bool {
	if len(s) < 1 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// Compile validates the entire config before it can be evaluated. Complexity:
// O(number of labels) per query, independent of the number of stored rules.
func Compile(cfg Config) (*Engine, error) {
	if err := ValidateListenerConfigL(cfg.Listener); err != nil {
		return nil, err
	}
	if cfg.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported schema_version %d", cfg.SchemaVersion)
	}
	if err := validateManagedLists(cfg); err != nil {
		return nil, err
	}
	if len(cfg.Upstreams) > MaxUpstreams {
		return nil, fmt.Errorf("too many upstreams: %d > %d", len(cfg.Upstreams), MaxUpstreams)
	}
	if len(cfg.Rules) > MaxRules {
		return nil, fmt.Errorf("too many rules: %d > %d", len(cfg.Rules), MaxRules)
	}
	e := &Engine{root: &node{child: map[string]*node{}}, upstreams: make(map[string]Upstream), ruleCount: len(cfg.Rules)}
	for _, u := range cfg.Upstreams {
		if !validID(u.ID) {
			return nil, fmt.Errorf("invalid upstream id %q", u.ID)
		}
		if _, dup := e.upstreams[u.ID]; dup {
			return nil, fmt.Errorf("duplicate upstream %q", u.ID)
		}
		if u.Transport != "udp" && u.Transport != "tcp" && u.Transport != "dot" && u.Transport != "doh" {
			return nil, fmt.Errorf("invalid transport for %q", u.ID)
		}
		host, port, err := net.SplitHostPort(u.Address)
		if err != nil {
			return nil, fmt.Errorf("upstream %q: %w", u.ID, err)
		}
		ip := net.ParseIP(host)
		if ip == nil || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return nil, fmt.Errorf("upstream %q requires a non-loopback unicast IP", u.ID)
		}
		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("upstream %q has invalid port", u.ID)
		}
		if u.Transport == "dot" || u.Transport == "doh" {
			normalized, err := normalizedDomain(u.ServerName)
			if err != nil || normalized != u.ServerName {
				return nil, fmt.Errorf("upstream %q requires a lowercase TLS server_name DNS name", u.ID)
			}
			if u.Transport == "doh" {
				if u.Path == "" {
					u.Path = "/dns-query"
				}
				if len(u.Path) > 256 || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") || strings.ContainsAny(u.Path, "?#\\ \t\r\n") || strings.Contains(u.Path, "..") {
					return nil, fmt.Errorf("invalid DoH path for %q", u.ID)
				}
			} else if u.Path != "" {
				return nil, fmt.Errorf("DoT upstream %q must not have HTTP path", u.ID)
			}
		} else if u.ServerName != "" || u.Path != "" {
			return nil, fmt.Errorf("plaintext upstream %q must not set TLS/HTTP options", u.ID)
		}
		if len(u.Fallbacks) > 3 {
			return nil, fmt.Errorf("upstream %q has more than 3 fallbacks", u.ID)
		}
		seenFallback := map[string]bool{}
		for _, fallback := range u.Fallbacks {
			if !validID(fallback) || fallback == u.ID || seenFallback[fallback] {
				return nil, fmt.Errorf("upstream %q has invalid/duplicate fallback", u.ID)
			}
			seenFallback[fallback] = true
		}
		e.upstreams[u.ID] = u
	}
	for _, u := range e.upstreams {
		for _, id := range u.Fallbacks {
			fallback, ok := e.upstreams[id]
			if !ok {
				return nil, fmt.Errorf("upstream %q references unknown fallback %q", u.ID, id)
			}
			if len(fallback.Fallbacks) != 0 {
				return nil, fmt.Errorf("upstream %q fallback %q must be terminal", u.ID, id)
			}
		}
	}
	ruleIDs := make(map[string]bool)
	for _, raw := range cfg.Rules {
		if !validID(raw.ID) || ruleIDs[raw.ID] {
			return nil, fmt.Errorf("invalid/duplicate rule id %q", raw.ID)
		}
		ruleIDs[raw.ID] = true
		domain, err := normalizedDomain(raw.Domain)
		if err != nil {
			return nil, err
		}
		if isProtected(domain) {
			return nil, fmt.Errorf("rule %q overlaps protected local namespace %q", raw.ID, domain)
		}
		if raw.Match != "exact" && raw.Match != "suffix" && raw.Match != "subdomains" {
			return nil, fmt.Errorf("invalid match mode in %q", raw.ID)
		}
		if raw.Priority < 0 || raw.Priority > 1000000 {
			return nil, fmt.Errorf("priority out of range for %q", raw.ID)
		}
		if _, ok := e.upstreams[raw.Upstream]; !ok {
			return nil, fmt.Errorf("rule %q: unknown upstream %q", raw.ID, raw.Upstream)
		}
		raw.Domain = domain
		current := e.root
		labels := strings.Split(domain, ".")
		for i := len(labels) - 1; i >= 0; i-- {
			if current.child[labels[i]] == nil {
				current.child[labels[i]] = &node{child: map[string]*node{}}
			}
			current = current.child[labels[i]]
		}
		current.rules = append(current.rules, raw)
	}
	return e, nil
}

// Evaluate never sends DNS packets. For unmatched/local names it explicitly
// returns Source=system, allowing a future adapter to preserve Keenetic DNS.
func (e *Engine) Evaluate(query string) (Decision, error) {
	if e == nil || e.root == nil {
		return Decision{}, errors.New("policy engine is not initialized")
	}
	d, err := normalizedDomain(query)
	if err != nil {
		return Decision{}, err
	}
	result := Decision{Domain: d, Source: "system", Reason: "no rule: keep Keenetic DNS"}
	if isProtected(d) {
		result.Reason = "protected local namespace: keep Keenetic DNS"
		return result, nil
	}
	current := e.root
	labels := strings.Split(d, ".")
	var best *Rule
	bestDepth, bestRank := -1, -1
	for i := len(labels) - 1; i >= 0; i-- {
		current = current.child[labels[i]]
		if current == nil {
			break
		}
		depth := len(labels) - i
		for _, candidate := range current.rules {
			isExact := depth == len(labels)
			eligible := candidate.Match == "suffix" || (candidate.Match == "exact" && isExact) || (candidate.Match == "subdomains" && !isExact)
			if !eligible {
				continue
			}
			rank := 0
			if candidate.Match == "exact" {
				rank = 2
			} else if candidate.Match == "suffix" {
				rank = 1
			}
			if best == nil || depth > bestDepth || (depth == bestDepth && (rank > bestRank || (rank == bestRank && (candidate.Priority < best.Priority || (candidate.Priority == best.Priority && candidate.ID < best.ID))))) {
				c := candidate
				best = &c
				bestDepth, bestRank = depth, rank
			}
		}
	}
	if best == nil {
		return result, nil
	}
	u := e.upstreams[best.Upstream]
	return Decision{Domain: d, Matched: true, Source: "policy", RuleID: best.ID, Upstream: u.ID, Transport: u.Transport, Address: u.Address, Reason: "most-specific matching domain rule"}, nil
}

// UpstreamChain yields only the explicitly selected upstream and its named
// fallbacks. No default resolvers or Keenetic DNS path are ever consulted.
func (e *Engine) UpstreamChain(id string) ([]Upstream, error) {
	primary, ok := e.upstreams[id]
	if !ok {
		return nil, fmt.Errorf("unknown upstream %q", id)
	}
	chain := []Upstream{primary}
	for _, fallback := range primary.Fallbacks {
		chain = append(chain, e.upstreams[fallback])
	}
	return chain, nil
}

func Load(path string) (*Engine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	// A second JSON value is not permitted.
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return nil, errors.New("trailing JSON object")
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("trailing JSON data: %w", err)
	}
	return Compile(cfg)
}

func (e *Engine) RuleCount() int {
	if e == nil {
		return 0
	}
	return e.ruleCount
}
func (e *Engine) UpstreamIDs() []string {
	var ids []string
	for id := range e.upstreams {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
