package main

import "testing"

func TestBenchFirewallCommandRecognition(t *testing.T) {
	for _, program := range []string{
		"iptables",
		"iptables-save",
		"/opt/sbin/iptables",
		"/opt/sbin/iptables-save",
	} {
		if !benchFirewallCommand(program) {
			t.Fatalf("%q was not recognized as a serialized firewall command", program)
		}
	}
	if benchFirewallCommand("/opt/bin/nfqws2") {
		t.Fatal("candidate process command must not be serialized as a firewall command")
	}
}
