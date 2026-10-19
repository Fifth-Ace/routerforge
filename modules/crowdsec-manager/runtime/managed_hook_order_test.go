package main

import (
	"strings"
	"testing"
)

// Contract test: a newly installed RouterForge hook must never preempt the
// Keenetic system rules or create a second jump on subsequent rebuilds.
func TestManagedNDMHookOrderContract(t *testing.T) {
	required := []string{
		`-S FORWARD`,
		`"_NDM_FORWARD"`,
		`"RELATED"`,
		`"ESTABLISHED"`,
		`-I FORWARD "$POSITION" -j CROWDSEC_CHAIN`,
		`-C FORWARD -j CROWDSEC_CHAIN`,
	}
	for _, fragment := range required {
		if !strings.Contains(managedNDMHook, fragment) {
			t.Fatalf("missing insertion guard: %q", fragment)
		}
	}
	if strings.Contains(managedNDMHook, `-I FORWARD 1 -j CROWDSEC_CHAIN`) || strings.Contains(managedNDMHook, `-I FORWARD 8 -j CROWDSEC_CHAIN`) {
		t.Fatal("hard-coded FORWARD position is forbidden")
	}
	find := strings.Index(managedNDMHook, `-S FORWARD`)
	mutate := strings.Index(managedNDMHook, `-F CROWDSEC_CHAIN`)
	if find < 0 || mutate < find {
		t.Fatal("preflight must precede firewall mutation")
	}
}
