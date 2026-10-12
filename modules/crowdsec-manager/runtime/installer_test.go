package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func syntheticArchive(t *testing.T, elfMachine byte, filename string) ([]byte, string) {
	t.Helper()
	content := make([]byte, 2048)
	copy(content, []byte("\x7fELF"))
	content[4], content[5], content[18] = 2, 1, elfMachine
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: filename, Mode: 0755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(digest[:])
}

func TestSyntheticVerifiedARM64Asset(t *testing.T) {
	blob, sha := syntheticArchive(t, 183, "dist/crowdsec-firewall-bouncer")
	got, err := extractVerifiedBouncer(blob, sha)
	if err != nil {
		t.Fatalf("expected verified ELF: %v", err)
	}
	if len(got) != 2048 || got[18] != 183 {
		t.Fatal("unexpected extracted asset")
	}
}

func TestSyntheticSHA256Mismatch(t *testing.T) {
	blob, _ := syntheticArchive(t, 183, "cs-firewall-bouncer")
	_, err := extractVerifiedBouncer(blob, strings.Repeat("0", 64))
	if err == nil || !strings.Contains(err.Error(), "SHA256") {
		t.Fatalf("missing SHA guard: %v", err)
	}
}

func TestSyntheticWrongArchitecture(t *testing.T) {
	blob, sha := syntheticArchive(t, 62, "cs-firewall-bouncer")
	_, err := extractVerifiedBouncer(blob, sha)
	if err == nil || !strings.Contains(err.Error(), "architecture") {
		t.Fatalf("wrong architecture accepted: %v", err)
	}
}

func TestSyntheticMissingBinary(t *testing.T) {
	blob, sha := syntheticArchive(t, 183, "README")
	_, err := extractVerifiedBouncer(blob, sha)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected archive accepted: %v", err)
	}
}

func TestSyntheticExclusiveWrite(t *testing.T) {
	file := filepath.Join(t.TempDir(), "existing")
	if err := writeExclusive(file, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusive(file, []byte("changed"), 0600); err == nil {
		t.Fatal("existing file overwritten")
	}
	got, err := os.ReadFile(file)
	if err != nil || string(got) != "original" {
		t.Fatal("existing file changed")
	}
}

func TestSyntheticPlatformContract(t *testing.T) {
	p, supported := installerPlatform()
	if p == "" {
		t.Fatal("empty platform")
	}
	if runtime.GOARCH == "arm64" && (!supported || p != "aarch64-3.10") {
		t.Fatalf("unexpected arm64 platform: %s", p)
	}
	if (runtime.GOARCH == "mips" || runtime.GOARCH == "mipsle") && supported {
		t.Fatal("unverified MIPS marked supported")
	}
}
