package main

import "testing"

func TestCounterDelta(t *testing.T) {
	if got := counterDelta(120, 100); got != 20 {
		t.Fatalf("delta=%v", got)
	}
	if got := counterDelta(10, 100); got != 0 {
		t.Fatalf("wrap delta=%v", got)
	}
}

func TestPseudoFilesystem(t *testing.T) {
	if !pseudoFilesystem("proc") {
		t.Fatal("proc must be pseudo")
	}
	if pseudoFilesystem("ext4") {
		t.Fatal("ext4 must not be pseudo")
	}
}
