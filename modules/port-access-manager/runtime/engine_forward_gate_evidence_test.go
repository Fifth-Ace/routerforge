package main

import (
	"strings"
	"testing"
)

func TestForwardGateEvidenceDSTestFailsClosed(t *testing.T) {
	input := `-P FORWARD DROP
-A FORWARD -m state --state RELATED,ESTABLISHED -j ACCEPT
-A FORWARD -j _NDM_FORWARD
-A FORWARD -m conntrack --ctstate DNAT -j ACCEPT
`
	e, err := inspectForwardGateRules(input)
	if err != nil {
		t.Fatal(err)
	}
	if !e.HasDNATAccept || !e.HasKeeneticForward || e.SuitableForDirectApply {
		t.Fatalf("unsafe gate conclusion: %+v", e)
	}
	if !strings.Contains(e.Reason, "DNAT") {
		t.Fatal("DNAT bypass not surfaced")
	}
}
func TestForwardGateEvidenceRejectsUnknownLayouts(t *testing.T) {
	if _, err := inspectForwardGateRules(""); err == nil {
		t.Fatal("empty rules accepted")
	}
	if _, err := inspectForwardGateRules("-A FORWARD -j ACCEPT"); err == nil {
		t.Fatal("missing policy accepted")
	}
	e, err := inspectForwardGateRules("-P FORWARD DROP\n-A FORWARD -j RF_PA_TEST\n")
	if err != nil || e.SuitableForDirectApply || !e.HasExistingRouterForgeGate {
		t.Fatal("ownership gate not detected")
	}
}
