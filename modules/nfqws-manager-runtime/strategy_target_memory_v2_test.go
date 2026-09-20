package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestV2TargetMemoryRecordsPortableVerifiedEvidence(t *testing.T) {
	oldRoot, oldPath, oldNow, oldBackup := v2TargetMemoryRoot, v2TargetMemoryPath, v2TargetMemoryNow, v2TargetMemoryBackup
	defer func() {
		v2TargetMemoryRoot, v2TargetMemoryPath = oldRoot, oldPath
		v2TargetMemoryNow, v2TargetMemoryBackup = oldNow, oldBackup
	}()

	tmp := t.TempDir()
	v2TargetMemoryRoot = tmp
	v2TargetMemoryPath = filepath.Join(tmp, "target-memory.json")
	v2TargetMemoryNow = func() time.Time {
		return time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	}
	v2TargetMemoryBackup = func(string, string, string, []byte, os.FileMode) (string, error) {
		return "", nil
	}

	configSHA := strings.Repeat("a", 64)
	candidate := v2CandidateResult{
		CandidateID: "builtin-test", CandidateName: "Test", CandidateSource: "builtin",
		Args: []string{
			"--hostlist-domains=example.com",
			"--filter-tcp=443",
			"--filter-l7=tls",
			"--payload=tls_client_hello",
			"--lua-desync=multisplit:pos=1,midsld",
		},
		Successes: 2, SuccessRate: 1, CompleteRate: 1, ResultClass: "WORKING",
		CleanupProven: true, InfrastructureOK: true, MedianTTFBMS: 50, MedianThroughput: 100000,
	}
	if err := v2RecordSelectorEvidence("example.com", configSHA, []v2CandidateResult{candidate}); err != nil {
		t.Fatal(err)
	}
	doc, err := readV2TargetMemory()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Entries) != 1 {
		t.Fatalf("expected one memory entry, got %d", len(doc.Entries))
	}
	entry := doc.Entries[0]
	if entry.SuccessStreak != 1 || entry.FailureStreak != 0 || entry.VerifiedCount != 1 {
		t.Fatalf("unexpected streak/count state: %+v", entry)
	}
	if entry.LastWorking == "" || entry.Environment == "" {
		t.Fatalf("working/environment evidence missing: %+v", entry)
	}
	for _, arg := range entry.Args {
		if strings.HasPrefix(arg, "--hostlist") || strings.HasPrefix(arg, "--ipset") {
			t.Fatalf("selection-only arg persisted into portable memory: %q", arg)
		}
	}

	pool, err := v2TargetMemoryCandidates("example.com", configSHA, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(pool) != 1 || pool[0].Source != "memory" {
		t.Fatalf("verified memory candidate was not reusable: %+v", pool)
	}
}

func TestV2TargetMemoryIgnoresInfrastructureFailure(t *testing.T) {
	oldRoot, oldPath, oldBackup := v2TargetMemoryRoot, v2TargetMemoryPath, v2TargetMemoryBackup
	defer func() {
		v2TargetMemoryRoot, v2TargetMemoryPath, v2TargetMemoryBackup = oldRoot, oldPath, oldBackup
	}()

	tmp := t.TempDir()
	v2TargetMemoryRoot = tmp
	v2TargetMemoryPath = filepath.Join(tmp, "target-memory.json")
	v2TargetMemoryBackup = func(string, string, string, []byte, os.FileMode) (string, error) {
		return "", nil
	}

	candidate := v2CandidateResult{
		CandidateID: "broken", CandidateName: "broken", CandidateSource: "builtin",
		Args:        []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld"},
		ResultClass: "INCONCLUSIVE", CleanupProven: true, InfrastructureOK: false,
	}
	if err := v2RecordSelectorEvidence("example.com", strings.Repeat("b", 64), []v2CandidateResult{candidate}); err != nil {
		t.Fatal(err)
	}
	doc, err := readV2TargetMemory()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Entries) != 0 {
		t.Fatalf("infrastructure failure must not poison target memory: %+v", doc.Entries)
	}
}
