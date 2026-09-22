package main

import "testing"

func TestV2DirectStripDaemon(t *testing.T) {
	in := []string{
		"--daemon",
		"--pidfile=/tmp/x.pid",
		"--qnum=123",
		"--fwmark=0x1",
		"--user=nobody",
		"--filter-tcp=443",
		"--filter-l7=tls",
	}
	out := v2DirectStripDaemon(in)
	if len(out) != 6 {
		t.Fatalf("unexpected daemon-only strip: %#v", out)
	}
	if out[0] != "--pidfile=/tmp/x.pid" || out[1] != "--qnum=123" || out[2] != "--fwmark=0x1" || out[3] != "--user=nobody" {
		t.Fatalf("non-daemon runtime args were unexpectedly stripped: %#v", out)
	}
}

func TestV2DirectWorkerPortRangesDoNotOverlap(t *testing.T) {
	for worker := 0; worker < v2MaxConcurrency; worker++ {
		lo := v2DirectPortBase + worker*v2DirectPortsPerWorker
		hi := lo + v2DirectPortsPerWorker - 1
		if lo < 1024 || hi > 65535 {
			t.Fatalf("worker %d range out of bounds: %d-%d", worker, lo, hi)
		}
		if worker > 0 {
			prevHi := v2DirectPortBase + worker*v2DirectPortsPerWorker - 1
			if lo <= prevHi {
				t.Fatalf("worker %d overlaps previous range", worker)
			}
		}
	}
}

func TestV2DirectReferenceMarksAndWorkerRanges(t *testing.T) {
	if v2DirectProcMark != "0x40000000/0x40000000" {
		t.Fatalf("unexpected process mark: %s", v2DirectProcMark)
	}
	if v2DirectExclMark != "0x20000000/0x20000000" {
		t.Fatalf("unexpected exclusion connmark: %s", v2DirectExclMark)
	}
	if v2DirectPortBase != 50000 || v2DirectPortsPerWorker != 200 {
		t.Fatalf("unexpected reference worker port layout: base=%d span=%d", v2DirectPortBase, v2DirectPortsPerWorker)
	}
}
