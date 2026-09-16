package main

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

type routeCapabilities struct {
	IPCommand     bool `json:"ip_command"`
	IPv4AllTables bool `json:"ipv4_all_tables"`
	IPv6AllTables bool `json:"ipv6_all_tables"`
	IPv4Rules     bool `json:"ipv4_rules"`
	IPv6Rules     bool `json:"ipv6_rules"`
	RouteGet      bool `json:"route_get"`
}

type routeRecord struct {
	Family      string `json:"family"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Interface   string `json:"interface,omitempty"`
	Source      string `json:"source,omitempty"`
	Metric      int    `json:"metric,omitempty"`
	Table       string `json:"table"`
	Type        string `json:"type"`
	Scope       string `json:"scope,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	Preference  string `json:"preference,omitempty"`
	Raw         string `json:"raw,omitempty"`
}

type policyRule struct {
	Family   string `json:"family"`
	Priority int    `json:"priority"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Table    string `json:"table,omitempty"`
	Mark     string `json:"mark,omitempty"`
	IIF      string `json:"iif,omitempty"`
	OIF      string `json:"oif,omitempty"`
	Action   string `json:"action,omitempty"`
	Raw      string `json:"raw"`
}

type kernelRouteDecision struct {
	Available   bool   `json:"available"`
	Family      string `json:"family,omitempty"`
	Destination string `json:"destination,omitempty"`
	Gateway     string `json:"gateway,omitempty"`
	Interface   string `json:"interface,omitempty"`
	Source      string `json:"source,omitempty"`
	Table       string `json:"table,omitempty"`
	Metric      int    `json:"metric,omitempty"`
	Type        string `json:"type,omitempty"`
	Raw         string `json:"raw,omitempty"`
	Error       string `json:"error,omitempty"`
}

func runIPCommand(parent context.Context, args ...string) (string, error) {
	binary, err := exec.LookPath("ip")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	output, runErr := safety.RunCommand(ctx, commandOutputMax, binary, args...)
	text := strings.TrimSpace(string(output))
	if ctx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("ip command timed out")
	}
	if runErr != nil {
		if text != "" {
			return text, fmt.Errorf("%v: %s", runErr, text)
		}
		return text, runErr
	}
	return text, nil
}

func parseRouteRecord(line, family string) (routeRecord, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return routeRecord{}, false
	}
	record := routeRecord{Family: family, Table: "main", Type: "unicast", Raw: strings.TrimSpace(line)}
	idx := 0
	switch fields[0] {
	case "unreachable", "blackhole", "prohibit", "throw", "local", "broadcast", "multicast", "nat", "anycast":
		record.Type = fields[0]
		idx++
	}
	if idx >= len(fields) {
		return routeRecord{}, false
	}
	record.Destination = fields[idx]
	idx++
	for idx < len(fields) {
		key := fields[idx]
		switch key {
		case "via":
			if idx+1 < len(fields) {
				record.Gateway = fields[idx+1]
				idx += 2
				continue
			}
		case "dev":
			if idx+1 < len(fields) {
				record.Interface = fields[idx+1]
				idx += 2
				continue
			}
		case "src":
			if idx+1 < len(fields) {
				record.Source = fields[idx+1]
				idx += 2
				continue
			}
		case "metric":
			if idx+1 < len(fields) {
				record.Metric, _ = strconv.Atoi(fields[idx+1])
				idx += 2
				continue
			}
		case "table":
			if idx+1 < len(fields) {
				record.Table = fields[idx+1]
				idx += 2
				continue
			}
		case "scope":
			if idx+1 < len(fields) {
				record.Scope = fields[idx+1]
				idx += 2
				continue
			}
		case "proto":
			if idx+1 < len(fields) {
				record.Protocol = fields[idx+1]
				idx += 2
				continue
			}
		case "pref":
			if idx+1 < len(fields) {
				record.Preference = fields[idx+1]
				idx += 2
				continue
			}
		}
		idx++
	}
	return record, record.Destination != ""
}

func parseRouteOutput(raw, family string) []routeRecord {
	out := make([]routeRecord, 0, 32)
	for _, line := range strings.Split(raw, "\n") {
		if len(out) >= 512 {
			break
		}
		if record, ok := parseRouteRecord(line, family); ok {
			out = append(out, record)
		}
	}
	return out
}

func parsePolicyRule(line, family string) (policyRule, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 2 {
		return policyRule{}, false
	}
	priority, err := strconv.Atoi(strings.TrimSuffix(fields[0], ":"))
	if err != nil {
		return policyRule{}, false
	}
	rule := policyRule{Family: family, Priority: priority, Raw: strings.TrimSpace(line)}
	for i := 1; i < len(fields); i++ {
		switch fields[i] {
		case "from":
			if i+1 < len(fields) {
				rule.From = fields[i+1]
				i++
			}
		case "to":
			if i+1 < len(fields) {
				rule.To = fields[i+1]
				i++
			}
		case "lookup", "table":
			if i+1 < len(fields) {
				rule.Table = fields[i+1]
				i++
			}
		case "fwmark":
			if i+1 < len(fields) {
				rule.Mark = fields[i+1]
				i++
			}
		case "iif":
			if i+1 < len(fields) {
				rule.IIF = fields[i+1]
				i++
			}
		case "oif":
			if i+1 < len(fields) {
				rule.OIF = fields[i+1]
				i++
			}
		case "blackhole", "unreachable", "prohibit", "goto":
			rule.Action = fields[i]
			if fields[i] == "goto" && i+1 < len(fields) {
				rule.Action += " " + fields[i+1]
				i++
			}
		}
	}
	if rule.From == "" {
		rule.From = "all"
	}
	return rule, true
}

func parsePolicyOutput(raw, family string) []policyRule {
	out := make([]policyRule, 0, 16)
	for _, line := range strings.Split(raw, "\n") {
		if len(out) >= 256 {
			break
		}
		if rule, ok := parsePolicyRule(line, family); ok {
			out = append(out, rule)
		}
	}
	return out
}

func parseRouteGet(raw, family string) kernelRouteDecision {
	line := ""
	for _, candidate := range strings.Split(raw, "\n") {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			line = candidate
			break
		}
	}
	if line == "" {
		return kernelRouteDecision{Available: false, Family: family, Error: "empty route-get output"}
	}
	fields := strings.Fields(line)
	decision := kernelRouteDecision{Available: true, Family: family, Table: "main", Type: "unicast", Raw: line}
	idx := 0
	if fields[0] == "unreachable" || fields[0] == "blackhole" || fields[0] == "prohibit" || fields[0] == "throw" {
		decision.Type = fields[0]
		idx++
	}
	if idx < len(fields) {
		decision.Destination = fields[idx]
		idx++
	}
	for idx < len(fields) {
		switch fields[idx] {
		case "via":
			if idx+1 < len(fields) {
				decision.Gateway = fields[idx+1]
				idx += 2
				continue
			}
		case "dev":
			if idx+1 < len(fields) {
				decision.Interface = fields[idx+1]
				idx += 2
				continue
			}
		case "src":
			if idx+1 < len(fields) {
				decision.Source = fields[idx+1]
				idx += 2
				continue
			}
		case "table":
			if idx+1 < len(fields) {
				decision.Table = fields[idx+1]
				idx += 2
				continue
			}
		case "metric":
			if idx+1 < len(fields) {
				decision.Metric, _ = strconv.Atoi(fields[idx+1])
				idx += 2
				continue
			}
		}
		idx++
	}
	return decision
}

func resolveRouteTarget(parent context.Context, target string) (net.IP, map[string]any) {
	if ip := net.ParseIP(target); ip != nil {
		family := "ipv6"
		if ip.To4() != nil {
			family = "ipv4"
		}
		return ip, map[string]any{"ok": true, "addresses": []string{ip.String()}, "family": family}
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", target)
	if err != nil {
		return nil, map[string]any{"ok": false, "addresses": []string{}, "error": err.Error()}
	}
	stringsOut := make([]string, 0, len(addresses))
	var first4, first6 net.IP
	for _, ip := range addresses {
		stringsOut = append(stringsOut, ip.String())
		if ip.To4() != nil && first4 == nil {
			first4 = ip
		}
		if ip.To4() == nil && first6 == nil {
			first6 = ip
		}
		if len(stringsOut) >= 16 {
			break
		}
	}
	selected := first4
	if selected == nil {
		selected = first6
	}
	family := ""
	if selected != nil {
		family = "ipv6"
		if selected.To4() != nil {
			family = "ipv4"
		}
	}
	return selected, map[string]any{"ok": selected != nil, "addresses": stringsOut, "family": family}
}

func legacyRouteRecords(routes []routeEntry) []routeRecord {
	out := make([]routeRecord, 0, len(routes))
	for _, route := range routes {
		destination := fmt.Sprintf("%s/%d", route.Destination, route.Prefix)
		if route.Prefix == 0 {
			destination = "default"
		}
		out = append(out, routeRecord{Family: "ipv4", Destination: destination, Gateway: route.Gateway, Interface: route.Interface, Metric: route.Metric, Table: route.Table, Type: route.Type, Raw: "procfs"})
	}
	return out
}

func nonDefaultPolicyRules(rules []policyRule) int {
	count := 0
	for _, rule := range rules {
		if rule.Priority == 0 && rule.Table == "local" {
			continue
		}
		if rule.Priority == 32766 && rule.Table == "main" {
			continue
		}
		if rule.Priority == 32767 && rule.Table == "default" {
			continue
		}
		count++
	}
	return count
}

func routeInspector(parent context.Context, target string) map[string]any {
	legacyRoutes, legacyErr := readRoutes()
	caps := routeCapabilities{}
	routes4 := []routeRecord{}
	routes6 := []routeRecord{}
	rules4 := []policyRule{}
	rules6 := []policyRule{}
	source := "procfs-fallback"

	if _, err := exec.LookPath("ip"); err == nil {
		caps.IPCommand = true
		if raw, err := runIPCommand(parent, "-4", "route", "show", "table", "all"); err == nil {
			routes4 = parseRouteOutput(raw, "ipv4")
			caps.IPv4AllTables = true
			source = "ip-command"
		}
		if raw, err := runIPCommand(parent, "-6", "route", "show", "table", "all"); err == nil {
			routes6 = parseRouteOutput(raw, "ipv6")
			caps.IPv6AllTables = true
			source = "ip-command"
		}
		if raw, err := runIPCommand(parent, "-4", "rule", "show"); err == nil {
			rules4 = parsePolicyOutput(raw, "ipv4")
			caps.IPv4Rules = true
		}
		if raw, err := runIPCommand(parent, "-6", "rule", "show"); err == nil {
			rules6 = parsePolicyOutput(raw, "ipv6")
			caps.IPv6Rules = true
		}
	}

	if len(routes4) == 0 && legacyErr == nil {
		routes4 = legacyRouteRecords(legacyRoutes)
	}
	response := map[string]any{
		"routes":            legacyRoutes,
		"routes_v4":         routes4,
		"routes_v6":         routes6,
		"rules_v4":          rules4,
		"rules_v6":          rules6,
		"capabilities":      caps,
		"source":            source,
		"table":             "all",
		"policy_rule_count": len(rules4) + len(rules6),
		"mutation_api":      false,
	}
	if target == "" {
		return response
	}
	response["target"] = target
	if !validTarget(target) {
		response["error"] = "invalid target"
		return response
	}
	ip, resolution := resolveRouteTarget(parent, target)
	response["resolution"] = resolution
	if ip == nil {
		response["policy_state"] = "unavailable"
		return response
	}
	family := "ipv6"
	familyArg := "-6"
	if ip.To4() != nil {
		family = "ipv4"
		familyArg = "-4"
	}
	response["target_address"] = ip.String()
	response["family"] = family

	decision := kernelRouteDecision{Available: false, Family: family}
	if caps.IPCommand {
		if raw, err := runIPCommand(parent, familyArg, "route", "get", ip.String()); err == nil {
			decision = parseRouteGet(raw, family)
			caps.RouteGet = decision.Available
			response["capabilities"] = caps
		} else {
			decision.Error = err.Error()
		}
	}
	if !decision.Available && ip.To4() != nil && legacyErr == nil {
		if selected := selectRoute(legacyRoutes, ip); selected != nil {
			decision = kernelRouteDecision{Available: true, Family: "ipv4", Destination: ip.String(), Gateway: selected.Gateway, Interface: selected.Interface, Table: selected.Table, Metric: selected.Metric, Type: selected.Type, Raw: "procfs longest-prefix fallback"}
		}
	}
	response["kernel_decision"] = decision
	if ip.To4() != nil && legacyErr == nil {
		response["selected"] = selectRoute(legacyRoutes, ip)
	}

	familyRules := rules6
	policyCaps := caps.IPv6Rules
	if family == "ipv4" {
		familyRules = rules4
		policyCaps = caps.IPv4Rules
	}
	if !policyCaps {
		response["policy_state"] = "unavailable"
	} else if nonDefaultPolicyRules(familyRules) > 0 || (decision.Table != "" && decision.Table != "main") {
		response["policy_state"] = "active"
	} else {
		response["policy_state"] = "default-only"
	}
	return response
}
