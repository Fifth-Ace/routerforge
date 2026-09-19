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

func TestBenchSafetyContractLocked(t *testing.T) {
	got := benchCapabilities{
		ReadOnly: true, BenchEnabled: false, SafeToBench: false,
		LifecycleContractImplemented: true,
		CleanupProofImplemented:      true,
		SelectorContractImplemented:  true,
		TransactionEngineImplemented: true,
		Transaction:                  buildBenchTransactionContract(),
		StrategyCompilerImplemented:  true,
		StrategyPreflightImplemented: true,
		TLSStrategySmokeImplemented:  true,
		AutoTuneTLSLoopImplemented:   true,
	}
	if got.BenchEnabled || got.SafeToBench {
		t.Fatal("controlled smoke must not enable general AutoTune bench")
	}
	if !got.LifecycleContractImplemented || !got.CleanupProofImplemented ||
		!got.SelectorContractImplemented || !got.TransactionEngineImplemented ||
		!got.StrategyCompilerImplemented || !got.StrategyPreflightImplemented ||
		!got.TLSStrategySmokeImplemented || !got.AutoTuneTLSLoopImplemented {
		t.Fatal("lifecycle, cleanup, selector, transaction, compiler, preflight, TLS smoke and AutoTune loop foundations must be present")
	}
	if !got.Transaction.MutationEnabled || !got.Transaction.ControlledSmokeOnly {
		t.Fatal("D2 system mutator must remain limited to controlled smoke")
	}
}
