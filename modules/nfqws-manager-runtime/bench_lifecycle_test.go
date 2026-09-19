package main

import (
	"reflect"
	"testing"
)

func TestBenchCleanupProofCleanBaseline(t *testing.T) {
	got := buildBenchCleanupProof(
		30000,
		true,
		[]int{300},
		[]int{300},
		[]int{300},
		"config-sha",
	)

	if !got.Proven || !got.ReservedRangeClean {
		t.Fatalf("cleanup proof=%+v want proven clean baseline", got)
	}
	if len(got.OccupiedReservedQueues) != 0 {
		t.Fatalf("occupied reserved queues=%v want none", got.OccupiedReservedQueues)
	}
	if got.ProductionConfigSHA256 != "config-sha" {
		t.Fatalf("config sha=%q", got.ProductionConfigSHA256)
	}
}

func TestBenchCleanupProofDetectsReservedFirewallLeftover(t *testing.T) {
	got := buildBenchCleanupProof(
		30001,
		true,
		[]int{300},
		[]int{300, 30000},
		[]int{300},
		"config-sha",
	)

	if got.Proven || got.ReservedRangeClean {
		t.Fatalf("cleanup proof=%+v want unproven", got)
	}
	if !reflect.DeepEqual(got.FirewallReservedQueues, []int{30000}) {
		t.Fatalf("firewall reserved=%v want [30000]", got.FirewallReservedQueues)
	}
	if !reflect.DeepEqual(got.OccupiedReservedQueues, []int{30000}) {
		t.Fatalf("occupied reserved=%v want [30000]", got.OccupiedReservedQueues)
	}
}

func TestBenchCleanupProofRequiresCompleteInventory(t *testing.T) {
	got := buildBenchCleanupProof(
		30000,
		false,
		[]int{300},
		[]int{},
		[]int{300},
		"config-sha",
	)

	if got.Proven {
		t.Fatalf("cleanup proof=%+v want unproven with incomplete inventory", got)
	}
	if len(got.Reasons) == 0 {
		t.Fatal("expected cleanup blocker reason")
	}
}

func TestBenchLifecycleContractStartsLockedAndClean(t *testing.T) {
	got := buildBenchLifecycleContract(
		30000,
		true,
		[]int{300},
		[]int{300},
		[]int{300},
		"config-sha",
	)

	if !got.Implemented || got.ActiveSession || got.SessionMutationImplemented {
		t.Fatalf("unexpected lifecycle contract=%+v", got)
	}
	if got.State != benchLifecycleStateClean || !got.Cleanup.Proven {
		t.Fatalf("lifecycle state=%q cleanup=%+v", got.State, got.Cleanup)
	}
	wantStates := []string{
		"NEW",
		"CANDIDATE_STARTED",
		"RULE_INSTALLED",
		"PROBE_RUNNING",
		"CLEANING",
		"CLEAN",
		"FAILED",
		"CLEANUP_UNPROVEN",
	}
	if !reflect.DeepEqual(got.States, wantStates) {
		t.Fatalf("states=%v want=%v", got.States, wantStates)
	}
}
