package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func c4UseMemoryStore(t *testing.T, now time.Time) {
	t.Helper()
	oldRoot, oldPath, oldNow, oldBackup := v2TargetMemoryRoot, v2TargetMemoryPath, v2TargetMemoryNow, v2TargetMemoryBackup
	t.Cleanup(func() {
		v2TargetMemoryRoot, v2TargetMemoryPath = oldRoot, oldPath
		v2TargetMemoryNow, v2TargetMemoryBackup = oldNow, oldBackup
	})

	tmp := t.TempDir()
	v2TargetMemoryRoot = tmp
	v2TargetMemoryPath = filepath.Join(tmp, "target-memory.json")
	v2TargetMemoryNow = func() time.Time { return now }
	v2TargetMemoryBackup = func(string, string, string, []byte, os.FileMode) (string, error) {
		return "", nil
	}
}

func c4WorkingMemoryEntry(now time.Time, id, fingerprint string, verified, streak int, age time.Duration) v2TargetMemoryEntry {
	stamp := now.Add(-age).UTC().Format(time.RFC3339)
	return v2TargetMemoryEntry{
		Target: "example.com", Protocol: benchTransportHTTPS, IPFamily: "ipv4",
		CandidateID: id, CandidateName: id, CandidateSource: "builtin",
		Fingerprint: fingerprint, Environment: v2MemoryEnvironmentFingerprint(strings.Repeat("a", 64)),
		ConfigSHA256: strings.Repeat("a", 64),
		Args: []string{
			"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
			"--lua-desync=multisplit:pos=1,midsld",
		},
		ResultClass: "WORKING", SuccessRate: 1, CompleteRate: 1,
		VerifiedCount: verified, SuccessStreak: streak,
		LastVerified: stamp, LastWorking: stamp,
		InfrastructureOK: true, CleanupProven: true,
	}
}

func TestC4MemoryConfidenceMaturesWithIndependentEvidence(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	fresh := c4WorkingMemoryEntry(now, "fresh", strings.Repeat("1", 64), 1, 1, time.Hour)
	strong := c4WorkingMemoryEntry(now, "strong", strings.Repeat("2", 64), 2, 2, 2*time.Hour)
	trusted := c4WorkingMemoryEntry(now, "trusted", strings.Repeat("3", 64), 3, 3, 3*time.Hour)

	cases := []struct {
		entry      v2TargetMemoryEntry
		confidence string
	}{
		{fresh, v2MemoryConfidenceFresh},
		{strong, v2MemoryConfidenceStrong},
		{trusted, v2MemoryConfidenceTrusted},
	}
	for _, tc := range cases {
		decision := v2TargetMemoryReuseDecisionForEntry(tc.entry, now)
		if !decision.ReuseEligible || decision.Confidence != tc.confidence {
			t.Fatalf("id=%s decision=%+v want confidence=%s reusable", tc.entry.CandidateID, decision, tc.confidence)
		}
	}
}

func TestC4MemoryAgingAndUnstableGuard(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	stale := c4WorkingMemoryEntry(now, "stale", strings.Repeat("4", 64), 5, 5, v2MemoryWorkingMaxAge+time.Second)
	decision := v2TargetMemoryReuseDecisionForEntry(stale, now)
	if decision.ReuseEligible || decision.Confidence != v2MemoryConfidenceStale {
		t.Fatalf("stale working evidence must not be reusable: %+v", decision)
	}

	unstable := c4WorkingMemoryEntry(now, "unstable", strings.Repeat("5", 64), 1, 0, time.Hour)
	unstable.ResultClass = "UNSTABLE"
	unstable.SuccessRate = 0.5
	unstable.LastWorking = ""
	decision = v2TargetMemoryReuseDecisionForEntry(unstable, now)
	if decision.ReuseEligible || decision.Confidence != v2MemoryConfidenceProbation {
		t.Fatalf("single unstable observation must remain probation-only: %+v", decision)
	}

	unstable.VerifiedCount = 2
	decision = v2TargetMemoryReuseDecisionForEntry(unstable, now)
	if !decision.ReuseEligible || decision.Confidence != v2MemoryConfidenceProbation {
		t.Fatalf("recent repeated unstable evidence should be reusable as probation: %+v", decision)
	}

	unstable.LastVerified = now.Add(-v2MemoryUnstableMaxAge - time.Second).Format(time.RFC3339)
	decision = v2TargetMemoryReuseDecisionForEntry(unstable, now)
	if decision.ReuseEligible || decision.Confidence != v2MemoryConfidenceStale {
		t.Fatalf("old unstable evidence must age out: %+v", decision)
	}

	future := c4WorkingMemoryEntry(now, "future", strings.Repeat("6", 64), 3, 3, 0)
	future.LastVerified = now.Add(v2MemoryFutureSkew + time.Second).Format(time.RFC3339)
	decision = v2TargetMemoryReuseDecisionForEntry(future, now)
	if decision.ReuseEligible || decision.Confidence != v2MemoryConfidenceBlocked {
		t.Fatalf("future timestamp must be blocked: %+v", decision)
	}
}

func TestC4MemoryCandidatesRankConfidenceAndExposeMetadata(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	c4UseMemoryStore(t, now)

	configSHA := strings.Repeat("a", 64)
	doc := v2TargetMemoryDocument{Version: v2TargetMemoryVersion, Entries: []v2TargetMemoryEntry{
		c4WorkingMemoryEntry(now, "fresh", strings.Repeat("1", 64), 1, 1, time.Hour),
		c4WorkingMemoryEntry(now, "trusted", strings.Repeat("3", 64), 3, 3, 3*time.Hour),
		c4WorkingMemoryEntry(now, "strong", strings.Repeat("2", 64), 2, 2, 2*time.Hour),
	}}
	if err := writeV2TargetMemory(doc); err != nil {
		t.Fatal(err)
	}

	items, err := v2TargetMemoryCandidatesForTransport("example.com", configSHA, benchTransportHTTPS, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("candidate count=%d want=3", len(items))
	}
	want := []string{v2MemoryConfidenceTrusted, v2MemoryConfidenceStrong, v2MemoryConfidenceFresh}
	for i := range want {
		if items[i].MemoryConfidence != want[i] {
			t.Fatalf("candidate[%d] confidence=%s want=%s items=%+v", i, items[i].MemoryConfidence, want[i], items)
		}
		if items[i].MemoryVerifiedCount <= 0 || items[i].MemoryAgeSeconds < 0 {
			t.Fatalf("candidate[%d] missing memory metadata: %+v", i, items[i])
		}
	}
}

func TestC4MemoryEndpointPublishesPolicyWithoutMutatingDocumentVersion(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	c4UseMemoryStore(t, now)

	req := httptest.NewRequest(http.MethodGet, "/v1/v2/memory", nil)
	rr := httptest.NewRecorder()
	handleV2TargetMemory(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Version int                  `json:"version"`
		Policy  v2TargetMemoryPolicy `json:"policy"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Version != v2TargetMemoryVersion {
		t.Fatalf("memory document version changed: %d", body.Version)
	}
	if body.Policy.WorkingMaxAgeSeconds != int64(v2MemoryWorkingMaxAge/time.Second) ||
		body.Policy.UnstableMaxAgeSeconds != int64(v2MemoryUnstableMaxAge/time.Second) ||
		body.Policy.TrustedMinVerified != v2MemoryTrustedMinVerified {
		t.Fatalf("unexpected policy snapshot: %+v", body.Policy)
	}
}
