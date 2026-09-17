package main

import (
	"strings"
	"testing"
)

func TestValidateNFQWS2Config(t *testing.T) {
	if err := validateNFQWS2Config("ENABLED=1\n"); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if err := validateNFQWS2Config(""); err == nil {
		t.Fatal("empty config must be rejected")
	}
	if err := validateNFQWS2Config("A=\x00B\n"); err == nil {
		t.Fatal("NUL config must be rejected")
	}
	tooLarge := strings.Repeat("x", nfqws2ConfigMaxBytes+1)
	if err := validateNFQWS2Config(tooLarge); err == nil {
		t.Fatal("oversized config must be rejected")
	}
}

func TestSafeNFQWS2ListName(t *testing.T) {
	for _, name := range []string{"user.list", "exclude-hosts.txt", "ipset_1.txt"} {
		if !safeNFQWS2ListName(name) {
			t.Fatalf("expected safe list name: %s", name)
		}
	}
	for _, name := range []string{"../x", "/tmp/x", ".secret", "a b", "x/../y"} {
		if safeNFQWS2ListName(name) {
			t.Fatalf("expected unsafe list name: %s", name)
		}
	}
}
