package main

import (
	"strings"
	"testing"
)

func TestBuildAutoTuneCandidateConfigReplacesOneCompleteCustomProfile(t *testing.T) {
	config := `NFQWS_ARGS="--filter-tcp=443 --lua-desync=keep"
NFQWS_ARGS_CUSTOM="--hostlist-domains=googlevideo.com
                   --filter-tcp=443 --filter-l7=tls
                   --payload=tls_client_hello --lua-desync=old
                   --new
                   --hostlist-domains=mobatek.net --filter-tcp=443
                   --filter-l7=tls --payload=tls_client_hello
                   --lua-desync=multisplit:pos=2"
OTHER_SETTING="untouched"
`
	source := []string{
		"--hostlist-domains=mobatek.net",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=2",
	}
	candidateArgs := []string{
		"--hostlist-domains=www.googlevideo.com",
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=2",
	}
	got, err := buildAutoTuneCandidateConfig(config, source, candidateArgs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, strings.Join(candidateArgs, " ")) {
		t.Fatalf("candidate profile missing:\n%s", got)
	}
	if strings.Contains(got, "--hostlist-domains=mobatek.net") {
		t.Fatalf("source profile was not replaced:\n%s", got)
	}
	if !strings.Contains(got, `NFQWS_ARGS="--filter-tcp=443 --lua-desync=keep"`) ||
		!strings.Contains(got, `OTHER_SETTING="untouched"`) {
		t.Fatalf("unrelated config changed:\n%s", got)
	}
	gotAgain, err := buildAutoTuneCandidateConfig(config, source, candidateArgs)
	if err != nil || gotAgain != got {
		t.Fatalf("builder is not deterministic: err=%v", err)
	}
}

func TestBuildAutoTuneCandidateConfigRejectsDuplicateSourceProfile(t *testing.T) {
	profile := "--hostlist-domains=a.example --filter-tcp=443 --filter-l7=tls --payload=tls_client_hello --lua-desync=x"
	config := "NFQWS_ARGS_CUSTOM=\"" + profile + " --new " + profile + "\"\n"
	source := strings.Fields(profile)
	if _, err := buildAutoTuneCandidateConfig(config, source, source); err == nil {
		t.Fatal("duplicate source profile was accepted")
	}
}

func TestBuildAutoTuneCandidateConfigRequiresCompleteProfileBoundary(t *testing.T) {
	config := `NFQWS_ARGS_CUSTOM="--hostlist-domains=a.example --filter-tcp=443 --filter-l7=tls --payload=tls_client_hello --lua-desync=x"
`
	source := []string{"--hostlist-domains=a.example", "--filter-tcp=443"}
	if _, err := buildAutoTuneCandidateConfig(config, source, source); err == nil {
		t.Fatal("partial profile match was accepted")
	}
}

func TestQuotedAssignmentValueRequiresUniqueCustomAssignment(t *testing.T) {
	config := "NFQWS_ARGS_CUSTOM=\"a\"\nNFQWS_ARGS_CUSTOM=\"b\"\n"
	if _, _, _, err := quotedAssignmentValue(config, "NFQWS_ARGS_CUSTOM"); err == nil {
		t.Fatal("duplicate assignment was accepted")
	}
}
