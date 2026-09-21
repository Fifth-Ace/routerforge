package main

import (
	"strings"
	"testing"
)

func TestV2BindCandidateToSourceProfilePreservesSourceSelection(t *testing.T) {
	source := analyzeBenchStrategyProfile(3, []string{
		"--hostlist=/opt/etc/nfqws2/lists/user.list",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	})
	if !source.CandidateEligible {
		t.Fatalf("source not eligible: %+v", source.Reasons)
	}
	tested := []string{
		"--hostlist-domains=example.com",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=fakedsplit:pos=midsld",
	}
	bound, err := v2BindCandidateToSourceProfile(source, tested)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(bound, " ")
	if !strings.Contains(text, "--hostlist=/opt/etc/nfqws2/lists/user.list") {
		t.Fatalf("source selection lost: %q", text)
	}
	if strings.Contains(text, "--hostlist-domains=example.com") {
		t.Fatalf("bench-only target selector leaked into production candidate: %q", text)
	}
	if !strings.Contains(text, "--lua-desync=fakedsplit:pos=midsld") {
		t.Fatalf("candidate technique lost: %q", text)
	}
}

func TestV2ProfileMatchesTargetWithLists(t *testing.T) {
	profile := analyzeBenchStrategyProfile(1, []string{
		"--hostlist=/opt/etc/nfqws2/lists/user.list",
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	})
	if !v2ProfileMatchesTargetWithLists("example.com", profile, map[string]bool{"user.list": true}) {
		t.Fatal("matching list should bind production profile")
	}
	if v2ProfileMatchesTargetWithLists("example.com", profile, map[string]bool{}) {
		t.Fatal("unmatched list must not bind production profile")
	}
}
