package main

import (
	"strings"
	"testing"
)

func TestNF3B1ParseMemInfo(t *testing.T) {
	total, available, ok := v2WorkerMemoryValues("MemTotal:         262144 kB\nMemFree:             48 kB\nMemAvailable:     92160 kB\n")
	if !ok || total != 262144 || available != 92160 {
		t.Fatalf("unexpected memory values: %d/%d ok=%t", total, available, ok)
	}
	for _, raw := range []string{
		"MemTotal: 131072 kB\nMemFree: 99999 kB\n",
		"MemTotal: 131072 kB\nMemAvailable: 999999 kB\n",
		"MemTotal: 131072 kB\nMemAvailable: -1 kB\n",
		strings.Repeat("X", 20000),
	} {
		if _, _, ok := v2WorkerMemoryValues(raw); ok {
			t.Fatalf("invalid memory inventory accepted: %q", raw[:min(len(raw), 120)])
		}
	}
}

func TestNF3B1WorkerCapByTotalAndAvailable(t *testing.T) {
	cases := []struct {
		name      string
		totalMiB  int64
		availMiB  int64
		wantLimit int
	}{
		{"128M plenty", 128, 80, 1},
		{"256M plenty", 256, 130, 2},
		{"512M moderate free", 512, 90, 2},
		{"512M more free", 512, 140, 3},
		{"512M plenty", 512, 250, 4},
		{"critically low", 256, 20, 0},
		{"near low threshold", 256, 24, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := v2WorkerBudgetFromMemory(tc.totalMiB*1024, tc.availMiB*1024, true)
			if b.Limit != tc.wantLimit || !b.Measured {
				t.Fatalf("worker memory admission unexpected: %+v", b)
			}
		})
	}
}

func TestNF3B1WorkerCapOnUnknownInventory(t *testing.T) {
	b := v2WorkerBudgetFromMemory(0, 0, false)
	if b.Limit != 1 || b.Measured || b.Reason == "" {
		t.Fatalf("unavailable memory inventory must preserve one worker: %+v", b)
	}
}
