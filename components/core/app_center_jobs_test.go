package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestActionLineCaptureSplitsAndBoundsLines(t *testing.T) {
	var lines []string
	writer := &actionLineCapture{emit: func(line string) {
		lines = append(lines, line)
	}}
	_, _ = writer.Write([]byte("one\ntwo"))
	_, _ = writer.Write([]byte("-part\nthree\n"))
	writer.Flush()

	got := strings.Join(lines, "|")
	if got != "one|two-part|three" {
		t.Fatalf("lines=%q", got)
	}
}

func TestAppActionJobBoundsLines(t *testing.T) {
	job := &appActionJob{}
	for i := 0; i < appActionMaxLines+25; i++ {
		job.appendLine("line")
	}
	view := job.snapshot()
	if len(view.Lines) != appActionMaxLines {
		t.Fatalf("lines=%d, want %d", len(view.Lines), appActionMaxLines)
	}
}

func TestPruneAppActionJobsBoundsTerminalRetention(t *testing.T) {
	appActions.Lock()
	savedActive := appActions.active
	savedJobs := appActions.jobs
	appActions.active = ""
	appActions.jobs = map[string]*appActionJob{}
	appActions.Unlock()

	t.Cleanup(func() {
		appActions.Lock()
		appActions.active = savedActive
		appActions.jobs = savedJobs
		appActions.Unlock()
	})

	base := time.Unix(1_700_000_000, 0)
	terminalCount := appActionTerminalRetention + 37

	appActions.Lock()
	for i := 0; i < terminalCount; i++ {
		id := fmt.Sprintf("terminal-%03d", i)
		appActions.jobs[id] = &appActionJob{
			ID:          id,
			State:       "succeeded",
			StartedAt:   base.Add(time.Duration(i) * time.Second),
			CompletedAt: base.Add(time.Duration(i) * time.Second),
		}
	}
	active := &appActionJob{
		ID:        "active-job",
		State:     "running",
		StartedAt: base.Add(time.Hour),
	}
	appActions.jobs[active.ID] = active
	appActions.active = active.ID
	appActions.Unlock()

	pruneAppActionJobs()

	appActions.Lock()
	defer appActions.Unlock()

	if got, want := len(appActions.jobs), appActionTerminalRetention+1; got != want {
		t.Fatalf("jobs=%d, want %d", got, want)
	}
	if got := appActions.jobs[active.ID]; got != active {
		t.Fatal("active job must never be pruned")
	}
	for i := 0; i < terminalCount; i++ {
		id := fmt.Sprintf("terminal-%03d", i)
		_, kept := appActions.jobs[id]
		wantKept := i >= terminalCount-appActionTerminalRetention
		if kept != wantKept {
			t.Fatalf("%s kept=%v, want %v", id, kept, wantKept)
		}
	}
}

func TestBuildCatalogPreflightRejectsCoreAsyncSelfUpdate(t *testing.T) {
	preflight, err := buildAppActionPreflight(context.Background(), appActionStartRequest{
		Kind:   "catalog",
		Target: "routerforge-core",
		Action: "update",
	})
	if err != nil {
		t.Fatal(err)
	}
	if preflight.Allowed {
		t.Fatal("Core async self-update must stay disabled")
	}
	if !strings.Contains(preflight.Reason, "synchronous") {
		t.Fatalf("unexpected reason: %q", preflight.Reason)
	}
}

func TestTerminalAppActionState(t *testing.T) {
	for _, state := range []string{"succeeded", "failed", "cancelled"} {
		if !terminalAppActionState(state) {
			t.Fatalf("%q should be terminal", state)
		}
	}
	for _, state := range []string{"queued", "running", ""} {
		if terminalAppActionState(state) {
			t.Fatalf("%q should not be terminal", state)
		}
	}
}
