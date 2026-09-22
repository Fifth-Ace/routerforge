package main

import "testing"

func TestV2DirectStripRuntimeArgs(t *testing.T) {
	in := []string{
		"--daemon",
		"--pidfile=/tmp/x.pid",
		"--qnum=123",
		"--fwmark=0x1",
		"--user=nobody",
		"--filter-tcp=443",
		"--filter-l7=tls",
	}
	out := v2DirectStripRuntimeArgs(in)
	if len(out) != 2 {
		t.Fatalf("unexpected stripped args: %#v", out)
	}
	if out[0] != "--filter-tcp=443" || out[1] != "--filter-l7=tls" {
		t.Fatalf("unexpected preserved args: %#v", out)
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
