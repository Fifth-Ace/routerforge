package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNF1AShellSyntaxValid(t *testing.T) {
	for _, content := range []string{
		"ISP_INTERFACE=eth3\nNFQWS_ARGS='--filter-tcp=443 --lua-desync=multisplit:pos=1'\n",
		"NFQWS_ARGS_CUSTOM=\"--filter-tcp=443\\\n--lua-desync=multisplit:pos=1\"\n",
		"# A comment\nif true; then\n  TCP_PORTS=443\nfi\n",
	} {
		if err := checkNFQWSShellSyntax(content); err != nil {
			t.Fatalf("valid config rejected: %q: %v", content, err)
		}
	}
}

func TestNF1AShellSyntaxRejectsInvalid(t *testing.T) {
	for _, content := range []string{
		"NFQWS_ARGS='unclosed\n",
		"if true; then\n  TCP_PORTS=443\n",
	} {
		if err := checkNFQWSShellSyntax(content); err == nil || !strings.Contains(err.Error(), "синтаксиса") {
			t.Fatalf("invalid config accepted: %q, err %v", content, err)
		}
	}
}

func TestNF1AShellSyntaxDoesNotExecuteCommands(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "SHOULD_NOT_EXIST")
	content := "VALUE=$(touch " + marker + ")\n"
	if err := checkNFQWSShellSyntax(content); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("config was executed during syntax check: err=%v", err)
	}
}
