package main

import "testing"

func TestCPUUsage(t *testing.T) {
	prev := cpuRaw{Idle: 100, IOWait: 10, Total: 200}
	current := cpuRaw{Idle: 140, IOWait: 10, Total: 300}

	got := cpuUsage(prev, current)
	if got != 60 {
		t.Fatalf("cpuUsage=%v want 60", got)
	}
}

func TestCPUUsageIdle(t *testing.T) {
	prev := cpuRaw{Idle: 100, Total: 200}
	current := cpuRaw{Idle: 200, Total: 300}

	if got := cpuUsage(prev, current); got != 0 {
		t.Fatalf("cpuUsage=%v want 0", got)
	}
}
