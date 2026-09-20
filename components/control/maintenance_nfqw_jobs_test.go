package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAdminNFQWSJobValidation(t *testing.T) {
	job, err := normalizeAdminNFQWSJob(adminNFQWSJob{
		ID: "youtube-check", Kind: "detect-target", Target: "YouTube.COM.",
		IntervalMinutes: 30, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.Target != "youtube.com" || job.Kind != "detect-target" {
		t.Fatalf("normalized=%+v", job)
	}
	if _, err := normalizeAdminNFQWSJob(adminNFQWSJob{
		ID: "bad", Kind: "detect-target", Target: "127.0.0.1",
		IntervalMinutes: 30,
	}); err == nil {
		t.Fatal("IP target accepted")
	}
}

func TestAdminNFQWSJobTickRunsOnlyDueEnabledJobs(t *testing.T) {
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	runtime := adminNFQWSJobRuntime{
		config: adminNFQWSJobsConfig{Version: 1, Items: map[string]adminNFQWSJob{
			"a": {ID: "a", Kind: "detect-target", Target: "example.com", IntervalMinutes: 15, Enabled: true},
			"b": {ID: "b", Kind: "detect-target", Target: "example.org", IntervalMinutes: 15, Enabled: false},
		}},
		state: map[string]adminNFQWSJobState{
			"a": {adminNFQWSJob: adminNFQWSJob{ID: "a", Kind: "detect-target", Target: "example.com", IntervalMinutes: 15, Enabled: true}, NextRunAt: now.Add(-time.Second)},
			"b": {adminNFQWSJob: adminNFQWSJob{ID: "b", Kind: "detect-target", Target: "example.org", IntervalMinutes: 15, Enabled: false}, NextRunAt: now.Add(-time.Second)},
		},
		now: func() time.Time { return now },
	}
	done := make(chan string, 2)
	runtime.run = func(_ context.Context, job adminNFQWSJob) (int, string, error) {
		done <- job.ID
		return 200, `{"ok":true}`, nil
	}

	runtime.tick(now)
	select {
	case id := <-done:
		if id != "a" {
			t.Fatalf("ran job=%s", id)
		}
	case <-time.After(time.Second):
		t.Fatal("due job did not run")
	}
	select {
	case id := <-done:
		t.Fatalf("unexpected second run=%s", id)
	case <-time.After(50 * time.Millisecond):
	}

	deadline := time.Now().Add(time.Second)
	for {
		statuses := runtime.statuses()
		for _, status := range statuses {
			if status.ID == "a" && !status.Running {
				if !status.LastOK || status.LastStatus != 200 || !strings.Contains(status.LastOutput, `"ok":true`) {
					t.Fatalf("state=%+v", status)
				}
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not settle")
		}
		time.Sleep(time.Millisecond)
	}
}
