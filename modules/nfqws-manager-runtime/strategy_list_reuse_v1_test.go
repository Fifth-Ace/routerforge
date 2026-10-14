package main

import (
	"strings"
	"testing"
)

func TestNF4BOnlyExactHostlistProfile(t *testing.T) {
	p := analyzeBenchStrategyProfile(4, []string{"--hostlist=/opt/etc/nfqws2/lists/user.list", "--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld"})
	fp := v2CandidateTechniqueFingerprint(p.Args)
	if name, ok := nf4bEligibleListProfile(p, fp); !ok || name != "user.list" {
		t.Fatalf("got %s %t", name, ok)
	}
	if _, ok := nf4bEligibleListProfile(p, "wrong"); ok {
		t.Fatal("wrong technique was allowed")
	}
}

func TestNF4BExcludesAmbiguousSelectors(t *testing.T) {
	basic := []string{"--hostlist=/opt/etc/nfqws2/lists/user.list", "--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld"}
	variants := [][]string{
		append(append([]string{}, basic...), "--hostlist-exclude=/opt/etc/nfqws2/lists/exclude.list"),
		append(append([]string{}, basic...), "--hostlist=/opt/etc/nfqws2/lists/other.list"),
		append(append([]string{}, basic...), "--ipset=/opt/etc/nfqws2/lists/ipset.list"),
		append(append([]string{}, basic...), "--hostlist-domains=another.com"),
	}
	for _, args := range variants {
		p := analyzeBenchStrategyProfile(4, args)
		if name, ok := nf4bEligibleListProfile(p, v2CandidateTechniqueFingerprint(p.Args)); ok {
			t.Fatalf("ambiguous selector %s", name)
		}
	}
}

func TestNF4BRejectsPathTricks(t *testing.T) {
	for _, path := range []string{"/tmp/user.list", "/opt/etc/nfqws2/lists/../lists/user.list", "user.list"} {
		p := analyzeBenchStrategyProfile(4, []string{"--hostlist=" + path, "--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld"})
		if _, ok := nf4bEligibleListProfile(p, v2CandidateTechniqueFingerprint(p.Args)); ok {
			t.Fatal(path)
		}
	}
}

func TestNF4BAppendDomainPreservesOtherLines(t *testing.T) {
	got, err := nf4bAppendDomain([]byte("# managed\nexisting.example\n"), "new.example")
	if err != nil || string(got) != "# managed\nexisting.example\nnew.example\n" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestNF4BAppendDomainRejectsDuplicates(t *testing.T) {
	if _, err := nf4bAppendDomain([]byte("existing.example\n"), "existing.example"); err == nil {
		t.Fatal("duplicate allowed")
	}
}

func TestNF4BAppendDomainPreservesCRLF(t *testing.T) {
	out, err := nf4bAppendDomain([]byte("existing.example\r\n"), "new.example")
	if err != nil || !strings.HasSuffix(string(out), "new.example\r\n") {
		t.Fatalf("%q %v", out, err)
	}
}
