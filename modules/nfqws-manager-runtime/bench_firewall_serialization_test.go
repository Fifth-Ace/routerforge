package main

import (
	"testing"
	"time"
)

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

func TestBenchFirewallRuleMutationMutexSerializesCriticalSections(t *testing.T) {
	benchFirewallRuleMutationMu.Lock()
	entered := make(chan struct{})
	done := make(chan struct{})
	go func() {
		benchFirewallRuleMutationMu.Lock()
		close(entered)
		benchFirewallRuleMutationMu.Unlock()
		close(done)
	}()

	select {
	case <-entered:
		benchFirewallRuleMutationMu.Unlock()
		t.Fatal("second firewall mutation entered while the critical section was held")
	case <-time.After(50 * time.Millisecond):
	}

	benchFirewallRuleMutationMu.Unlock()

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("second firewall mutation did not proceed after the critical section was released")
	}
	<-done
}