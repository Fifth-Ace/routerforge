package main

// buildKnockRollback is a dry-run removal plan. It is safe only for an
// unhooked module-owned chain set; callers must verify zero references.
func buildKnockRollback() [][]string {
	return [][]string{
		{"iptables", "-t", "filter", "-F", "RF_PORT_KNOCK"},
		{"iptables", "-t", "filter", "-F", "RF_KNOCK_STEP2"},
		{"iptables", "-t", "filter", "-F", "RF_KNOCK_STEP3"},
		{"iptables", "-t", "filter", "-X", "RF_KNOCK_STEP2"},
		{"iptables", "-t", "filter", "-X", "RF_KNOCK_STEP3"},
		{"iptables", "-t", "filter", "-X", "RF_PORT_KNOCK"},
	}
}
