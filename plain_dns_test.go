package main

import (
	"testing"
	"time"
)

func TestParsePlainDNSProxy(t *testing.T) {
	input := `
proxy-status:
  proxy-name: System
proxy-config:
dns_server = 8.8.8.8 .
dns_server = 8.8.4.4 example.org
dns_server = 127.0.0.1:40500 . # 9.9.9.9@dns.quad9.net
proxy-status:
  proxy-name: Policy2
proxy-config:
dns_server = 8.8.8.8 .
`
	got := parsePlainDNSProxy(input)
	if len(got) != 2 {
		t.Fatalf("got %d plain resolvers: %#v", len(got), got)
	}
	var google *PlainDNSMeta
	for i := range got {
		if got[i].Address == "8.8.8.8" {
			google = &got[i]
		}
	}
	if google == nil {
		t.Fatal("8.8.8.8 not found")
	}
	if len(google.Profiles) != 2 || google.Profiles[0] != "Policy2" || google.Profiles[1] != "System" {
		t.Fatalf("profiles=%#v", google.Profiles)
	}
}

func TestPlainDNSTrackerRoundTrip(t *testing.T) {
	tracker := newPlainDNSTracker(10)
	tracker.UpdateResolvers([]PlainDNSMeta{{Address: "8.8.8.8", Port: 53, Profiles: []string{"System"}}})

	start := time.Unix(100, 0)
	query := DNSMessage{ID: 42, QName: "example.org", QType: 1}
	response := DNSMessage{ID: 42, QR: true, QName: "example.org", QType: 1, RCode: 0}

	if !tracker.RecordQuery(start, "UDP", "8.8.8.8", 53, 53000, query) {
		t.Fatal("query was not recorded")
	}
	if !tracker.RecordResponse(start.Add(25*time.Millisecond), "UDP", "8.8.8.8", 53, 53000, response) {
		t.Fatal("response was not recorded")
	}

	snap := tracker.Snapshot(10)
	if len(snap.Resolvers) != 1 {
		t.Fatalf("resolvers=%d", len(snap.Resolvers))
	}
	view := snap.Resolvers[0]
	if view.Requests != 1 || view.Responses != 1 || view.LastLatency != 25 {
		t.Fatalf("bad resolver stats: %#v", view)
	}
	if len(snap.Recent) != 1 || snap.Recent[0].Status != "ANSWER" {
		t.Fatalf("recent=%#v", snap.Recent)
	}
}

func TestPlainDNSTrackerSuppressesForwardedClientQuery(t *testing.T) {
	tracker := newPlainDNSTracker(10)
	tracker.UpdateResolvers([]PlainDNSMeta{{Address: "1.1.1.1", Port: 53}})
	start := time.Unix(200, 0)
	msg := DNSMessage{ID: 9, QName: "example.com", QType: 1}

	tracker.ObserveDirectClientQuery(start, "UDP", "1.1.1.1", 53, msg)
	if tracker.RecordQuery(start.Add(10*time.Millisecond), "UDP", "1.1.1.1", 53, 44000, msg) {
		t.Fatal("forwarded direct-client query must be suppressed")
	}
	if got := tracker.Snapshot(10).Resolvers[0].Requests; got != 0 {
		t.Fatalf("requests=%d want 0", got)
	}
}

func TestPlainDNSTrackerSuppressesMultipleForwardedClientQueries(t *testing.T) {
	tracker := newPlainDNSTracker(10)
	tracker.UpdateResolvers([]PlainDNSMeta{{Address: "1.1.1.1", Port: 53}})
	start := time.Unix(210, 0)
	msg := DNSMessage{ID: 11, QName: "same.example", QType: 1}

	tracker.ObserveDirectClientQuery(start, "UDP", "1.1.1.1", 53, msg)
	tracker.ObserveDirectClientQuery(start.Add(time.Millisecond), "UDP", "1.1.1.1", 53, msg)

	if tracker.RecordQuery(start.Add(10*time.Millisecond), "UDP", "1.1.1.1", 53, 44000, msg) {
		t.Fatal("first forwarded query must be suppressed")
	}
	if tracker.RecordQuery(start.Add(20*time.Millisecond), "UDP", "1.1.1.1", 53, 44001, msg) {
		t.Fatal("second forwarded query must be suppressed")
	}
	if got := tracker.Snapshot(10).Resolvers[0].Requests; got != 0 {
		t.Fatalf("requests=%d want 0", got)
	}
}

func TestPlainDNSTrackerTimeout(t *testing.T) {
	tracker := newPlainDNSTracker(10)
	tracker.UpdateResolvers([]PlainDNSMeta{{Address: "1.1.1.1", Port: 53}})
	start := time.Unix(100, 0)
	tracker.RecordQuery(start, "TCP", "1.1.1.1", 53, 42000, DNSMessage{ID: 7, QName: "example.net", QType: 28})
	tracker.Sweep(start.Add(plainDNSTimeout + time.Second))
	snap := tracker.Snapshot(10)
	if snap.Resolvers[0].Timeouts != 1 || snap.Pending != 0 {
		t.Fatalf("bad timeout snapshot: %#v", snap)
	}
}

func TestPlainDNSObserveDirection(t *testing.T) {
	tracker := newPlainDNSTracker(10)
	tracker.UpdateResolvers([]PlainDNSMeta{{Address: "1.1.1.1", Port: 53}})
	start := time.Unix(200, 0)
	q := DNSMessage{ID: 9, QName: "example.net", QType: 28}
	if !tracker.Observe(start, "UDP", "192.168.1.1", "1.1.1.1", 40123, 53, true, q) {
		t.Fatal("outgoing configured DNS query was not observed")
	}
	r := DNSMessage{ID: 9, QR: true, RCode: 0}
	if !tracker.Observe(start.Add(12*time.Millisecond), "UDP", "1.1.1.1", "192.168.1.1", 53, 40123, false, r) {
		t.Fatal("incoming configured DNS response was not observed")
	}
	snap := tracker.Snapshot(10)
	if snap.Resolvers[0].LastLatency != 12 {
		t.Fatalf("latency=%v", snap.Resolvers[0].LastLatency)
	}
	if tracker.Observe(start, "UDP", "192.168.1.1", "9.9.9.9", 40123, 53, true, q) {
		t.Fatal("unconfigured resolver must not be observed")
	}
}
