package main

import (
	"net/http"
	"sort"
)

type v2NFQueueHealth struct {
	OK                    bool   `json:"ok"`
	State                 string `json:"state"`
	ProductionRunning     bool   `json:"production_running"`
	ReadOnly              bool   `json:"read_only"`
	ProductionMutation    bool   `json:"production_mutation"`
	FirewallInventoryOK   bool   `json:"firewall_inventory_ok"`
	KernelInventoryOK     bool   `json:"kernel_inventory_ok"`
	ProductionQueues      []int  `json:"production_queues"`
	FirewallQueues        []int  `json:"firewall_queues"`
	KernelQueues          []int  `json:"kernel_queues"`
	MissingFirewallQueues []int  `json:"missing_firewall_queues"`
	MissingKernelQueues   []int  `json:"missing_kernel_queues"`
	StaleFirewallQueues   []int  `json:"stale_firewall_queues"`
	Reason                string `json:"reason"`
	ConfigSHA256          string `json:"config_sha256,omitempty"`
}

func registerNFQueueHealthV1Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/nfqueue-health", getOnly(handleV2NFQueueHealth))
}

func v2NonReservedQueues(values []int) []int {
	out := []int{}
	seen := map[int]bool{}
	for _, value := range values {
		if value < 0 || value > 65535 || (value >= benchQueueMin && value <= benchQueueMax) || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func v2QueueDifference(expected, actual []int) []int {
	have := map[int]bool{}
	for _, value := range actual {
		have[value] = true
	}
	out := []int{}
	for _, value := range expected {
		if !have[value] {
			out = append(out, value)
		}
	}
	sort.Ints(out)
	return out
}

func buildV2NFQueueHealth(running, firewallOK, kernelOK bool, processQueues, firewallQueues, kernelQueues []int, configSHA string) v2NFQueueHealth {
	production := v2NonReservedQueues(processQueues)
	firewall := v2NonReservedQueues(firewallQueues)
	kernel := v2NonReservedQueues(kernelQueues)
	result := v2NFQueueHealth{
		State:               "INCONCLUSIVE",
		ProductionRunning:   running,
		ReadOnly:            true,
		ProductionMutation:  false,
		FirewallInventoryOK: firewallOK,
		KernelInventoryOK:   kernelOK,
		ProductionQueues:    production,
		FirewallQueues:      firewall,
		KernelQueues:        kernel,
		ConfigSHA256:        configSHA,
	}
	if !running {
		result.State = "STOPPED"
		result.Reason = "production nfqws2 is not running"
		return result
	}
	if !firewallOK || !kernelOK {
		result.Reason = "NFQUEUE inventory is incomplete"
		return result
	}
	if len(production) == 0 {
		result.Reason = "running nfqws2 exposes no production queue number in argv"
		return result
	}
	result.MissingFirewallQueues = v2QueueDifference(production, firewall)
	result.MissingKernelQueues = v2QueueDifference(production, kernel)
	result.StaleFirewallQueues = v2QueueDifference(firewall, kernel)
	if len(result.MissingFirewallQueues) > 0 || len(result.MissingKernelQueues) > 0 {
		result.State = "UNHEALTHY"
		result.Reason = "production NFQUEUE binding is stale or missing"
		return result
	}
	result.OK = true
	result.State = "HEALTHY"
	result.Reason = "production NFQUEUE process/firewall/kernel evidence agrees"
	return result
}

func readV2NFQueueHealth() v2NFQueueHealth {
	status := readStatus()
	_, processQueues := readBenchProcesses()
	iptablesSave := findExecutable("iptables-save")
	nft := findExecutable("nft")
	firewallQueues, _, firewallOK, _ := readFirewallQueueInventory(iptablesSave, nft)
	kernelQueues, kernelOK := readKernelQueueInventory()
	return buildV2NFQueueHealth(
		status.Running,
		firewallOK,
		kernelOK,
		processQueues,
		firewallQueues,
		kernelQueues,
		status.ConfigSHA256,
	)
}

func handleV2NFQueueHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, readV2NFQueueHealth())
}
