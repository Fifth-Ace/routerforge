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

func TestParseManagementSessionPorts(t *testing.T) {
	input := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: FC01A8C0:0016 0A01A8C0:C350 01 00000000:00000000 00:00000000 00000000 0 0 1
   1: FC01A8C0:00DE 0A01A8C0:C351 01 00000000:00000000 00:00000000 00000000 0 0 2
   2: FC01A8C0:0016 0B01A8C0:C352 0A 00000000:00000000 00:00000000 00000000 0 0 3
   3: FC01A8C0:01BB 0C01A8C0:C353 01 00000000:00000000 00:00000000 00000000 0 0 4
`
	got := parseManagementSessionPorts(input)
	want := []int{22, 222}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("management ports=%v want=%v", got, want)
	}
}

func TestMergeManagementSessionPorts(t *testing.T) {
	got := mergeManagementSessionPorts([]int{222}, []int{22, 222}, []int{443})
	want := []int{22, 222}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("management ports=%v want=%v", got, want)
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
		ManagementSessionInventoryOK: true,
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
	got.Selector.MutationImplemented = true
	got.ManagementSessionActive = true
	got.ManagementSessionPorts = []int{222}
	if !benchExecutionReady(got) {
		t.Fatal("active management SSH session must remain ready when exact bench isolation is proven")
	}
	got.Transaction.ManagementTrafficIsolation = false
	if benchExecutionReady(got) {
		t.Fatal("unproven management traffic isolation must lock bench readiness")
	}
	got.Transaction.ManagementTrafficIsolation = true
	got.ManagementSessionActive = false
	got.ManagementSessionInventoryOK = false
	if benchExecutionReady(got) {
		t.Fatal("unproven management SSH inventory must lock bench readiness")
	}
}
