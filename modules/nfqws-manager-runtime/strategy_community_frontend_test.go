package main

import (
	"os/exec"
	"testing"
)

// Verify extracted Svelte-owned NFQWS runtime module parses; this is a
// syntax gate, NOT a functional Svelte build or browser parity proof.
func TestNF3CInlineJavaScriptSyntax(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable")
	}
	cmd := exec.Command(node, "--check", "../nfqws-manager/frontend/NfqwsRuntime.js")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("NFQWS runtime JS syntax: %v\n%s", err, out)
	}
}
