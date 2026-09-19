package main

import (
	"reflect"
	"testing"
)

func TestSplitBenchStrategyArgvUsesLiveRuntimeShape(t *testing.T) {
	argv := []string{
		"/opt/usr/bin/nfqws2",
		"--daemon",
		"--pidfile=/opt/var/run/nfqws2.pid",
		"--user=nobody",
		"--qnum=300",
		"--lua-init=@/opt/etc/nfqws2/lua/zapret-lib.lua",
		"--blob=tls_clienthello:@/opt/etc/nfqws2/blobs/tls_clienthello.bin",
		"--hostlist-domains=mobatek.net",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=2",
		"--new",
		"--filter-udp=443",
		"--filter-l7=quic",
		"--payload=quic_initial",
	}

	base, profiles := splitBenchStrategyArgv(argv)
	wantBase := []string{
		"--lua-init=@/opt/etc/nfqws2/lua/zapret-lib.lua",
		"--blob=tls_clienthello:@/opt/etc/nfqws2/blobs/tls_clienthello.bin",
	}
	if !reflect.DeepEqual(base, wantBase) {
		t.Fatalf("base=%v want=%v", base, wantBase)
	}
	if len(profiles) != 2 {
		t.Fatalf("profiles=%d want=2", len(profiles))
	}
}

func TestAnalyzeBenchStrategyProfileAllowsDomainLiteralTLSProfile(t *testing.T) {
	got := analyzeBenchStrategyProfile(0, []string{
		"--hostlist-domains=mobatek.net",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=fake:blob=fake_default_tls",
		"--lua-desync=multisplit:pos=2",
	})
	if !got.CandidateEligible {
		t.Fatalf("profile unexpectedly ineligible: %+v", got)
	}
	if got.DesyncCount != 2 {
		t.Fatalf("desync_count=%d want=2", got.DesyncCount)
	}
	if !reflect.DeepEqual(got.HostlistDomains, []string{"mobatek.net"}) {
		t.Fatalf("domains=%v", got.HostlistDomains)
	}
}

func TestAnalyzeBenchStrategyProfileRejectsFileBoundProfile(t *testing.T) {
	got := analyzeBenchStrategyProfile(1, []string{
		"--hostlist=/opt/etc/nfqws2/lists/user.list",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1",
	})
	if got.CandidateEligible {
		t.Fatalf("file-bound profile accepted: %+v", got)
	}
	if len(got.FileBoundFilters) != 1 {
		t.Fatalf("file_bound_filters=%v", got.FileBoundFilters)
	}
}

func TestAnalyzeBenchStrategyProfileRejectsUDP(t *testing.T) {
	got := analyzeBenchStrategyProfile(2, []string{
		"--filter-udp=443",
		"--filter-l7=quic",
		"--payload=quic_initial",
		"--lua-desync=fake:repeats=6",
	})
	if got.CandidateEligible {
		t.Fatalf("udp profile accepted: %+v", got)
	}
}

func TestBenchPortListContains(t *testing.T) {
	if !benchPortListContains([]string{"80", "443", "1984"}, 443) {
		t.Fatal("exact 443 not detected")
	}
	if !benchPortListContains([]string{"400-500"}, 443) {
		t.Fatal("range containing 443 not detected")
	}
	if benchPortListContains([]string{"1443"}, 443) {
		t.Fatal("substring port false positive")
	}
}

func TestParseBenchStrategyTag(t *testing.T) {
	if got, ok := parseBenchStrategyTag("--lua-desync=fake:repeats=2:strategy=7"); !ok || got != 7 {
		t.Fatalf("tag=%d ok=%v", got, ok)
	}
	if _, ok := parseBenchStrategyTag("--lua-desync=multisplit:pos=2"); ok {
		t.Fatal("unexpected strategy tag")
	}
}
