package main

import (
	"context"
	"strings"
	"testing"
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
