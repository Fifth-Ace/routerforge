package main

import "testing"

func TestCoreCPUUsagePercent(t *testing.T) {
	previous := coreCPUCounter{Idle: 100, IOWait: 10, Total: 200}
	current := coreCPUCounter{Idle: 140, IOWait: 10, Total: 300}

	got, ok := coreCPUUsagePercent(previous, current)
	if !ok || got != 60 {
		t.Fatalf("cpu usage=(%v, %v), want (60, true)", got, ok)
	}
}

func TestParseCoreCPUCounter(t *testing.T) {
	counter, ok := parseCoreCPUCounter("cpu  10 2 3 80 5 1 1 0 0 0\ncpu0 1 0 1 40 2 0 0 0 0 0\n")
	if !ok {
		t.Fatal("aggregate CPU counter was not parsed")
	}
	if counter.Idle != 80 || counter.IOWait != 5 || counter.Total != 102 {
		t.Fatalf("unexpected counter: %#v", counter)
	}
}

func TestParseCoreRAMUsage(t *testing.T) {
	raw := "MemTotal: 1000 kB\nMemFree: 100 kB\nMemAvailable: 400 kB\nBuffers: 50 kB\nCached: 200 kB\nSReclaimable: 25 kB\n"
	got, ok := parseCoreRAMUsage(raw)
	if !ok || got != 60 {
		t.Fatalf("ram usage=(%v, %v), want (60, true)", got, ok)
	}
}

func TestNormalizeCoreTemperature(t *testing.T) {
	got, ok := normalizeCoreTemperature(58321)
	if !ok || got != 58.321 {
		t.Fatalf("temperature=(%v, %v), want (58.321, true)", got, ok)
	}
	if _, ok := normalizeCoreTemperature(999999); ok {
		t.Fatal("invalid temperature was accepted")
	}
}
