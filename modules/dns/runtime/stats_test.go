package main

import (
	"testing"
	"time"
	"unsafe"
)

func TestFallbackDetection(t *testing.T) {
	s := NewStore(32, 8)
	s.UpdateDiscovery([]UpstreamMeta{
		{Port: 40510, Profile: "System", Protocol: "DoH", Name: "Xbox DNS DoH", ProceedMS: 500},
		{Port: 40504, Profile: "System", Protocol: "DoT", Name: "Yandex DoT", ProceedMS: 500},
	})
	d := DNSMessage{ID: 1, QName: "example.org", QType: 1}
	t0 := time.Unix(100, 0)
	s.RecordQuery(t0, "UDP", 40510, 50000, 1, d)
	s.RecordQuery(t0.Add(500*time.Millisecond), "UDP", 40504, 50001, 2, d)
	snap := s.Snapshot(10, 10, 10)
	if snap["total_fallbacks"].(uint64) != 1 {
		t.Fatalf("fallbacks=%v", snap["total_fallbacks"])
	}
	flow := snap["flow"].([]FlowEvent)
	if len(flow) != 2 || !flow[1].Fallback {
		t.Fatalf("flow=%#v", flow)
	}
}
func TestDefaultFlowRetentionBudget(t *testing.T) {
	if defaultFlowRetentionCap < 2*maxClientDetailEvents {
		t.Fatalf("retention cap %d must keep at least 2x client detail max %d", defaultFlowRetentionCap, maxClientDetailEvents)
	}

	s := NewStore(defaultFlowRetentionCap, 8)
	if len(s.flow) != defaultFlowRetentionCap || len(s.clientFlow) != defaultFlowRetentionCap {
		t.Fatalf("preallocated rings flow=%d client_flow=%d want=%d", len(s.flow), len(s.clientFlow), defaultFlowRetentionCap)
	}

	ringBytes := uintptr(defaultFlowRetentionCap) * (unsafe.Sizeof(FlowEvent{}) + unsafe.Sizeof(ClientFlowEvent{}))
	if unsafe.Sizeof(uintptr(0)) == 8 && ringBytes > 3*1024*1024 {
		t.Fatalf("64-bit base ring budget too large: %d bytes", ringBytes)
	}
}
