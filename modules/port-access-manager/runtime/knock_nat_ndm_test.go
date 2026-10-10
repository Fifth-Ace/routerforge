package main

import "testing"

func TestNDMOneHopMatch(t *testing.T) {
	s := "-P PREROUTING ACCEPT\n-A PREROUTING -j _NDM_NAT\n-A _NDM_NAT -i eth3 -p tcp -m tcp --dport 50000 -j DNAT --to-destination 192.168.1.8:2222\n"
	got, err := inspectDNATNDM(s, "eth3", 50000, 2222)
	if err != nil || got != "192.168.1.8" {
		t.Fatalf("got %q err %v", got, err)
	}
	for _, tc := range []struct {
		wan         string
		ext, target int
	}{{"eth2", 50000, 2222}, {"eth3", 50001, 2222}, {"eth3", 50000, 22}} {
		if _, err := inspectDNATNDM(s, tc.wan, tc.ext, tc.target); err == nil {
			t.Fatalf("false positive %+v", tc)
		}
	}
}
func TestNDMRejectsUnprovenTopology(t *testing.T) {
	cases := []string{
		"-A _NDM_NAT -i eth3 -p tcp --dport 50000 -j DNAT --to-destination 192.168.1.8:2222",
		"-A PREROUTING -i eth2 -j _NDM_NAT\n-A _NDM_NAT -i eth3 -p tcp --dport 50000 -j DNAT --to-destination 192.168.1.8:2222",
		"-A PREROUTING -j _NDM_NAT\n-A _NDM_NAT -j OTHER\n-A OTHER -i eth3 -p tcp --dport 50000 -j DNAT --to-destination 192.168.1.8:2222",
		"-A PREROUTING -j _NDM_NAT\n-A _NDM_NAT -i eth3 -p tcp --dport 50000 -j DNAT --to-destination 192.168.1.8:2222\n-A _NDM_NAT -i eth3 -p tcp --dport 50000 -j DNAT --to-destination 192.168.1.9:2222",
	}
	for _, s := range cases {
		if got, err := inspectDNATNDM(s, "eth3", 50000, 2222); err == nil {
			t.Fatalf("accepted %s: %q", s, got)
		}
	}
}
