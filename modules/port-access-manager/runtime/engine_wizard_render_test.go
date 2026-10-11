package main

import (
	"strings"
	"testing"
)

func TestWizardRendersNoOpenFirewallOrSecrets(t *testing.T) {
	s := []ProtectedService{{Name: "web", Address: "192.168.10.103", Port: 18080}}
	knock := EngineWizardRequest{Engine: "knockd", Interface: "eth3", Knock: []int{7777, 8888, 6666}, SequenceSeconds: 15, AccessSeconds: 120, Services: s}
	k, err := renderWizard(knock)
	if err != nil {
		t.Fatal(err)
	}
	if !k.NeedsForwardGate || k.RequiresClientSecret {
		t.Fatal("wrong knockd capabilities")
	}
	if !strings.Contains(k.Files["knockd.conf.preview"], "sequence = 7777,8888,6666") {
		t.Fatal("missing knock sequence")
	}
	if strings.Contains(k.Files["knockd.conf.preview"], "iptables -I") {
		t.Fatal("unsafe firewall command")
	}
	spa := EngineWizardRequest{Engine: "fwknopd", Interface: "eth3", AccessSeconds: 120, Services: s}
	f, err := renderWizard(spa)
	if err != nil {
		t.Fatal(err)
	}
	if !f.RequiresClientSecret || !f.NeedsForwardGate {
		t.Fatal("wrong SPA capabilities")
	}
	if !strings.Contains(f.Files["access.conf.preview"], "OPEN_PORTS tcp/18080") {
		t.Fatal("missing ports")
	}
	if strings.Contains(f.Files["access.conf.preview"], "KEY_BASE64 __CHANGEME__") {
		t.Fatal("secret placeholder")
	}
}
