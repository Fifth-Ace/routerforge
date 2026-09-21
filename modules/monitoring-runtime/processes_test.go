package main

import "testing"

func TestParseProcessStatLine(t *testing.T) {
	line := "123 (router forge) R 1 2 3 4 5 6 7 8 9 10 120 30 14 15 16 17 8 19 2000 4096000 256"
	got, ok := parseProcessStatLine(123, line)
	if !ok {
		t.Fatal("parseProcessStatLine rejected valid stat line")
	}
	if got.PID != 123 || got.PPID != 1 || got.Name != "router forge" || got.State != "R" {
		t.Fatalf("unexpected identity fields: %+v", got)
	}
	if got.CPUTime != 150 || got.StartTime != 2000 || got.Threads != 8 {
		t.Fatalf("unexpected cpu fields: %+v", got)
	}
	if got.VmSizeKB != 4000 || got.RSSKB != 256 {
		t.Fatalf("unexpected memory fields: %+v", got)
	}
}

func TestParseProcessStatLineRejectsPIDMismatch(t *testing.T) {
	line := "123 (router forge) S 1 2 3 4 5 6 7 8 9 10 12 13 14 15 16 17 2 19 2000 4096 1"
	if _, ok := parseProcessStatLine(124, line); ok {
		t.Fatal("parseProcessStatLine accepted mismatched pid")
	}
}

func TestProcessCPUPercentPerCoreScale(t *testing.T) {
	got := processCPUPercent(100, 130, 1000, 1400, 4)
	if got != 30 {
		t.Fatalf("processCPUPercent=%v want 30", got)
	}
}

func TestProcessCPUPercentRejectsCounterRegression(t *testing.T) {
	if got := processCPUPercent(130, 100, 1000, 1400, 4); got != 0 {
		t.Fatalf("processCPUPercent=%v want 0", got)
	}
	if got := processCPUPercent(100, 130, 1400, 1000, 4); got != 0 {
		t.Fatalf("processCPUPercent total regression=%v want 0", got)
	}
}

func TestFirstProcessCommandDoesNotExposeArguments(t *testing.T) {
	raw := []byte("/opt/bin/example\x00--token\x00super-secret\x00")
	if got := firstProcessCommand(raw); got != "/opt/bin/example" {
		t.Fatalf("firstProcessCommand=%q", got)
	}
}

func TestSortProcessInfosCPUThenRSS(t *testing.T) {
	items := []processInfo{
		{PID: 3, CPUPct: 5, RSSKB: 100},
		{PID: 2, CPUPct: 20, RSSKB: 50},
		{PID: 1, CPUPct: 20, RSSKB: 200},
	}
	sortProcessInfos(items)
	if items[0].PID != 1 || items[1].PID != 2 || items[2].PID != 3 {
		t.Fatalf("unexpected order: %+v", items)
	}
}
