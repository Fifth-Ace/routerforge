package main

// buildKnockRollback is a dry-run inverse of the module-owned chain creation.
// No commands are executed; the caller must separately verify that the chain
// is not referenced before a future removal. NDM chains are never modified.
func buildKnockRollback() [][]string {
	return [][]string{
		{"iptables", "-t", "filter", "-F", "RF_PORT_KNOCK"},
		{"iptables", "-t", "filter", "-X", "RF_PORT_KNOCK"},
	}
}
