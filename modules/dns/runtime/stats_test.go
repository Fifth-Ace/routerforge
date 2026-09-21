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
	if defaultFlowRetentionCap != 10000 {
		t.Fatalf("retention cap=%d want=10000", defaultFlowRetentionCap)
	}
	if defaultFlowRetentionCap < 2*maxClientDetailEvents {
		t.Fatalf("retention cap %d must keep at least 2x client detail max %d", defaultFlowRetentionCap, maxClientDetailEvents)
	}

	s := NewStore(defaultFlowRetentionCap, 8)
	if len(s.flow) != defaultFlowRetentionCap || len(s.clientFlow) != defaultFlowRetentionCap {
		t.Fatalf("preallocated rings flow=%d client_flow=%d want=%d", len(s.flow), len(s.clientFlow), defaultFlowRetentionCap)
	}

	fullPair := unsafe.Sizeof(FlowEvent{}) + unsafe.Sizeof(ClientFlowEvent{})
	compactPair := unsafe.Sizeof(compactFlowEvent{}) + unsafe.Sizeof(compactClientFlowEvent{})
	if compactPair >= fullPair/2 {
		t.Fatalf("compact pair=%d full pair=%d; want compact below 50%%", compactPair, fullPair)
	}

	ringBytes := uintptr(defaultFlowRetentionCap) * compactPair
	if unsafe.Sizeof(uintptr(0)) == 8 && ringBytes > 2500*1024 {
		t.Fatalf("64-bit 10k compact ring base storage too large: %d bytes", ringBytes)
	}
}

func TestDNSMemoryProfiles(t *testing.T) {
	tests := []struct {
		name          string
		totalKB       int64
		profile       string
		flow          int
		domains       int
		clientDomains int
	}{
		{name: "unknown", totalKB: 0, profile: "legacy", flow: defaultFlowRetentionCap, domains: defaultDomainCountCap, clientDomains: defaultClientDomainCap},
		{name: "small", totalKB: 128 * 1024, profile: "small", flow: 2048, domains: 4096, clientDomains: 256},
		{name: "medium", totalKB: 128*1024 + 1, profile: "medium", flow: 4096, domains: 8192, clientDomains: 384},
		{name: "medium-max", totalKB: 256 * 1024, profile: "medium", flow: 4096, domains: 8192, clientDomains: 384},
		{name: "large", totalKB: 256*1024 + 1, profile: "large", flow: 8192, domains: 12000, clientDomains: 512},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dnsMemoryProfileFor(tt.totalKB)
			if got.Name != tt.profile || got.FlowRetentionCap != tt.flow || got.DomainCountCap != tt.domains || got.ClientDomainCap != tt.clientDomains {
				t.Fatalf("profile=%#v", got)
			}
			if got.FlowRetentionCap < maxClientDetailEvents {
				t.Fatalf("flow cap %d below client detail max %d", got.FlowRetentionCap, maxClientDetailEvents)
			}
		})
	}
}
func TestStoreMemoryLimitsCapGrowth(t *testing.T) {
	s := newStoreWithLimits(4, 8, 2, 2)
	if len(s.flow) != 4 || len(s.clientFlow) != 4 || s.domainCountCap != 2 || s.clientDomainCap != 2 {
		t.Fatalf("unexpected limits: flow=%d clientFlow=%d domains=%d clientDomains=%d", len(s.flow), len(s.clientFlow), s.domainCountCap, s.clientDomainCap)
	}
	s.UpdateDiscovery([]UpstreamMeta{{Port: 40502, Profile: "System", Protocol: "DoT", Name: "Test"}})
	now := time.Unix(5000, 0)
	domains := []string{"a.example", "b.example", "c.example", "a.example"}
	for i, domain := range domains {
		d := DNSMessage{ID: uint16(i + 1), QName: domain, QType: 1}
		s.RecordQuery(now.Add(time.Duration(i)*time.Millisecond), "UDP", 40502, uint16(51000+i), d.ID, d)
	}
	if len(s.domainCounts) != 2 || s.domainCounts["a.example"] != 2 {
		t.Fatalf("domainCounts=%#v", s.domainCounts)
	}
	client := ClientInfo{IP: "192.168.1.20", MAC: "02:00:00:00:00:20", Name: "Test", Policy: "System", Active: true}
	s.UpdateClientRegistry([]ClientInfo{client})
	for i, domain := range []string{"one.example", "two.example", "three.example"} {
		d := DNSMessage{ID: uint16(100 + i), QName: domain, QType: 1}
		s.RecordClientQuery(now.Add(time.Duration(i)*time.Millisecond), "UDP", client.IP, client.MAC, d.ID, d)
	}
	state := s.clientStats[clientStateKey(client)]
	if state == nil || len(state.domains) != 2 {
		t.Fatalf("client domains=%#v", state)
	}
}
