package main

import "testing"

func TestCrowdSecFirewallVariants(t *testing.T) {
	sets := []string{"crowdsec-blacklists-ipv4"}
	original := "-A FORWARD -j CROWDSEC_CHAIN\n"
	standard := "-A CROWDSEC_CHAIN -m set --match-set crowdsec-blacklists-ipv4 src -j DROP"
	custom := "-A _CROWDSEC_CHAIN -m set --match-set crowdsec-blacklists-ipv4 src -j DROP"
	good := []struct{ name, fwd, ndm, std, cus string }{
		{"standard", original, "", standard, ""},
		{"custom_direct", "-A FORWARD -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT\n-A FORWARD -j _CROWDSEC_CHAIN\n-A FORWARD -j _NDM_FORWARD", "", "", custom},
		{"custom_nested", "-A FORWARD -j _NDM_FORWARD", "-A _NDM_FORWARD -j _CROWDSEC_CHAIN", "", custom},
	}
	for _, tc := range good {
		t.Run(tc.name, func(t *testing.T) {
			if x := analyzeCrowdSecFirewall(tc.fwd, tc.ndm, tc.std, tc.cus, sets); !x.OK {
				t.Fatalf("unexpected fail: %+v", x)
			}
		})
	}
	bad := []struct{ name, fwd, ndm, std, cus string }{
		{"no_jump", "-A FORWARD -j ACCEPT", "", "", custom},
		{"wrong_order", "-A FORWARD -j _NDM_FORWARD\n-A FORWARD -j _CROWDSEC_CHAIN", "", "", custom},
		{"wrong_ipset", "-A FORWARD -j _CROWDSEC_CHAIN", "", "", "-A _CROWDSEC_CHAIN -m set --match-set other src -j DROP"},
		{"wrong_direction", "-A FORWARD -j _CROWDSEC_CHAIN", "", "", "-A _CROWDSEC_CHAIN -m set --match-set crowdsec-blacklists-ipv4 dst -j DROP"},
		{"not_drop", "-A FORWARD -j _CROWDSEC_CHAIN", "", "", "-A _CROWDSEC_CHAIN -m set --match-set crowdsec-blacklists-ipv4 src -j ACCEPT"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if x := analyzeCrowdSecFirewall(tc.fwd, tc.ndm, tc.std, tc.cus, sets); x.OK {
				t.Fatalf("false green: %+v", x)
			}
		})
	}
}
func TestNoSubstringFalsePositive(t *testing.T) {
	if hasJump("-A FORWARD -j _CROWDSEC_CHAIN_FAKE", "FORWARD", "_CROWDSEC_CHAIN") {
		t.Fatal("substring accepted")
	}
}
