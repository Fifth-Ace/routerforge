package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// This is a syntax-only check. JavaScript is not executed. On GitHub's Linux
// runner node is available; development hosts without node skip this test.
func TestNF3CInlineJavaScriptSyntax(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable on this host")
	}
	html, err := os.ReadFile(filepath.Join("..", "nfqws-manager", "frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	scripts := regexp.MustCompile(`(?is)<script\b[^>]*>(.*?)</script>`).FindAllSubmatch(html, -1)
	tested := 0
	for _, script := range scripts {
		if len(strings.TrimSpace(string(script[1]))) == 0 {
			continue
		}
		path := filepath.Join(t.TempDir(), "nf3c-inline.js")
		if err := os.WriteFile(path, script[1], 0600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(node, "--check", path).CombinedOutput()
		if err != nil {
			t.Fatalf("JavaScript syntax error: %v\n%s", err, out)
		}
		tested++
	}
	if tested == 0 {
		t.Fatal("no inline JavaScript checked")
	}
}
