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
	if source.CandidateEligible {
		t.Fatal("fixture must exercise a file-bound production source, not a bench candidate")
	}
	if !v2ProductionSourceProfileEligible(source) {
		t.Fatalf("production source rejected: %+v", source.Reasons)
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
	if !v2ProfileMatchesTargetWithLists("example.com", profile, map[string]bool{"user.list": true}, map[string]bool{}) {
		t.Fatal("matching list should bind production profile")
	}
	if v2ProfileMatchesTargetWithLists("example.com", profile, map[string]bool{}, map[string]bool{}) {
		t.Fatal("unmatched list must not bind production profile")
	}
}
func TestV2ProfileMatchesTargetWithListsExcludeVetoesInclude(t *testing.T) {
	profile := analyzeBenchStrategyProfile(1, []string{
		"--hostlist=/opt/etc/nfqws2/lists/user.list",
		"--hostlist-exclude=/opt/etc/nfqws2/lists/exclude.list",
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	})
	if v2ProfileMatchesTargetWithLists("example.com", profile, map[string]bool{
		"user.list":    true,
		"exclude.list": true,
	}, map[string]bool{}) {
		t.Fatal("exclude-list match must veto a positive include-list match")
	}
}

func TestV2ProfileMatchesTargetWithListsExcludeOnlyAllowsNonExcludedTarget(t *testing.T) {
	profile := analyzeBenchStrategyProfile(1, []string{
		"--hostlist-exclude=/opt/etc/nfqws2/lists/exclude.list",
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	})
	if !v2ProfileMatchesTargetWithLists("example.com", profile, map[string]bool{}, map[string]bool{}) {
		t.Fatal("exclude-only profile must match a target that is not excluded")
	}
}

func TestV2ProfileMatchesTargetWithListsExcludeOnlyRejectsExcludedTarget(t *testing.T) {
	profile := analyzeBenchStrategyProfile(1, []string{
		"--hostlist-exclude=/opt/etc/nfqws2/lists/exclude.list",
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	})
	if v2ProfileMatchesTargetWithLists("example.com", profile, map[string]bool{"exclude.list": true}, map[string]bool{}) {
		t.Fatal("exclude-only profile must reject a target present in its exclude list")
	}
}
func TestV2ListContainsIPv4ExactAndCIDR(t *testing.T) {
	text := `
# comment
198.51.100.7
203.0.113.0/24
`
	if !v2ListContainsIPv4(text, "198.51.100.7") {
		t.Fatal("exact IPv4 entry did not match")
	}
	if !v2ListContainsIPv4(text, "203.0.113.42") {
		t.Fatal("CIDR IPv4 entry did not match")
	}
	if v2ListContainsIPv4(text, "192.0.2.1") {
		t.Fatal("unlisted IPv4 unexpectedly matched")
	}
}

func TestV2ProfileMatchesTargetWithListsIPSetUsesDestinationIPv4(t *testing.T) {
	profile := analyzeBenchStrategyProfile(1, []string{
		"--ipset=/opt/etc/nfqws2/lists/video.list",
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	})
	if !v2ProfileMatchesTargetWithLists(
		"video.example",
		profile,
		map[string]bool{},
		map[string]bool{"video.list": true},
	) {
		t.Fatal("ipset-bound profile must match by destination IPv4")
	}
	if v2ProfileMatchesTargetWithLists(
		"video.example",
		profile,
		map[string]bool{"video.list": true},
		map[string]bool{},
	) {
		t.Fatal("domain text must not satisfy an ipset selector")
	}
}

func TestV2ProfileMatchesTargetWithListsIPSetExcludeVetoesIPv4(t *testing.T) {
	profile := analyzeBenchStrategyProfile(1, []string{
		"--ipset-exclude=/opt/etc/nfqws2/lists/exclude.list",
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	})
	if v2ProfileMatchesTargetWithLists(
		"video.example",
		profile,
		map[string]bool{},
		map[string]bool{"exclude.list": true},
	) {
		t.Fatal("ipset-exclude match must veto the production profile")
	}
	if !v2ProfileMatchesTargetWithLists(
		"video.example",
		profile,
		map[string]bool{},
		map[string]bool{},
	) {
		t.Fatal("ipset-exclude-only profile must allow a non-excluded destination IPv4")
	}
}
