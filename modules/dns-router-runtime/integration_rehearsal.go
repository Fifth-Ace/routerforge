package dnsrouter

// DPR2-G is a deliberately self-contained in-memory rehearsal of transaction
// semantics. No adapter, shell command, network, sysfs or Keenetic API is used.

type IntegrationRehearsal struct {
	SimulationOnly    bool     `json:"simulation_only"`
	Scenario          string   `json:"scenario"`
	State             string   `json:"state"`
	Steps             []string `json:"steps"`
	BaselineRestored  bool     `json:"baseline_restored"`
	ActivationAllowed bool     `json:"activation_allowed"`
	ChangesApplied    bool     `json:"changes_applied"`
	HardwareAccepted  bool     `json:"hardware_accepted"`
}

func RehearseKeeneticTransaction(failAt string) IntegrationRehearsal {
	out := IntegrationRehearsal{SimulationOnly: true, Scenario: failAt, State: "FAILED", ActivationAllowed: false, ChangesApplied: false, HardwareAccepted: false}
	if failAt != "none" && failAt != "snapshot" && failAt != "apply" && failAt != "verify" && failAt != "rollback" && failAt != "rollback-verify" {
		out.State = "INVALID_SCENARIO"
		return out
	}
	baseline := "original-keenetic-preimage"
	current := baseline
	out.Steps = append(out.Steps, "PRECHECK: simulated only")
	if failAt == "snapshot" {
		out.Steps = append(out.Steps, "SNAPSHOT: injected failure; no change")
		out.BaselineRestored = true
		return out
	}
	out.Steps = append(out.Steps, "SNAPSHOT: original-preimage captured in memory")
	current = "staged-dns-proxy-setting"
	out.Steps = append(out.Steps, "APPLY: simulated in-memory replacement")
	if failAt == "none" {
		out.Steps = append(out.Steps, "VERIFY: staged state matches", "COMMIT: simulated only")
		out.State = "SIMULATED_COMMIT"
		return out
	}
	if failAt == "verify" || failAt == "rollback" || failAt == "rollback-verify" {
		out.Steps = append(out.Steps, "VERIFY: injected failure")
	} else {
		out.Steps = append(out.Steps, "APPLY: injected partial failure")
	}
	if failAt == "rollback" {
		out.Steps = append(out.Steps, "ROLLBACK: injected failure; state ambiguous")
		out.State = "AMBIGUOUS"
		return out
	}
	current = baseline
	out.Steps = append(out.Steps, "ROLLBACK: simulated baseline restored")
	if failAt == "rollback-verify" {
		out.Steps = append(out.Steps, "ROLLBACK_VERIFY: injected failure; state ambiguous")
		out.State = "AMBIGUOUS"
		return out
	}
	out.BaselineRestored = current == baseline
	out.Steps = append(out.Steps, "ROLLBACK_VERIFY: simulated baseline identity matches")
	out.State = "SIMULATED_ROLLBACK"
	return out
}
