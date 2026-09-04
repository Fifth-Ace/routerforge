package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func readDNSPolicyNames() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	client := newDNSRCIClient("http://127.0.0.1:79/rci")
	var payload any
	if err := client.getJSON(ctx, "/show/ip/policy", &payload); err != nil {
		return nil
	}
	out := map[string]string{"System": "System"}
	collectDNSPolicyNames(payload, "", out)
	return out
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
