package main

const (
	benchLifecycleStateNew              = "NEW"
	benchLifecycleStateCandidateStarted = "CANDIDATE_STARTED"
	benchLifecycleStateRuleInstalled    = "RULE_INSTALLED"
	benchLifecycleStateProbeRunning     = "PROBE_RUNNING"
	benchLifecycleStateCleaning         = "CLEANING"
	benchLifecycleStateClean            = "CLEAN"
	benchLifecycleStateFailed           = "FAILED"
	benchLifecycleStateCleanupUnproven  = "CLEANUP_UNPROVEN"
)

type benchCleanupProof struct {
	SelectedQueue          int      `json:"selected_queue,omitempty"`
	ReservedQueueMin       int      `json:"reserved_queue_min"`
	ReservedQueueMax       int      `json:"reserved_queue_max"`
	InventoryComplete      bool     `json:"inventory_complete"`
	ProcessReservedQueues  []int    `json:"process_reserved_queues"`
	FirewallReservedQueues []int    `json:"firewall_reserved_queues"`
	KernelReservedQueues   []int    `json:"kernel_reserved_queues"`
	OccupiedReservedQueues []int    `json:"occupied_reserved_queues"`
	ReservedRangeClean     bool     `json:"reserved_range_clean"`
	ProductionConfigSHA256 string   `json:"production_config_sha256,omitempty"`
	Proven                 bool     `json:"proven"`
	Reasons                []string `json:"reasons"`
}

type benchLifecycleContract struct {
	Implemented                bool              `json:"implemented"`
	ActiveSession              bool              `json:"active_session"`
	SessionMutationImplemented bool              `json:"session_mutation_implemented"`
	State                      string            `json:"state"`
	States                     []string          `json:"states"`
	Cleanup                    benchCleanupProof `json:"cleanup"`
}

func benchReservedQueues(queues []int) []int {
	out := []int{}
	seen := map[int]bool{}
	for _, q := range queues {
		if q < benchQueueMin || q > benchQueueMax || seen[q] {
			continue
		}
		seen[q] = true
		out = append(out, q)
	}
	return mergeBenchQueues(out)
}

func buildBenchCleanupProof(
	selectedQueue int,
	inventoryComplete bool,
	processQueues []int,
	firewallQueues []int,
	kernelQueues []int,
	configSHA256 string,
) benchCleanupProof {
	processReserved := benchReservedQueues(processQueues)
	firewallReserved := benchReservedQueues(firewallQueues)
	kernelReserved := benchReservedQueues(kernelQueues)
	occupiedReserved := mergeBenchQueues(processReserved, firewallReserved, kernelReserved)

	proof := benchCleanupProof{
		SelectedQueue:          selectedQueue,
		ReservedQueueMin:       benchQueueMin,
		ReservedQueueMax:       benchQueueMax,
		InventoryComplete:      inventoryComplete,
		ProcessReservedQueues:  processReserved,
		FirewallReservedQueues: firewallReserved,
		KernelReservedQueues:   kernelReserved,
		OccupiedReservedQueues: occupiedReserved,
		ReservedRangeClean:     len(occupiedReserved) == 0,
		ProductionConfigSHA256: configSHA256,
		Reasons:                []string{},
	}

	if selectedQueue < benchQueueMin || selectedQueue > benchQueueMax {
		proof.Reasons = append(proof.Reasons, "reserved bench queue was not selected")
	}
	if !inventoryComplete {
		proof.Reasons = append(proof.Reasons, "queue inventory is incomplete")
	}
	if len(processReserved) > 0 {
		proof.Reasons = append(proof.Reasons, "reserved bench queue is already owned by an nfqws2 process")
	}
	if len(firewallReserved) > 0 {
		proof.Reasons = append(proof.Reasons, "reserved bench queue is already referenced by firewall rules")
	}
	if len(kernelReserved) > 0 {
		proof.Reasons = append(proof.Reasons, "reserved bench queue is already bound in kernel NFQUEUE state")
	}

	proof.Proven = len(proof.Reasons) == 0
	return proof
}

func buildBenchLifecycleContract(
	selectedQueue int,
	inventoryComplete bool,
	processQueues []int,
	firewallQueues []int,
	kernelQueues []int,
	configSHA256 string,
) benchLifecycleContract {
	cleanup := buildBenchCleanupProof(
		selectedQueue,
		inventoryComplete,
		processQueues,
		firewallQueues,
		kernelQueues,
		configSHA256,
	)

	state := benchLifecycleStateCleanupUnproven
	if cleanup.Proven {
		state = benchLifecycleStateClean
	}

	return benchLifecycleContract{
		Implemented:                true,
		ActiveSession:              false,
		SessionMutationImplemented: false,
		State:                      state,
		States: []string{
			benchLifecycleStateNew,
			benchLifecycleStateCandidateStarted,
			benchLifecycleStateRuleInstalled,
			benchLifecycleStateProbeRunning,
			benchLifecycleStateCleaning,
			benchLifecycleStateClean,
			benchLifecycleStateFailed,
			benchLifecycleStateCleanupUnproven,
		},
		Cleanup: cleanup,
	}
}
