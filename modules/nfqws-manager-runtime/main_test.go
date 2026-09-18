package main

import (
	"strings"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	if err := validateConfig("ENABLED=1\n"); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if err := validateConfig(""); err == nil {
		t.Fatal("empty config must be rejected")
	}
	if err := validateConfig("A=\x00B\n"); err == nil {
		t.Fatal("NUL must be rejected")
	}
	if err := validateConfig(strings.Repeat("x", configMaxBytes+1)); err == nil {
		t.Fatal("oversized config must be rejected")
	}
}

func TestSafeListName(t *testing.T) {
	for _, name := range []string{"user.list", "exclude-hosts.txt", "ipset_1.txt"} {
		if !safeListName(name) {
			t.Fatalf("expected safe: %s", name)
		}
	}
	for _, name := range []string{"../x", "/tmp/x", ".secret", "a b", "x/../y"} {
		if safeListName(name) {
			t.Fatalf("expected unsafe: %s", name)
		}
	}
}
