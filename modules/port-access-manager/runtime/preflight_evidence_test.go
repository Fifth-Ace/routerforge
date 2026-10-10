package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListenPortsEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tcp")
	sample := "  sl  local_address rem_address   st\n 0: 00000000:0016 00000000:0000 0A\n 1: 00000000:08AE 00000000:0000 01\n"
	if err := os.WriteFile(path, []byte(sample), 0600); err != nil {
		t.Fatal(err)
	}
	ports := listenPorts(path)
	if !ports[22] || ports[2222] {
		t.Fatalf("LISTEN evidence parsed incorrectly: %#v", ports)
	}
}

func TestEvidenceDoesNotAuthorizeApply(t *testing.T) {
	p := preflightFrom(true, "filter", nil)
	appendPreflightEvidence(&p)
	if p.ReadyForApply || p.MutationAPI || len(p.Checks) != 9 {
		t.Fatalf("unsafe result: %+v", p)
	}
	for _, c := range p.Checks {
		if c.State == "pass" || c.State == "ready" {
			t.Fatalf("unsafe gate: %+v", c)
		}
	}
}
