package main

import (
	"strings"
	"testing"
)

func nf4TestConfig() string {
	return `NFQWS_ARGS="--filter-tcp=443 --lua-desync=keep"
NFQWS_ARGS_CUSTOM="--hostlist-domains=existing.example
--filter-tcp=443 --filter-l7=tls
--payload=tls_client_hello --lua-desync=multisplit:pos=2"
OTHER_SETTING="untouched"
`
}

func nf4TestCandidate() []string {
	return []string{
		"--hostlist-domains=new.example",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=2",
	}
}

func TestNF4AAppendPreservesExistingProfile(t *testing.T) {
	base := nf4TestConfig()
	candidate := nf4TestCandidate()
	got, err := nf4BuildPlacementConfig(base, true, nil, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "--hostlist-domains=existing.example") ||
		!strings.Contains(got, "--hostlist-domains=new.example") ||
		!strings.Contains(got, `OTHER_SETTING="untouched"`) ||
		strings.Count(got, "--new") != 1 {
		t.Fatalf("append changed existing settings or missed target: %s", got)
	}
	gotAgain, err := nf4BuildPlacementConfig(base, true, nil, candidate)
	if err != nil || gotAgain != got {
		t.Fatalf("append preview/apply is not deterministic: err=%v", err)
	}
	if got == base || smartApplySHA256([]byte(got)) == smartApplySHA256([]byte(base)) {
		t.Fatal("append produced no change")
	}
}

func TestNF4AReplaceUsesExactSource(t *testing.T) {
	base := nf4TestConfig()
	source := []string{
		"--hostlist-domains=existing.example",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=2",
	}
	got, err := nf4BuildPlacementConfig(base, false, source, nf4TestCandidate())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "existing.example") || !strings.Contains(got, "new.example") ||
		strings.Contains(got, "--new") || !strings.Contains(got, `OTHER_SETTING="untouched"`) {
		t.Fatalf("replacement scope mismatch: %s", got)
	}
}

func TestNF4ARejectsMixedOrUnboundPlacement(t *testing.T) {
	base := nf4TestConfig()
	for _, tc := range []struct {
		name   string
		append bool
		source []string
		args   []string
	}{
		{"append-with-source", true, []string{"--lua-desync=fake"}, nf4TestCandidate()},
		{"replace-without-source", false, nil, nf4TestCandidate()},
		{"empty-append-args", true, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := nf4BuildPlacementConfig(base, tc.append, tc.source, tc.args); err == nil {
				t.Fatalf("invalid placement accepted: %s", got)
			}
		})
	}
}

func TestNF4AAppendRejectsAmbiguousCustomConfig(t *testing.T) {
	base := nf4TestConfig() + "NFQWS_ARGS_CUSTOM=\"--new\"\n"
	if _, err := nf4BuildPlacementConfig(base, true, nil, nf4TestCandidate()); err == nil {
		t.Fatal("duplicate custom assignment accepted")
	}
}
