package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestNF4CFrontendOnlyShowsSuccessAfterLiveTargetVerification(t *testing.T) {
	bytes, err := os.ReadFile("../nfqws-manager/frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	content := string(bytes)
	for _, marker := range []string{
		"if(!result.post_apply_verified)",
		"сайт повторно открылся через nfqws2",
	} {
		if !strings.Contains(content, marker) {
			t.Errorf("NF4C UI gate missing: %s", marker)
		}
	}
	start := strings.Index(content, "<script>")
	end := strings.Index(content, "</script>")
	if start < 0 || end <= start || strings.Count(content, "<script>") != 1 {
		t.Fatal("inline JavaScript is not uniquely located")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable; frontend marker assertions still run")
	}
	c := exec.Command(node, "--check", "-")
	c.Stdin = strings.NewReader(content[start+len("<script>") : end])
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("inline JavaScript invalid: %v: %s", err, out)
	}
}
