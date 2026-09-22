package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunBenchCandidateStartCommandDoesNotWaitForDaemonDescendant(t *testing.T) {
	script := filepath.Join(t.TempDir(), "daemonish.sh")
	content := "#!/bin/sh\n(sleep 3) &\nexit 0\n"
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := runBenchCandidateStartCommand(ctx, script); err != nil {
		t.Fatalf("candidate start helper waited on daemon descendant: %v", err)
	}
	if elapsed := time.Since(started); elapsed >= 750*time.Millisecond {
		t.Fatalf("candidate start helper took %v; want parent-process completion without waiting for descendant", elapsed)
	}
}

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

func TestBenchStartCandidateBlocksActiveManagementSessionBeforeLaunch(t *testing.T) {
	previous := benchManagementSessionInventory
	benchManagementSessionInventory = func() ([]int, bool) {
		return []int{222}, true
	}
	t.Cleanup(func() {
		benchManagementSessionInventory = previous
	})

	ops := &benchSystemOps{candidateBinary: "/definitely/must-not-run/nfqws2"}
	spec := benchTransactionSpec{
		SessionID:       "session-1234",
		DestinationIPv4: "1.1.1.1",
		LocalPort:       43123,
		Queue:           30000,
	}
	err := ops.StartCandidate(context.Background(), spec)
	if err == nil || !strings.Contains(err.Error(), "active management SSH session") {
		t.Fatalf("StartCandidate error=%v, want management-session blocker", err)
	}
}

func TestBenchStartCandidateBlocksUnprovenManagementInventory(t *testing.T) {
	previous := benchManagementSessionInventory
	benchManagementSessionInventory = func() ([]int, bool) {
		return nil, false
	}
	t.Cleanup(func() {
		benchManagementSessionInventory = previous
	})

	ops := &benchSystemOps{candidateBinary: "/definitely/must-not-run/nfqws2"}
	err := ops.StartCandidate(context.Background(), benchTransactionSpec{})
	if err == nil || !strings.Contains(err.Error(), "inventory is not proven") {
		t.Fatalf("StartCandidate error=%v, want inventory blocker", err)
	}
}
