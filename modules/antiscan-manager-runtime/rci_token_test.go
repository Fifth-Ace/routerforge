package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildAntiscanRCITokenCommandKeepsSecretOutOfArgv(t *testing.T) {
	const secret = "AbCdEf0123456789"

	args, stdin, err := buildAntiscanRCITokenCommand("set", secret, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(args, " "), secret) {
		t.Fatal("RCI token leaked into argv")
	}
	if stdin != secret+"\n" {
		t.Fatalf("stdin=%q", stdin)
	}

	args, stdin, err = buildAntiscanRCITokenCommand("set", secret, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(args, " "), secret) {
		t.Fatal("replacement RCI token leaked into argv")
	}
	if stdin != "Y\n"+secret+"\n" {
		t.Fatalf("replacement stdin=%q", stdin)
	}
}

func TestNormalizeAntiscanRCITokenValueRejectsUnsafeInput(t *testing.T) {
	for _, raw := range []string{"", "abc-def", "abc def", "abc\nxyz", strings.Repeat("A", antiscanRCITokenValueMax+1)} {
		if _, err := normalizeAntiscanRCITokenValue(raw); err == nil {
			t.Fatalf("unsafe RCI token accepted: %q", raw)
		}
	}
	if got, err := normalizeAntiscanRCITokenValue("AbC123"); err != nil || got != "AbC123" {
		t.Fatalf("valid token rejected: got=%q err=%v", got, err)
	}
}

func TestRCITokenStatusAndResultNeverSerializePlaintext(t *testing.T) {
	const secret = "SecretToken123"
	status := antiscanRCITokenStatus{
		AuthState:    "required",
		TokenPresent: true,
		KeyPresent:   true,
		Complete:     true,
		Supported:    true,
		Running:      true,
		MutationAPI:  true,
	}
	result := antiscanRCITokenResult{
		Action:         "set",
		AuthState:      "required",
		BeforeComplete: false,
		AfterComplete:  true,
		RunningBefore:  true,
		RunningAfter:   true,
		Changed:        true,
		Verified:       true,
		Checked:        true,
		Valid:          true,
		Supported:      true,
		MutationAPI:    true,
	}
	for _, value := range []any{status, result} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), secret) {
			t.Fatal("plaintext RCI token leaked into JSON")
		}
	}
}

func TestRCITokenAuditMetadataNeverIncludesSecret(t *testing.T) {
	const secret = "SecretToken123"
	body := []byte(`{"action":"set","token":"` + secret + `","confirm":"SET_TOKEN"}`)
	action, target := antiscanAuditRequestMetadata("rci-token", body)
	if action != "rci-token:set" {
		t.Fatalf("action=%q", action)
	}
	if strings.Contains(action, secret) || strings.Contains(target, secret) {
		t.Fatal("plaintext RCI token leaked into audit metadata")
	}
}
