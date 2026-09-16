package main

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DNSPolicyOption struct {
	Proxy       string `json:"proxy"`
	DisplayName string `json:"display_name"`
	System      bool   `json:"system"`
	Ordinal     int    `json:"ordinal,omitempty"`
}

func readDNSPolicyInventory() ([]DNSPolicyOption, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	client := newDNSRCIClient("http://127.0.0.1:79/rci")
	var payload any
	if err := client.getJSON(ctx, "/show/ip/policy", &payload); err != nil {
		return nil, err
	}
	names := map[string]string{"System": "System"}
	collectDNSPolicyNames(payload, "", names)
	return dnsPolicyInventoryFromNames(names), nil
}

func readDNSPolicyNames() map[string]string {
	inventory, err := readDNSPolicyInventory()
	if err != nil {
		return nil
	}
	out := make(map[string]string, len(inventory))
	for _, item := range inventory {
		out[item.Proxy] = item.DisplayName
	}
	return out
}

func dnsPolicyInventoryFromNames(names map[string]string) []DNSPolicyOption {
	out := make([]DNSPolicyOption, 0, len(names))
	for proxy, display := range names {
		proxy = normalizePolicyProxyName(proxy)
		display = strings.TrimSpace(display)
		if display == "" {
			display = proxy
		}
		item := DNSPolicyOption{
			Proxy:       proxy,
			DisplayName: display,
			System:      strings.EqualFold(proxy, "System"),
		}
		if ordinal, ok := policyProxyOrdinal(proxy); ok {
			item.Ordinal = ordinal
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].System != out[j].System {
			return out[i].System
		}
		oi, iok := policyProxyOrdinal(out[i].Proxy)
		oj, jok := policyProxyOrdinal(out[j].Proxy)
		if iok && jok && oi != oj {
			return oi < oj
		}
		if iok != jok {
			return iok
		}
		return out[i].Proxy < out[j].Proxy
	})
	return out
}

func policyProxyOrdinal(value string) (int, bool) {
	value = normalizePolicyProxyName(value)
	if !isPolicyProxyKey(value) {
		return 0, false
	}
	ordinal, err := strconv.Atoi(strings.TrimSpace(value[len("Policy"):]))
	if err != nil {
		return 0, false
	}
	return ordinal, true
}

type DNSPolicyMatch struct {
	ClientCIDR   string `json:"client_cidr,omitempty"`
	DomainSuffix string `json:"domain_suffix,omitempty"`
	QueryType    string `json:"qtype,omitempty"`
}

type DNSPolicyRule struct {
	ID       string         `json:"id"`
	Priority int            `json:"priority"`
	Policy   string         `json:"policy"`
	Match    DNSPolicyMatch `json:"match"`
}

type DNSPolicyEvaluationRequest struct {
	ClientIP  string          `json:"client_ip,omitempty"`
	Domain    string          `json:"domain"`
	QueryType string          `json:"qtype,omitempty"`
	Rules     []DNSPolicyRule `json:"rules"`
}

type DNSPolicyEvaluation struct {
	Matched          bool     `json:"matched"`
	Policy           string   `json:"policy"`
	RuleID           string   `json:"rule_id,omitempty"`
	RulePriority     int      `json:"rule_priority,omitempty"`
	Specificity      int      `json:"specificity,omitempty"`
	Reasons          []string `json:"reasons"`
	EvaluatedRules   int      `json:"evaluated_rules"`
	FallbackToSystem bool     `json:"fallback_to_system"`
}

type dnsPolicyCandidate struct {
	rule        DNSPolicyRule
	specificity int
	reasons     []string
}

func evaluateDNSPolicy(req DNSPolicyEvaluationRequest, allowed map[string]bool) (DNSPolicyEvaluation, error) {
	req.Domain = normalizeDNSPolicyDomain(req.Domain)
	if req.Domain == "" {
		return DNSPolicyEvaluation{}, fmt.Errorf("domain is required")
	}
	if len(req.Domain) > 253 {
		return DNSPolicyEvaluation{}, fmt.Errorf("domain is too long")
	}
	req.QueryType = strings.ToUpper(strings.TrimSpace(req.QueryType))
	if req.QueryType != "" && !validDNSPolicyQueryType(req.QueryType) {
		return DNSPolicyEvaluation{}, fmt.Errorf("unsupported qtype %q", req.QueryType)
	}
	var client net.IP
	if strings.TrimSpace(req.ClientIP) != "" {
		client = net.ParseIP(strings.TrimSpace(req.ClientIP))
		if client == nil {
			return DNSPolicyEvaluation{}, fmt.Errorf("invalid client_ip")
		}
	}
	if len(req.Rules) > 128 {
		return DNSPolicyEvaluation{}, fmt.Errorf("too many rules")
	}

	candidates := make([]dnsPolicyCandidate, 0, len(req.Rules))
	seenIDs := make(map[string]bool, len(req.Rules))
	for _, raw := range req.Rules {
		rule, err := normalizeDNSPolicyRule(raw)
		if err != nil {
			return DNSPolicyEvaluation{}, err
		}
		if seenIDs[rule.ID] {
			return DNSPolicyEvaluation{}, fmt.Errorf("duplicate rule id %q", rule.ID)
		}
		seenIDs[rule.ID] = true
		if allowed != nil && !allowed[rule.Policy] {
			return DNSPolicyEvaluation{}, fmt.Errorf("unknown policy %q", rule.Policy)
		}
		candidate, matched, err := matchDNSPolicyRule(rule, client, req.Domain, req.QueryType)
		if err != nil {
			return DNSPolicyEvaluation{}, err
		}
		if matched {
			candidates = append(candidates, candidate)
		}
	}

	if len(candidates) == 0 {
		return DNSPolicyEvaluation{
			Matched:          false,
			Policy:           "System",
			Reasons:          []string{"no candidate rule matched; using System"},
			EvaluatedRules:   len(req.Rules),
			FallbackToSystem: true,
		}, nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].specificity != candidates[j].specificity {
			return candidates[i].specificity > candidates[j].specificity
		}
		if candidates[i].rule.Priority != candidates[j].rule.Priority {
			return candidates[i].rule.Priority < candidates[j].rule.Priority
		}
		return candidates[i].rule.ID < candidates[j].rule.ID
	})
	winner := candidates[0]
	return DNSPolicyEvaluation{
		Matched:          true,
		Policy:           winner.rule.Policy,
		RuleID:           winner.rule.ID,
		RulePriority:     winner.rule.Priority,
		Specificity:      winner.specificity,
		Reasons:          winner.reasons,
		EvaluatedRules:   len(req.Rules),
		FallbackToSystem: false,
	}, nil
}

func validateDNSPolicyRules(rules []DNSPolicyRule, allowed map[string]bool) ([]DNSPolicyRule, error) {
	if len(rules) > 128 {
		return nil, fmt.Errorf("too many rules")
	}
	normalized := make([]DNSPolicyRule, 0, len(rules))
	seenIDs := make(map[string]bool, len(rules))
	for _, raw := range rules {
		rule, err := normalizeDNSPolicyRule(raw)
		if err != nil {
			return nil, err
		}
		if seenIDs[rule.ID] {
			return nil, fmt.Errorf("duplicate rule id %q", rule.ID)
		}
		seenIDs[rule.ID] = true
		if allowed != nil && !allowed[rule.Policy] {
			return nil, fmt.Errorf("unknown policy %q", rule.Policy)
		}
		normalized = append(normalized, rule)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].Priority != normalized[j].Priority {
			return normalized[i].Priority < normalized[j].Priority
		}
		return normalized[i].ID < normalized[j].ID
	})
	return normalized, nil
}

func normalizeDNSPolicyRule(rule DNSPolicyRule) (DNSPolicyRule, error) {
	rule.ID = strings.TrimSpace(rule.ID)
	if rule.ID == "" || len(rule.ID) > 64 {
		return DNSPolicyRule{}, fmt.Errorf("invalid rule id")
	}
	if rule.Priority < 0 || rule.Priority > 1000000 {
		return DNSPolicyRule{}, fmt.Errorf("invalid priority for rule %q", rule.ID)
	}
	rule.Policy = normalizePolicyProxyName(rule.Policy)
	if !strings.EqualFold(rule.Policy, "System") && !isPolicyProxyKey(rule.Policy) {
		return DNSPolicyRule{}, fmt.Errorf("invalid policy %q", rule.Policy)
	}
	if strings.EqualFold(rule.Policy, "System") {
		rule.Policy = "System"
	}
	rule.Match.ClientCIDR = strings.TrimSpace(rule.Match.ClientCIDR)
	rule.Match.DomainSuffix = normalizeDNSPolicyDomain(rule.Match.DomainSuffix)
	rule.Match.QueryType = strings.ToUpper(strings.TrimSpace(rule.Match.QueryType))
	if rule.Match.ClientCIDR != "" {
		if _, _, err := net.ParseCIDR(rule.Match.ClientCIDR); err != nil {
			return DNSPolicyRule{}, fmt.Errorf("invalid client_cidr for rule %q", rule.ID)
		}
	}
	if rule.Match.QueryType != "" && !validDNSPolicyQueryType(rule.Match.QueryType) {
		return DNSPolicyRule{}, fmt.Errorf("unsupported qtype for rule %q", rule.ID)
	}
	return rule, nil
}

func matchDNSPolicyRule(rule DNSPolicyRule, client net.IP, domain, qtype string) (dnsPolicyCandidate, bool, error) {
	specificity := 0
	reasons := make([]string, 0, 3)
	if rule.Match.ClientCIDR != "" {
		_, network, err := net.ParseCIDR(rule.Match.ClientCIDR)
		if err != nil {
			return dnsPolicyCandidate{}, false, err
		}
		if client == nil || !network.Contains(client) {
			return dnsPolicyCandidate{}, false, nil
		}
		specificity += 4
		reasons = append(reasons, "client_ip matched "+rule.Match.ClientCIDR)
	}
	if rule.Match.DomainSuffix != "" {
		if domain != rule.Match.DomainSuffix && !strings.HasSuffix(domain, "."+rule.Match.DomainSuffix) {
			return dnsPolicyCandidate{}, false, nil
		}
		specificity += 2
		reasons = append(reasons, "domain matched *."+rule.Match.DomainSuffix)
	}
	if rule.Match.QueryType != "" {
		if qtype == "" || qtype != rule.Match.QueryType {
			return dnsPolicyCandidate{}, false, nil
		}
		specificity++
		reasons = append(reasons, "qtype matched "+rule.Match.QueryType)
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "catch-all rule matched")
	}
	return dnsPolicyCandidate{rule: rule, specificity: specificity, reasons: reasons}, true, nil
}

func normalizeDNSPolicyDomain(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, ".")
	return value
}

func validDNSPolicyQueryType(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "A", "AAAA", "CNAME", "MX", "NS", "PTR", "SOA", "SRV", "TXT", "HTTPS", "SVCB":
		return true
	default:
		return false
	}
}

func collectDNSPolicyNames(value any, keyHint string, out map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		proxyName := ""
		for _, key := range []string{"proxy", "proxy-name", "policy", "id"} {
			if raw, ok := typed[key]; ok {
				candidate := strings.TrimSpace(fmt.Sprint(raw))
				if strings.HasPrefix(strings.ToLower(candidate), "policy") {
					proxyName = normalizePolicyProxyName(candidate)
					break
				}
			}
		}
		if proxyName == "" {
			if raw, ok := typed["index"]; ok {
				candidate := strings.TrimSpace(fmt.Sprint(raw))
				if strings.HasSuffix(candidate, ".0") {
					candidate = strings.TrimSuffix(candidate, ".0")
				}
				if candidate != "" {
					proxyName = "Policy" + candidate
				}
			}
		}
		if proxyName == "" && isPolicyProxyKey(keyHint) {
			proxyName = normalizePolicyProxyName(keyHint)
		}
		if proxyName != "" {
			display := ""
			for _, key := range []string{"description", "title", "comment", "name"} {
				if raw, ok := typed[key]; ok {
					candidate := strings.TrimSpace(fmt.Sprint(raw))
					if candidate != "" && !strings.EqualFold(candidate, proxyName) {
						display = candidate
						break
					}
				}
			}
			if display != "" {
				out[proxyName] = display
			}
		}
		for key, child := range typed {
			collectDNSPolicyNames(child, key, out)
		}
	case []any:
		for _, child := range typed {
			collectDNSPolicyNames(child, keyHint, out)
		}
	}
}

func normalizePolicyProxyName(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if !strings.HasPrefix(lower, "policy") {
		return value
	}
	suffix := strings.TrimSpace(value[len("Policy"):])
	return "Policy" + suffix
}

func isPolicyProxyKey(value string) bool {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if !strings.HasPrefix(lower, "policy") {
		return false
	}
	suffix := strings.TrimSpace(value[len("Policy"):])
	if suffix == "" {
		return false
	}
	for _, ch := range suffix {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
