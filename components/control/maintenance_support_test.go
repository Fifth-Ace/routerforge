package main

import (
	"strings"
	"testing"
)

func TestClassifyAdminSupportStorage(t *testing.T) {
	tests := []struct {
		used float64
		want string
	}{
		{0, "ok"},
		{79.9, "ok"},
		{80, "warning"},
		{89.9, "warning"},
		{90, "critical"},
		{100, "critical"},
	}
	for _, test := range tests {
		if got := classifyAdminSupportStorage(test.used); got != test.want {
			t.Fatalf("used=%v got=%q want=%q", test.used, got, test.want)
		}
	}
}

func TestRedactAdminSupportText(t *testing.T) {
	input := "token=abc123\nAuthorization: Bearer secret-token\nPrivateKey = raw-key\n-----BEGIN PRIVATE KEY-----\nsecret-body\n-----END PRIVATE KEY-----\nnormal line"
	got := redactAdminSupportText(input)
	for _, forbidden := range []string{"abc123", "secret-token", "raw-key", "secret-body"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("redaction leaked %q: %s", forbidden, got)
		}
	}
	if !strings.Contains(got, "normal line") {
		t.Fatalf("normal line lost: %s", got)
	}
	if !strings.Contains(got, "[REDACTED") {
		t.Fatalf("redaction marker missing: %s", got)
	}
}

func TestValidAdminSupportBundleName(t *testing.T) {
	valid := []string{
		"routerforge-support-1.zip",
		"routerforge-support-123456789.zip",
	}
	for _, name := range valid {
		if !validAdminSupportBundleName(name) {
			t.Fatalf("expected valid: %s", name)
		}
	}
	invalid := []string{
		"support.zip",
		"routerforge-support-1.tar.gz",
		"../routerforge-support-1.zip",
		"routerforge-support-/1.zip",
	}
	for _, name := range invalid {
		if validAdminSupportBundleName(name) {
			t.Fatalf("expected invalid: %s", name)
		}
	}
}
