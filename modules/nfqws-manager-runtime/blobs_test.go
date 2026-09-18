package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSafeBlobName(t *testing.T) {
	good := []string{
		"tls_clienthello.bin",
		"ACTIVE_DISCORD_UDP.bin",
		"quic-initial.v2.bin",
	}
	for _, name := range good {
		if !safeBlobName(name) {
			t.Fatalf("expected safe blob name: %s", name)
		}
	}
	bad := []string{
		"",
		".hidden.bin",
		"../escape.bin",
		"nested/blob.bin",
		"blob",
		"blob.txt",
		"bad name.bin",
	}
	for _, name := range bad {
		if safeBlobName(name) {
			t.Fatalf("expected unsafe blob name: %s", name)
		}
	}
}

func TestValidateBlobPayload(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0xff}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	if err := validateBlobPayload("sample.bin", data, hash); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	if err := validateBlobPayload("sample.bin", data, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected SHA mismatch")
	}
	if err := validateBlobPayload("sample.txt", data, hash); err == nil {
		t.Fatal("expected unsafe extension rejection")
	}
	if err := validateBlobPayload("sample.bin", nil, hash); err == nil {
		t.Fatal("expected empty payload rejection")
	}
}

func TestValidateBlobPayloadLimit(t *testing.T) {
	data := make([]byte, blobMaxBytes+1)
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if err := validateBlobPayload("large.bin", data, hash); err == nil {
		t.Fatal("expected blob size limit rejection")
	}
}
