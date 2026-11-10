package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestNF4CFrontendOnlyShowsSuccessAfterLiveTargetVerification(t *testing.T) {
	content := nfqwsFrontendSource(t)
	for _, marker := range []string{
		"if(!result.post_apply_verified)",
		"сайт повторно открылся через nfqws2",
	} {
		if !strings.Contains(content, marker) {
			t.Errorf("NF4C UI gate missing: %s", marker)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable; frontend marker assertions still run")
	}
	c := exec.Command(node, "--check", "../nfqws-manager/frontend/NfqwsRuntime.js")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("inline JavaScript invalid: %v: %s", err, out)
	}
}
