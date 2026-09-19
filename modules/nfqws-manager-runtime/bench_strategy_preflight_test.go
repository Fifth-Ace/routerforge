package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestFindBenchStrategyProfileEligibleOnly(t *testing.T) {
	inventory := benchStrategyInventory{
		Profiles: []benchStrategyProfile{
			{Index: 0, CandidateEligible: true},
			{Index: 1, CandidateEligible: false},
		},
	}
	if got, err := findBenchStrategyProfile(inventory, 0); err != nil || got.Index != 0 {
		t.Fatalf("eligible profile rejected: got=%+v err=%v", got, err)
	}
	if _, err := findBenchStrategyProfile(inventory, 1); err == nil {
		t.Fatal("ineligible profile accepted")
	}
	if _, err := findBenchStrategyProfile(inventory, 7); err == nil {
		t.Fatal("missing profile accepted")
	}
}

func TestBuildBenchStrategyCandidateArgsPreservesLiveBaseAndProfile(t *testing.T) {
	spec := benchTransactionSpec{SessionID: "session-1234", Queue: 30000}
	inventory := benchStrategyInventory{
		BaseDependenciesProven: true,
		BaseArgs: []string{
			"--lua-init=@/opt/etc/nfqws2/lua/zapret-lib.lua",
			"--blob=tls_clienthello:@/opt/etc/nfqws2/blobs/tls_clienthello.bin",
		},
	}
	profile := benchStrategyProfile{
		Index:             0,
		CandidateEligible: true,
		Args: []string{
			"--hostlist-domains=googlevideo.com",
			"--filter-tcp=443",
			"--filter-l7=tls",
			"--payload=tls_client_hello",
			"--lua-desync=multisplit:pos=2",
		},
	}
	got, err := buildBenchStrategyCandidateArgs(spec, inventory, profile)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, " ")
	for _, want := range []string{
		"--qnum=30000",
		"--fwmark=0x40000000",
		"--lua-init=@/opt/etc/nfqws2/lua/zapret-lib.lua",
		"--blob=tls_clienthello:@/opt/etc/nfqws2/blobs/tls_clienthello.bin",
		"--hostlist-domains=googlevideo.com",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=2",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("candidate argv missing %q: %q", want, joined)
		}
	}
}

func TestBuildBenchStrategyCandidateArgsRejectsUnprovenBase(t *testing.T) {
	_, err := buildBenchStrategyCandidateArgs(
		benchTransactionSpec{SessionID: "session-1234", Queue: 30000},
		benchStrategyInventory{BaseDependenciesProven: false},
		benchStrategyProfile{Index: 0, CandidateEligible: true},
	)
	if err == nil {
		t.Fatal("unproven base dependencies accepted")
	}
}

func TestValidateBenchStrategyPreflightRequest(t *testing.T) {
	good := benchStrategyPreflightRequest{
		ProfileIndex:         0,
		ExpectedConfigSHA256: strings.Repeat("a", 64),
		Confirm:              benchStrategyPreflightConfirm,
	}
	if err := validateBenchStrategyPreflightRequest(good); err != nil {
		t.Fatalf("good request rejected: %v", err)
	}
	bad := good
	bad.ProfileIndex = -1
	if err := validateBenchStrategyPreflightRequest(bad); err == nil {
		t.Fatal("negative profile accepted")
	}
}

func TestCandidateArgSequenceKeepsProfileExact(t *testing.T) {
	profileArgs := []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello"}
	inventory := benchStrategyInventory{
		BaseDependenciesProven: true,
		BaseArgs:               []string{"--blob=tls_clienthello:@/x/tls.bin"},
	}
	profile := benchStrategyProfile{Index: 2, CandidateEligible: true, Args: profileArgs}
	got, err := buildBenchStrategyCandidateArgs(
		benchTransactionSpec{SessionID: "session-1234", Queue: 30000},
		inventory,
		profile,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[len(got)-len(profileArgs):], profileArgs) {
		t.Fatalf("profile tail changed: got=%v want=%v", got, profileArgs)
	}
}
