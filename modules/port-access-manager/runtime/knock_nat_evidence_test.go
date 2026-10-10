package main

import "testing"

func TestInspectDNATDirect(t *testing.T) {
	s := "-P PREROUTING ACCEPT\n-A PREROUTING -i eth3 -p tcp -m tcp --dport 50000 -j DNAT --to-destination 192.168.1.8:2222\n"
	dest, err := inspectDNATDirect(s, "eth3", 50000, 2222)
	if err != nil || dest != "192.168.1.8" {
		t.Fatalf("dest=%q err=%v", dest, err)
	}
	for _, tc := range []struct {
		wan         string
		ext, target int
	}{{"eth2", 50000, 2222}, {"eth3", 50001, 2222}, {"eth3", 50000, 22}, {"eth3", 0, 2222}} {
		if _, err := inspectDNATDirect(s, tc.wan, tc.ext, tc.target); err == nil {
			t.Fatalf("false match %+v", tc)
		}
	}
}
func TestInspectDNATDirectRejectsNDMIndirect(t *testing.T) {
	s := "-A PREROUTING -j _NDM_PREROUTING\n-A _NDM_PREROUTING -i eth3 -p tcp --dport 50000 -j DNAT --to-destination 192.168.1.8:2222\n"
	if _, err := inspectDNATDirect(s, "eth3", 50000, 2222); err == nil {
		t.Fatal("indirect NAT claimed verified")
	}
}
