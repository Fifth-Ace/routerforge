package main

import (
	"fmt"
	"testing"
)

func TestFlowExplorerResourceBoundsContract(t *testing.T) {
	if flowExplorerMaxLimit != 256 {
		t.Fatalf("flowExplorerMaxLimit=%d want 256", flowExplorerMaxLimit)
	}
	if flowSocketSnapshotMax != 4096 {
		t.Fatalf("flowSocketSnapshotMax=%d want 4096", flowSocketSnapshotMax)
	}
	if flowOwnerPIDBudget != 2048 {
		t.Fatalf("flowOwnerPIDBudget=%d want 2048", flowOwnerPIDBudget)
	}
	if flowOwnerFDBudget != 16384 {
		t.Fatalf("flowOwnerFDBudget=%d want 16384", flowOwnerFDBudget)
	}
	if flowPolicyLookupBudget != 32 {
		t.Fatalf("flowPolicyLookupBudget=%d want 32", flowPolicyLookupBudget)
	}
	if flowPolicyLookupBudget > flowExplorerMaxLimit {
		t.Fatalf("route-get budget=%d exceeds flow sample=%d", flowPolicyLookupBudget, flowExplorerMaxLimit)
	}
}

func TestFlowExplorerBoundedSampleParsesMaximumContract(t *testing.T) {
	entries := make([]flowExplorerEntry, 0, flowExplorerMaxLimit)
	for i := 0; i < flowExplorerMaxLimit; i++ {
		source := fmt.Sprintf("192.0.2.%d", (i%250)+1)
		line := fmt.Sprintf(
			"ipv4 2 tcp 6 120 ESTABLISHED src=%s dst=198.51.100.10 sport=%d dport=443 packets=1 bytes=64 src=198.51.100.10 dst=%s sport=443 dport=%d packets=1 bytes=96 mark=0 use=1",
			source,
			40000+i,
			source,
			40000+i,
		)
		entry, ok := parseFlowExplorer(line)
		if !ok {
			t.Fatalf("entry %d failed to parse", i)
		}
		entries = append(entries, entry)
	}
	if len(entries) != flowExplorerMaxLimit {
		t.Fatalf("parsed=%d want=%d", len(entries), flowExplorerMaxLimit)
	}
}
