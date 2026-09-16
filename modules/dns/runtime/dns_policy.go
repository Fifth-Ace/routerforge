package main

import (
	"context"
	"fmt"
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
