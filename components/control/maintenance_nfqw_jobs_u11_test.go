package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdminNFQWSJobValidationU11Kinds(t *testing.T) {
	cases := []struct {
		job  adminNFQWSJob
		kind string
	}{
		{adminNFQWSJob{ID: "detect", Kind: "detect-target", Target: "Example.COM.", IntervalMinutes: 15}, "detect-target"},
		{adminNFQWSJob{ID: "health", Kind: "strategy-health-recheck", Target: "Example.COM.", IntervalMinutes: 15}, "strategy-health-recheck"},
		{adminNFQWSJob{ID: "tcp16", Kind: "tcp16-revalidate", Target: "he-01", IntervalMinutes: 30}, "tcp16-revalidate"},
		{adminNFQWSJob{ID: "queue", Kind: "nfqueue-health", Target: "ignored", IntervalMinutes: 30}, "nfqueue-health"},
		{adminNFQWSJob{ID: "backup", Kind: "backup-cleanup", Target: "ignored", IntervalMinutes: 60}, "backup-cleanup"},
	}
	for _, tc := range cases {
		got, err := normalizeAdminNFQWSJob(tc.job)
		if err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		if got.Kind != tc.kind {
			t.Fatalf("kind=%q want=%q", got.Kind, tc.kind)
		}
	}
	tcp, _ := normalizeAdminNFQWSJob(adminNFQWSJob{ID: "tcp16", Kind: "tcp16-revalidate", Target: "he-01", IntervalMinutes: 30})
	if tcp.Target != "HE-01" {
		t.Fatalf("tcp16 target=%q", tcp.Target)
	}
	queue, _ := normalizeAdminNFQWSJob(adminNFQWSJob{ID: "queue", Kind: "nfqueue-health", Target: "ignored", IntervalMinutes: 30})
	if queue.Target != "" {
		t.Fatalf("nfqueue target=%q want empty", queue.Target)
	}
	if _, err := normalizeAdminNFQWSJob(adminNFQWSJob{ID: "refresh", Kind: "list-refresh", Target: "user.list", IntervalMinutes: 60}); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("list-refresh should fail closed until a refresh-capable source provider exists: %v", err)
	}
}

func TestAdminNFQWSJobOutcomeClassification(t *testing.T) {
	if got := classifyAdminNFQWSJobOutcome(adminNFQWSJob{Kind: "detect-target"}, 200, `{"classification":"inconclusive"}`, nil); got != adminNFQWSJobOutcomeInconclusive {
		t.Fatalf("detect inconclusive=%q", got)
	}
	if got := classifyAdminNFQWSJobOutcome(adminNFQWSJob{Kind: "tcp16-revalidate"}, 200, `{"ok":false,"alive_count":0}`, nil); got != adminNFQWSJobOutcomeInconclusive {
		t.Fatalf("tcp16 inconclusive=%q", got)
	}
	if got := classifyAdminNFQWSJobOutcome(adminNFQWSJob{Kind: "nfqueue-health"}, 200, `{"state":"UNHEALTHY"}`, nil); got != adminNFQWSJobOutcomeUnhealthy {
		t.Fatalf("nfqueue unhealthy=%q", got)
	}
	if got := classifyAdminNFQWSJobOutcome(adminNFQWSJob{Kind: "detect-target"}, 503, ``, errors.New("boom")); got != adminNFQWSJobOutcomeFailed {
		t.Fatalf("failed=%q", got)
	}
}

func TestAdminNFQWSJobSingleFlightByKindKey(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	started := make(chan string, 2)
	release := make(chan struct{})
	runtime := adminNFQWSJobRuntime{
		config: adminNFQWSJobsConfig{Version: 1, Items: map[string]adminNFQWSJob{
			"a": {ID: "a", Kind: "detect-target", Target: "example.com", IntervalMinutes: 15, Enabled: true},
			"b": {ID: "b", Kind: "detect-target", Target: "example.com", IntervalMinutes: 15, Enabled: true},
		}},
		state: map[string]adminNFQWSJobState{
			"a": {adminNFQWSJob: adminNFQWSJob{ID: "a", Kind: "detect-target", Target: "example.com", IntervalMinutes: 15, Enabled: true}, NextRunAt: now.Add(-time.Second)},
			"b": {adminNFQWSJob: adminNFQWSJob{ID: "b", Kind: "detect-target", Target: "example.com", IntervalMinutes: 15, Enabled: true}, NextRunAt: now.Add(-time.Second)},
		},
		runningKeys: map[string]bool{},
		now:         func() time.Time { return now },
	}
	runtime.run = func(_ context.Context, job adminNFQWSJob) (int, string, error) {
		started <- job.ID
		<-release
		return 200, `{"classification":"clear"}`, nil
	}

	runtime.tick(now)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("no job started")
	}
	select {
	case id := <-started:
		t.Fatalf("single-flight allowed duplicate kind/key job %s", id)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
}

func TestAdminNFQWSJobBoundedRetryAndLastKnownGood(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	var calls int32
	job := adminNFQWSJob{ID: "a", Kind: "detect-target", Target: "example.com", IntervalMinutes: 15, Enabled: true}
	runtime := adminNFQWSJobRuntime{
		config:      adminNFQWSJobsConfig{Version: 1, Items: map[string]adminNFQWSJob{"a": job}},
		state:       map[string]adminNFQWSJobState{"a": {adminNFQWSJob: job, LastGoodOutput: `{"old":true}`}},
		runningKeys: map[string]bool{adminNFQWSJobKey(job): true},
		now:         func() time.Time { return now },
	}
	runtime.run = func(_ context.Context, _ adminNFQWSJob) (int, string, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return 503, `{"error":"temporary"}`, errors.New("temporary")
		}
		return 200, `{"classification":"clear"}`, nil
	}
	runtime.execute(job)
	state := runtime.statuses()[0]
	if state.Outcome != adminNFQWSJobOutcomeOK || state.RetryCount != 1 || atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("state=%+v calls=%d", state, calls)
	}
	if !strings.Contains(state.LastGoodOutput, `"classification":"clear"`) {
		t.Fatalf("last known good not updated: %q", state.LastGoodOutput)
	}

	runtime.run = func(_ context.Context, _ adminNFQWSJob) (int, string, error) {
		return 503, `{"error":"still bad"}`, errors.New("bad")
	}
	runtime.runningKeys[adminNFQWSJobKey(job)] = true
	runtime.execute(job)
	failed := runtime.statuses()[0]
	if failed.Outcome != adminNFQWSJobOutcomeFailed {
		t.Fatalf("outcome=%q", failed.Outcome)
	}
	if failed.LastGoodOutput != state.LastGoodOutput {
		t.Fatalf("last known good was overwritten: before=%q after=%q", state.LastGoodOutput, failed.LastGoodOutput)
	}
}
func TestParseAdminNFQWSManagerConfigSHAFromCompactHealth(t *testing.T) {
	want := strings.Repeat("b", 64)
	got, err := parseAdminNFQWSManagerConfigSHA(`{"ok":true,"module":"nfqws-manager","config_sha256":"` + want + `"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("config sha=%q want=%q", got, want)
	}
	if _, err := parseAdminNFQWSManagerConfigSHA(`{"ok":true}`); err == nil {
		t.Fatal("health without config_sha256 was accepted")
	}
}
func TestAdminNFQWSStrategyHealthRecheckOutcomeClassification(t *testing.T) {
	job := adminNFQWSJob{Kind: "strategy-health-recheck"}

	if got := classifyAdminNFQWSJobOutcome(job, 200, `{"ok":true,"cleanup_baseline_after":true,"strategy_needed":false,"recommendation_available":false}`, nil); got != adminNFQWSJobOutcomeOK {
		t.Fatalf("stable baseline outcome=%q", got)
	}
	if got := classifyAdminNFQWSJobOutcome(job, 200, `{"ok":true,"cleanup_baseline_after":true,"strategy_needed":true,"recommendation_available":true}`, nil); got != adminNFQWSJobOutcomeOK {
		t.Fatalf("verified recommendation outcome=%q", got)
	}
	if got := classifyAdminNFQWSJobOutcome(job, 200, `{"ok":true,"cleanup_baseline_after":true,"strategy_needed":true,"recommendation_available":false}`, nil); got != adminNFQWSJobOutcomeInconclusive {
		t.Fatalf("missing recommendation outcome=%q", got)
	}
	if got := classifyAdminNFQWSJobOutcome(job, 200, `{"ok":false,"cleanup_baseline_after":false}`, nil); got != adminNFQWSJobOutcomeUnhealthy {
		t.Fatalf("failed cleanup outcome=%q", got)
	}
}

func TestAdminNFQWSStrategyHealthRecheckTimeoutCoversFastSelector(t *testing.T) {
	job := adminNFQWSJob{Kind: "strategy-health-recheck"}
	if got := adminNFQWSJobTimeout(job); got != 120*time.Second {
		t.Fatalf("strategy health recheck timeout=%v want=120s", got)
	}
}
