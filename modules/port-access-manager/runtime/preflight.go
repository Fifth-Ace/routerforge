package main

import (
	"os"
	"strings"
)

// Preflight deliberately reports evidence, not authorization to touch firewall.
// Every check is read-only; missing visibility is an unknown, never a PASS.
type PreflightCheck struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}

type Preflight struct {
	Module        string           `json:"module"`
	Mode          string           `json:"mode"`
	ReadyForApply bool             `json:"ready_for_apply"`
	MutationAPI   bool             `json:"mutation_api"`
	Checks        []PreflightCheck `json:"checks"`
}

func preflightFrom(kernelRecent bool, tableNames string, tableErr error) Preflight {
	result := Preflight{Module: "port-access-manager", Mode: "read-only-preflight", ReadyForApply: false, MutationAPI: false}
	add := func(id, state, detail string) {
		result.Checks = append(result.Checks, PreflightCheck{ID: id, State: state, Detail: detail})
	}
	if kernelRecent {
		add("recent_match", "present", "Kernel recent match is loaded; this does not prove firewall protection")
	} else {
		add("recent_match", "unknown", "recent match is not visible; capability has not been proven")
	}
	if tableErr != nil {
		add("filter_table", "unknown", "Cannot read kernel table registry")
	} else if strings.Contains("\n"+tableNames+"\n", "\nfilter\n") {
		add("filter_table", "present", "Kernel filter table is visible; no rule order has been verified")
	} else {
		add("filter_table", "unknown", "Filter table is not visible in kernel table registry")
	}
	add("ndm_forward_chain", "requires-verification", "Keenetic NDM forwarding chain order and persistence have not been verified")
	add("management_rescue", "requires-verification", "SSH 22/2222, active management path and an out-of-band rescue have not been verified")
	add("rollback", "requires-verification", "No timed rollback transaction has been staged or tested")
	add("wan_scope", "requires-verification", "WAN interface, protected target and NAT forwarding have not been verified")
	return result
}

func preflightSnapshot() Preflight {
	data, err := os.ReadFile("/proc/net/ip_tables_names")
	result := preflightFrom(kernelMatchPresent("/proc/net/ip_tables_matches", "recent"), strings.TrimSpace(string(data)), err)
	appendPreflightEvidence(&result)
	return result
}
