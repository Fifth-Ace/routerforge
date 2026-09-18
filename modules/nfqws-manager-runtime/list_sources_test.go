package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSafeListSourceName(t *testing.T) {
	good := []string{"user.list", "exclude-extra.list", "ipset_cloudflare.list"}
	for _, name := range good {
		if !safeListSourceName(name) {
			t.Fatalf("expected safe list source name: %s", name)
		}
	}
	bad := []string{"", ".hidden.list", "../escape.list", "nested/list.list", "source", "source.list-opkg", "bad name.list"}
	for _, name := range bad {
		if safeListSourceName(name) {
			t.Fatalf("expected unsafe list source name: %s", name)
		}
	}
}

func TestValidateListSourcePayload(t *testing.T) {
	data := []byte("example.com\nsub.example.net\n")
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	if err := validateListSourcePayload("user.list", data, hash); err != nil {
		t.Fatalf("valid list source rejected: %v", err)
	}
	if err := validateListSourcePayload("user.list", data, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected SHA mismatch")
	}
	if err := validateListSourcePayload("user.list-opkg", data, hash); err == nil {
		t.Fatal("expected unsafe extension rejection")
	}
	if err := validateListSourcePayload("user.list", nil, hash); err == nil {
		t.Fatal("expected empty payload rejection")
	}
}

func TestValidateListSourcePayloadLimit(t *testing.T) {
	data := make([]byte, listFileMaxBytes+1)
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if err := validateListSourcePayload("large.list", data, hash); err == nil {
		t.Fatal("expected list source size limit rejection")
	}
}
