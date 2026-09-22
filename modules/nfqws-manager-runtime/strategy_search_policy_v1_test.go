package main

import "testing"

func TestP27FastUsesCeilThirtyPercent(t *testing.T) {
	if got := v2DirectAutoPoolLimit("fast", 111); got != 34 {
		t.Fatalf("fast limit=%d want 34 for 111 corpus entries", got)
	}
	if got := v2DirectAutoPoolLimit("fast", 1); got != 1 {
		t.Fatalf("fast limit=%d want 1", got)
	}
}

func TestP27NormalAndThoroughExposeFullCorpus(t *testing.T) {
	for _, mode := range []string{"normal", "thorough"} {
		if got := v2DirectAutoPoolLimit(mode, 111); got != 111 {
			t.Fatalf("%s limit=%d want 111", mode, got)
		}
	}
}

func TestP27NormalRunsTwentyCandidateBatches(t *testing.T) {
	if got := v2DirectBatchSize("normal", 111); got != 20 {
		t.Fatalf("normal first batch=%d want 20", got)
	}
	if got := v2DirectBatchSize("normal", 11); got != 11 {
		t.Fatalf("normal tail batch=%d want 11", got)
	}
}

func TestP27NormalStopsAtTwoWorkingOnlyAfterBatch(t *testing.T) {
	if v2DirectShouldStopAfterBatch("normal", 1, 20, 111) {
		t.Fatal("normal stopped with only one working candidate")
	}
	if !v2DirectShouldStopAfterBatch("normal", 2, 40, 111) {
		t.Fatal("normal did not stop after reaching two working candidates")
	}
	if !v2DirectShouldStopAfterBatch("normal", 0, 111, 111) {
		t.Fatal("normal did not stop after exhausting corpus")
	}
}

func TestP27ThoroughNeverStopsBeforeExhaustion(t *testing.T) {
	if v2DirectShouldStopAfterBatch("thorough", 99, 100, 111) {
		t.Fatal("thorough stopped before full corpus was exhausted")
	}
	if !v2DirectShouldStopAfterBatch("thorough", 0, 111, 111) {
		t.Fatal("thorough did not stop at exhaustion")
	}
}

func TestP27FastStopsAfterItsSingleSelectionWindow(t *testing.T) {
	if !v2DirectShouldStopAfterBatch("fast", 0, 34, 111) {
		t.Fatal("fast did not stop after its selected 30% window")
	}
}
