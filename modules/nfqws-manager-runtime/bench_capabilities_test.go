package main

import (
	"reflect"
	"testing"
)

func TestParseBenchQueueNumbers(t *testing.T) {
	input := `
-A FORWARD -j NFQUEUE --queue-num 200
-A FORWARD -j NFQUEUE --queue-balance 300:302
table inet filter { chain forward { queue num 400-401 bypass } }
nfqws2 --qnum=777 --dpi-desync=fake
`
	got := parseBenchQueueNumbers(input)
	want := []int{200, 300, 301, 302, 400, 401, 777}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("queues=%v want=%v", got, want)
	}
}

func TestParseNFNetlinkQueue(t *testing.T) {
	got := parseNFNetlinkQueue("200 1234 2 65535 0 0 0 1\n777 4321 2 65535 0 0 0 1\n")
	want := []int{200, 777}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("queues=%v want=%v", got, want)
	}
}

func TestFirstFreeBenchQueue(t *testing.T) {
	if got := firstFreeBenchQueue([]int{30000, 30001, 30003}); got != 30002 {
		t.Fatalf("free queue=%d want=30002", got)
	}
}

func TestBenchExecutionReady(t *testing.T) {
	got := benchCapabilities{
		CandidateSpawnCapable:        true,
		IPTablesPath:                 "/opt/sbin/iptables",
		IPTablesSavePath:             "/opt/sbin/iptables-save",
		FirewallInventoryOK:          true,
		KernelQueueInventoryOK:       true,
		QueueInventoryComplete:       true,
		RecommendedQueue:             30000,
		LifecycleContractImplemented: true,
		CleanupProofImplemented:      true,
		CleanupBaselineProven:        true,
		SelectorContractImplemented:  true,
		Selector:                     buildBenchSelectorContract(),
		TransactionEngineImplemented: true,
		Transaction:                  buildBenchTransactionContract(),
	}
	if !benchExecutionReady(got) {
		t.Fatal("fully proven bench capability set must be ready")
	}
	got.RecommendedQueue = 0
	if benchExecutionReady(got) {
		t.Fatal("missing reserved queue must lock bench readiness")
	}
	got.RecommendedQueue = 30000
	got.CleanupBaselineProven = false
	if benchExecutionReady(got) {
		t.Fatal("unproven cleanup baseline must lock bench readiness")
	}
	got.CleanupBaselineProven = true
	got.Selector.MutationImplemented = false
	if benchExecutionReady(got) {
		t.Fatal("inactive selector mutation must lock bench readiness")
	}
}
