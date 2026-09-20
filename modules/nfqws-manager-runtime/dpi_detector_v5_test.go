package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDPIDetectorV5NormalizeTests(t *testing.T) {
	got, err := normalizeDPIDetectorV5Tests("660210")
	if err != nil {
		t.Fatal(err)
	}
	if got != "0126" {
		t.Fatalf("tests=%q want=0126", got)
	}
	for _, bad := range []string{"", "7", "27", "2x"} {
		if _, err := normalizeDPIDetectorV5Tests(bad); err == nil {
			t.Fatalf("bad tests %q accepted", bad)
		}
	}
}

func TestDPIDetectorV5ValidateAndBuildArgs(t *testing.T) {
	sha := strings.Repeat("a", 64)
	dir := t.TempDir()
	domains := filepath.Join(dir, "domains.txt")
	tcp16 := filepath.Join(dir, "tcp16.json")
	if err := os.WriteFile(domains, []byte("example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tcp16, []byte("[]\n"), 0600); err != nil {
		t.Fatal(err)
	}

	validated, err := validateDPIDetectorV5Request(dpiDetectorV5RunRequest{
		Tests:                "6230",
		Domains:              []string{"example.com", "EXAMPLE.com.", "openai.com"},
		DomainsFile:          domains,
		TCP16File:            tcp16,
		Concurrency:          20,
		Language:             "ru",
		Profile:              "ru",
		Interface:            "eth2.1",
		Proxy:                "socks5://127.0.0.1:1080",
		Fingerprint:          "chrome146",
		Burst:                7,
		BurstTimeout:         9,
		BurstProfiles:        "rustls,chrome146",
		BurstTLS:             "1.3",
		BurstALPN:            "h2",
		Trace:                true,
		ExpectedConfigSHA256: sha,
		Confirm:              dpiDetectorV5Confirm,
	})
	if err != nil {
		t.Fatal(err)
	}
	if validated.Tests != "0236" {
		t.Fatalf("tests=%q", validated.Tests)
	}
	if !reflect.DeepEqual(validated.Domains, []string{"example.com", "openai.com"}) {
		t.Fatalf("domains=%q", validated.Domains)
	}

	args := buildDPIDetectorV5Args(validated, "/tmp/trace.txt")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--json", "-t 0236", "-l ru", "--profile ru", "-c 20",
		"--iface eth2.1", "--proxy socks5://127.0.0.1:1080",
		"--fingerprint chrome146", "-d example.com", "-d openai.com",
		"--domains " + domains, "--tcp16 " + tcp16,
		"--burst 7", "--burst-timeout 9", "--burst-profiles rustls,chrome146",
		"--burst-tls 1.3", "--burst-alpn h2", "--trace /tmp/trace.txt",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
	for _, forbidden := range []string{"--batch", " -o ", "--output"} {
		if strings.Contains(" "+joined+" ", forbidden) {
			t.Fatalf("legacy/output argument %q leaked into %q", forbidden, joined)
		}
	}
}

func TestDPIDetectorV5DefaultsAndVersion(t *testing.T) {
	sha := strings.Repeat("b", 64)
	validated, err := validateDPIDetectorV5Request(dpiDetectorV5RunRequest{
		Tests:                "2",
		Domains:              []string{"example.com"},
		ExpectedConfigSHA256: sha,
		Confirm:              dpiDetectorV5Confirm,
	})
	if err != nil {
		t.Fatal(err)
	}
	if validated.Concurrency != 20 || validated.Language != "ru" || validated.Profile != "ru" || validated.Burst != 5 || validated.BurstTimeout != 8 || validated.BurstTLS != "1.3+1.2" || validated.BurstALPN != "h2" {
		t.Fatalf("unexpected defaults: %+v", validated)
	}
	if !dpiDetectorV5VersionOK("dpi-detector 5.0.0-alpha.19") || !dpiDetectorV5VersionOK("v5.1.0") {
		t.Fatal("v5 version rejected")
	}
	if dpiDetectorV5VersionOK("dpi-detector 4.2.4") || dpiDetectorV5VersionOK("unknown") {
		t.Fatal("legacy/unknown version accepted")
	}
}

func TestDPIDetectorV5FingerprintsMatchUpstreamSet(t *testing.T) {
	got := dpiDetectorV5FingerprintList()
	if len(got) != 20 {
		t.Fatalf("fingerprints=%d want=20: %q", len(got), got)
	}
	for _, required := range []string{"rustls", "chrome146", "firefox147", "safari260", "tor145"} {
		found := false
		for _, value := range got {
			if value == required {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("fingerprint %q missing from %q", required, got)
		}
	}
}
