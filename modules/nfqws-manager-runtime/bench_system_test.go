package main

import (
	"strings"
	"testing"
)

func TestBenchCandidateArgsAreMinimalAndIsolated(t *testing.T) {
	spec := benchTransactionSpec{
		SessionID:       "session-1234",
		DestinationIPv4: "203.0.113.10",
		LocalPort:       43123,
		Queue:           30000,
	}
	args := benchCandidateArgs(spec)
	joined := strings.Join(args, " ")

	for _, want := range []string{
		"--daemon",
		"--pidfile=/tmp/routerforge-bench-session-1234.pid",
		"--user=nobody",
		"--qnum=30000",
		"--fwmark=0x40000000",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("candidate args=%q missing %q", joined, want)
		}
	}
	for _, forbidden := range []string{"--lua-init", "--hostlist", "--ipset", "--lua-desync"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("candidate args unexpectedly contain %q: %q", forbidden, joined)
		}
	}
}

func TestValidateBenchSmokeRequest(t *testing.T) {
	good := benchSmokeRequest{
		DestinationIPv4:      "1.1.1.1",
		ExpectedConfigSHA256: strings.Repeat("a", 64),
		Confirm:              benchSmokeConfirm,
	}
	if err := validateBenchSmokeRequest(good); err != nil {
		t.Fatalf("good request rejected: %v", err)
	}

	badPrivate := good
	badPrivate.DestinationIPv4 = "192.168.1.1"
	if err := validateBenchSmokeRequest(badPrivate); err == nil {
		t.Fatal("private destination accepted")
	}

	badConfirm := good
	badConfirm.Confirm = "YES"
	if err := validateBenchSmokeRequest(badConfirm); err == nil {
		t.Fatal("wrong confirmation accepted")
	}
}

func TestBenchCandidatePIDFileIsSessionScoped(t *testing.T) {
	got := benchCandidatePIDFile("rf-0123456789abcdef")
	want := "/tmp/routerforge-bench-rf-0123456789abcdef.pid"
	if got != want {
		t.Fatalf("pidfile=%q want=%q", got, want)
	}
}
