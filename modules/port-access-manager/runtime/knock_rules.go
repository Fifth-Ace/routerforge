package main

import (
	"errors"
	"strconv"
)

// KnockRules is a dry-run plan only; no iptables commands are executed.
// An iptables recent stage transition is deliberately split into separate
// chains so every rule uses exactly one recent matcher.
// NOTE: this is gated progression, not yet strict sequence enforcement.
// Previous stage entries can survive an unrelated packet until expiry.
// Deployment remains prohibited pending packet-level validation.
type KnockRules struct {
	Chain                  string     `json:"chain"`
	Hooked                 bool       `json:"hooked"`
	Applied                bool       `json:"applied"`
	Commands               [][]string `json:"commands"`
	StrictSequenceVerified bool       `json:"strict_sequence_verified"`
	DeploymentBlocked      bool       `json:"deployment_blocked"`
	DeploymentBlockers     []string   `json:"deployment_blockers"`
}

type KnockOptions struct {
	Sequence      [3]int
	Target        int
	WindowSeconds int
	AccessSeconds int
}

func buildKnockRules(o KnockOptions) (KnockRules, error) {
	all := []int{o.Sequence[0], o.Sequence[1], o.Sequence[2], o.Target}
	seen := map[int]bool{}
	for _, port := range all {
		if port < 1 || port > 65535 || seen[port] {
			return KnockRules{}, errors.New("ports must be distinct and in range 1..65535")
		}
		seen[port] = true
	}
	if o.WindowSeconds < 5 || o.WindowSeconds > 300 || o.AccessSeconds < 30 || o.AccessSeconds > 3600 {
		return KnockRules{}, errors.New("time limits out of range")
	}
	p := func(v int) string { return strconv.Itoa(v) }
	chain := "RF_PORT_KNOCK"
	// Only the primary chain may ever be externally hooked (NOT in this plan).
	// Step subchains set the next stage after the main chain has checked the prior stage.
	c := [][]string{
		{"iptables", "-t", "filter", "-N", chain},
		{"iptables", "-t", "filter", "-N", "RF_KNOCK_STEP2"},
		{"iptables", "-t", "filter", "-N", "RF_KNOCK_STEP3"},
		{"iptables", "-t", "filter", "-A", chain, "-p", "tcp", "--dport", p(o.Sequence[0]), "-m", "recent", "--name", "RF_KNOCK_1", "--set", "-j", "RETURN"},
		{"iptables", "-t", "filter", "-A", chain, "-p", "tcp", "--dport", p(o.Sequence[1]), "-m", "recent", "--name", "RF_KNOCK_1", "--rcheck", "--seconds", p(o.WindowSeconds), "--reap", "-j", "RF_KNOCK_STEP2"},
		{"iptables", "-t", "filter", "-A", chain, "-p", "tcp", "--dport", p(o.Sequence[2]), "-m", "recent", "--name", "RF_KNOCK_2", "--rcheck", "--seconds", p(o.WindowSeconds), "--reap", "-j", "RF_KNOCK_STEP3"},
		{"iptables", "-t", "filter", "-A", chain, "-p", "tcp", "--dport", p(o.Target), "-m", "recent", "--name", "RF_KNOCK_AUTH", "--rcheck", "--seconds", p(o.AccessSeconds), "--reap", "-j", "RETURN"},
		{"iptables", "-t", "filter", "-A", chain, "-p", "tcp", "--dport", p(o.Target), "-j", "DROP"},
		{"iptables", "-t", "filter", "-A", "RF_KNOCK_STEP2", "-m", "recent", "--name", "RF_KNOCK_2", "--set", "-j", "RETURN"},
		{"iptables", "-t", "filter", "-A", "RF_KNOCK_STEP3", "-m", "recent", "--name", "RF_KNOCK_AUTH", "--set", "-j", "RETURN"},
	}
	return KnockRules{Chain: chain, Hooked: false, Applied: false, StrictSequenceVerified: false, DeploymentBlocked: true, DeploymentBlockers: []string{"strict sequence not proven", "kernel packet-level verification missing", "hardware acceptance missing"}, Commands: c}, nil
}
