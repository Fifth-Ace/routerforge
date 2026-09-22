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

func TestEditableListName(t *testing.T) {
	for _, name := range []string{"user.list", "exclude.list-opkg", "backup.list-old"} {
		if !editableListName(name) {
			t.Fatalf("expected editable list name: %s", name)
		}
	}
	for _, name := range []string{"nfqws2.conf", "../user.list", ".hidden.list", "bad name.list"} {
		if editableListName(name) {
			t.Fatalf("expected invalid list name: %s", name)
		}
	}
}

func TestProtectedListName(t *testing.T) {
	for _, name := range []string{"user.list", "exclude.list", "auto.list", "ipset.list", "ipset_exclude.list"} {
		if !protectedListName(name) {
			t.Fatalf("expected protected list: %s", name)
		}
	}
	if protectedListName("youtube.list") {
		t.Fatal("custom list must not be protected")
	}
}

func TestNormalizeCheckURL(t *testing.T) {
	got, err := normalizeCheckURL("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://example.com/" {
		t.Fatalf("unexpected normalized URL: %s", got)
	}
	for _, value := range []string{"http://127.0.0.1/", "file:///etc/passwd", "localhost", "http://example.com:8080/"} {
		if _, err := normalizeCheckURL(value); err == nil {
			t.Fatalf("expected URL rejection: %s", value)
		}
	}
}
func TestNFQWSManagerHealthPayloadIncludesConfigSHA(t *testing.T) {
	status := managerStatus{
		Available:    true,
		TargetID:     "nfqws2",
		Running:      true,
		ConfigSHA256: strings.Repeat("a", 64),
	}
	payload := nfqwsManagerHealthPayload(status)
	if payload["config_sha256"] != status.ConfigSHA256 {
		t.Fatalf("health config_sha256=%v want=%s", payload["config_sha256"], status.ConfigSHA256)
	}
	if payload["target_id"] != "nfqws2" || payload["target_running"] != true {
		t.Fatalf("health identity changed: %+v", payload)
	}
}
