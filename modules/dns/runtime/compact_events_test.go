package main

import (
	"reflect"
	"testing"
	"time"
	"unsafe"
)

func TestCompactFlowEventRoundTrip(t *testing.T) {
	pool := newEventStringPool()
	want := FlowEvent{
		Time:           time.Unix(1700000000, 123456789),
		Profile:        "Policy1",
		Protocol:       "DoT",
		Upstream:       "9.9.9.9 DoT",
		Port:           40504,
		Domain:         "example.org",
		QType:          "AAAA",
		Transport:      "UDP",
		Fallback:       true,
		ClientIP:       "192.168.1.20",
		ClientMAC:      "aa:bb:cc:dd:ee:ff",
		ClientName:     "Laptop",
		ClientHostname: "laptop.home",
		ClientPolicy:   "Policy1",
		ClientNetwork:  "Home",
		ClientAccess:   "Wi-Fi",
		ClientSSID:     "Home-5G",
		ClientAP:       "Ultra",
	}

	compact := compactFlowFromEvent(pool, want)
	got := compact.materialize(pool)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip mismatch:\n got=%#v\nwant=%#v", got, want)
	}
}

func TestCompactClientFlowEventRoundTrip(t *testing.T) {
	pool := newEventStringPool()
	want := ClientFlowEvent{
		Time:              time.Unix(1700000000, 123456789),
		CompletedAt:       time.Unix(1700000000, 223456789),
		ClientIP:          "192.168.1.20",
		ClientMAC:         "aa:bb:cc:dd:ee:ff",
		ClientName:        "Laptop",
		ClientHostname:    "laptop.home",
		ClientPolicy:      "Policy1",
		ClientAccess:      "Wi-Fi",
		Domain:            "example.org",
		QType:             "A",
		Transport:         "UDP",
		Outcome:           "FORWARDED",
		RCode:             "NOERROR",
		LatencyMS:         12.5,
		Resolver:          "9.9.9.9 DoT",
		ResolverPort:      40504,
		Fallback:          true,
		UpstreamRCode:     "NOERROR",
		UpstreamLatencyMS: 10.25,
		UpstreamTimeout:   true,
	}

	compact := compactClientFlowFromEvent(pool, want)
	got := compact.materialize(pool)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip mismatch:\n got=%#v\nwant=%#v", got, want)
	}
}

func TestCompactRingRetainsLogicalCapacityAndReleasesStrings(t *testing.T) {
	s := NewStore(3, 8)

	for i, domain := range []string{"one.example", "two.example", "three.example", "four.example"} {
		s.mu.Lock()
		s.addFlowLocked(FlowEvent{
			Time:      time.Unix(int64(i+1), 0),
			Profile:   "System",
			Protocol:  "DoT",
			Upstream:  "Resolver",
			Port:      40504,
			Domain:    domain,
			QType:     "A",
			Transport: "UDP",
		})
		s.mu.Unlock()
	}

	s.mu.RLock()
	got := s.flowTailLocked(3)
	_, oldStillInterned := s.eventStrings.byValue["one.example"]
	s.mu.RUnlock()

	if oldStillInterned {
		t.Fatal("overwritten ring string still retained in pool")
	}
	if len(got) != 3 {
		t.Fatalf("tail len=%d want=3", len(got))
	}
	if got[0].Domain != "two.example" || got[1].Domain != "three.example" || got[2].Domain != "four.example" {
		t.Fatalf("ring order mismatch: %#v", got)
	}
}

func TestCompactEventStorageBudget(t *testing.T) {
	full := unsafe.Sizeof(FlowEvent{}) + unsafe.Sizeof(ClientFlowEvent{})
	compact := unsafe.Sizeof(compactFlowEvent{}) + unsafe.Sizeof(compactClientFlowEvent{})

	if compact >= full/2 {
		t.Fatalf("compact event storage=%d bytes, full=%d bytes; want compact below 50%%", compact, full)
	}

	total := uintptr(defaultFlowRetentionCap) * compact
	if unsafe.Sizeof(uintptr(0)) == 8 && total > 2500*1024 {
		t.Fatalf("10k compact ring base storage=%d bytes; want <= 2.5 MiB", total)
	}

	t.Logf("full_pair=%d compact_pair=%d retention=%d compact_ring_bytes=%d", full, compact, defaultFlowRetentionCap, total)
}
