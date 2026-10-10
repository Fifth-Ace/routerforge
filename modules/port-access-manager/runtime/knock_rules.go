package main

import (
	"errors"
	"strconv"
)

// KnockRules is a dry-run plan. Nothing in this file invokes iptables.
// The chain is deliberately NOT hooked into NDM/FORWARD.
type KnockRules struct {
	Chain    string     `json:"chain"`
	Hooked   bool       `json:"hooked"`
	Applied  bool       `json:"applied"`
	Commands [][]string `json:"commands"`
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
	p := func(i int) string { return strconv.Itoa(i) }
	chain := "RF_PORT_KNOCK"
	c := [][]string{
		{"iptables", "-t", "filter", "-N", chain},
		{"iptables", "-t", "filter", "-A", chain, "-p", "tcp", "--dport", p(o.Sequence[0]), "-m", "recent", "--name", "RF_KNOCK_1", "--set", "-j", "RETURN"},
		{"iptables", "-t", "filter", "-A", chain, "-p", "tcp", "--dport", p(o.Sequence[1]), "-m", "recent", "--name", "RF_KNOCK_1", "--rcheck", "--seconds", p(o.WindowSeconds), "--reap", "-m", "recent", "--name", "RF_KNOCK_2", "--set", "-j", "RETURN"},
		{"iptables", "-t", "filter", "-A", chain, "-p", "tcp", "--dport", p(o.Sequence[2]), "-m", "recent", "--name", "RF_KNOCK_2", "--rcheck", "--seconds", p(o.WindowSeconds), "--reap", "-m", "recent", "--name", "RF_KNOCK_AUTH", "--set", "-j", "RETURN"},
		{"iptables", "-t", "filter", "-A", chain, "-p", "tcp", "--dport", p(o.Target), "-m", "recent", "--name", "RF_KNOCK_AUTH", "--rcheck", "--seconds", p(o.AccessSeconds), "--reap", "-j", "RETURN"},
		{"iptables", "-t", "filter", "-A", chain, "-p", "tcp", "--dport", p(o.Target), "-j", "DROP"},
	}
	return KnockRules{Chain: chain, Hooked: false, Applied: false, Commands: c}, nil
}
