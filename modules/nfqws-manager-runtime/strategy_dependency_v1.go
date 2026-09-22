package main

import (
	"sort"
	"strings"
)

type v2StrategyExternalDependency struct {
	Kind       string `json:"kind"`
	Token      string `json:"token"`
	Argument   string `json:"argument,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

func v2StrategyDependencySafeToken(token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" || strings.HasPrefix(token, "0x") {
		return true
	}
	switch token {
	case "!", "?", "none", "rnd", "empty",
		"tls_clienthello", "http_req", "quic_initial",
		"fake_default_tls", "fake_default_quic",
		"stun", "syn_packet":
		return true
	default:
		return false
	}
}

func v2StrategyTokenNeedsExternalArtifact(token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	if v2StrategyDependencySafeToken(token) {
		return false
	}
	if strings.HasPrefix(token, "z2k_real_") {
		return true
	}
	if strings.HasPrefix(token, "tls_clienthello_") {
		return true
	}
	if strings.HasPrefix(token, "tls_") {
		return true
	}
	switch token {
	case "t2":
		return true
	default:
		return false
	}
}

func v2StrategyExternalDependencyKind(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "blob":
		return "blob"
	case "seqovl_pattern", "pattern", "fallback":
		return "pattern"
	default:
		return "artifact"
	}
}

func v2AddStrategyExternalDependency(out []v2StrategyExternalDependency, seen map[string]bool, dep v2StrategyExternalDependency) []v2StrategyExternalDependency {
	dep.Kind = strings.ToLower(strings.TrimSpace(dep.Kind))
	dep.Token = strings.ToLower(strings.TrimSpace(dep.Token))
	if dep.Kind == "" || dep.Token == "" {
		return out
	}
	if dep.Resolution == "" {
		dep.Resolution = "missing-provider"
	}
	key := dep.Kind + "\x00" + dep.Token
	if seen[key] {
		return out
	}
	seen[key] = true
	return append(out, dep)
}

func v2StrategyDesyncExternalDependencies(arg string, seen map[string]bool) []v2StrategyExternalDependency {
	if !strings.HasPrefix(arg, "--lua-desync=") {
		return nil
	}
	body := strings.TrimSpace(strings.TrimPrefix(arg, "--lua-desync="))
	if body == "" {
		return nil
	}
	fields := strings.Split(body, ":")
	method := strings.ToLower(strings.TrimSpace(fields[0]))
	out := []v2StrategyExternalDependency{}

	switch method {
	case "circular":
		out = v2AddStrategyExternalDependency(out, seen, v2StrategyExternalDependency{
			Kind:       "runtime",
			Token:      "circular",
			Argument:   arg,
			Resolution: "requires-z2k-circular-provider",
		})
	case "tls_client_hello_clone":
		out = v2AddStrategyExternalDependency(out, seen, v2StrategyExternalDependency{
			Kind:       "feature",
			Token:      "tls_client_hello_clone",
			Argument:   arg,
			Resolution: "requires-clone-provider",
		})
	}

	for _, field := range fields[1:] {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "blob", "seqovl_pattern", "pattern", "fallback":
			if v2StrategyTokenNeedsExternalArtifact(value) {
				out = v2AddStrategyExternalDependency(out, seen, v2StrategyExternalDependency{
					Kind:       v2StrategyExternalDependencyKind(key),
					Token:      value,
					Argument:   arg,
					Resolution: "requires-external-artifact",
				})
			}
		}
	}
	return out
}

func v2StrategyExternalDependencies(args []string) []v2StrategyExternalDependency {
	seen := map[string]bool{}
	out := []v2StrategyExternalDependency{}
	for _, arg := range args {
		for _, dep := range v2StrategyDesyncExternalDependencies(arg, seen) {
			out = append(out, dep)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Token != out[j].Token {
			return out[i].Token < out[j].Token
		}
		return out[i].Argument < out[j].Argument
	})
	return out
}

func v2StrategyHasExternalDependencies(args []string) bool {
	return len(v2StrategyExternalDependencies(args)) > 0
}
