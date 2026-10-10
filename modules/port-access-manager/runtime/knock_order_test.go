package main

import "testing"

func TestForwardOrderReadOnly(t *testing.T) {
	for _, rules := range []string{
		"-P FORWARD ACCEPT\n-A FORWARD -j _NDM_FORWARD\n-A FORWARD -j RETURN\n",
		"-P FORWARD DROP\n-A INPUT -j _NDM_INPUT\n",
		"-P FORWARD ACCEPT\n-A FORWARD -i eth3 -j DROP\n-A FORWARD -j _NDM_FORWARD\n",
	} {
		pos, err := checkForwardOrder(rules, "eth3", 12345)
		if err != nil || pos != 1 {
			t.Fatalf("position=%d err=%v", pos, err)
		}
	}
}
func TestForwardOrderRejectDuplicateAndInvalid(t *testing.T) {
	rules := "-P FORWARD ACCEPT\n-A FORWARD -i eth3 -p tcp --dport 12345 -j RF_PORT_KNOCK\n-A FORWARD -j _NDM_FORWARD\n"
	if _, err := checkForwardOrder(rules, "eth3", 12345); err == nil {
		t.Fatal("duplicate hook accepted")
	}
	if _, err := checkForwardOrder("-P FORWARD ACCEPT\n", "bad;iface", 12345); err == nil {
		t.Fatal("invalid scope accepted")
	}
}
