package main

import (
	"path/filepath"
	"testing"
	"time"
)

func portReuseLogger(t *testing.T, name string) *EventLogger {
	t.Helper()
	return NewEventLogger(filepath.Join(t.TempDir(), name))
}

func portReuseMeta(port uint16, name, target string) UpstreamMeta {
	return UpstreamMeta{
		Port:      port,
		Profile:   "System",
		Protocol:  "DoT",
		Target:    target,
		Name:      name,
		SNI:       name + ".example",
		Interface: "ISP",
		ProceedMS: 500,
		TimeoutMS: 1000,
	}
}

func assertNoPortAttribution(t *testing.T, s *Store, port uint16) {
	t.Helper()

	for i := range s.history {
		b := &s.history[i]
		if _, ok := b.upstreams[port]; ok {
			t.Fatalf("history still contains resolver port %d", port)
		}
		for edge := range b.edges {
			if edge[0] == port || edge[1] == port {
				t.Fatalf("history fallback edge still contains resolver port %d: %#v", port, edge)
			}
		}
		for key := range b.errorBursts {
			if key.port == port {
				t.Fatalf("history error burst still contains resolver port %d: %#v", port, key)
			}
		}
	}
	for edge := range s.fallbackEdges {
		if edge[0] == port || edge[1] == port {
			t.Fatalf("cumulative fallback edge still contains resolver port %d: %#v", port, edge)
		}
	}
	for _, st := range s.clientStats {
		if st == nil {
			continue
		}
		if _, ok := st.resolvers[port]; ok {
			t.Fatalf("client resolver counters still contain resolver port %d", port)
		}
	}
}

func TestPortReuseResetsPortIndexedTelemetryWithoutRelabelingErrors(t *testing.T) {
	const reusedPort uint16 = 40506
	const fallbackPort uint16 = 40507

	s := NewStore(64, 64)
	comss := portReuseMeta(reusedPort, "Comss DoT", "dns.comss.one")
	other := portReuseMeta(fallbackPort, "Google DoT", "8.8.8.8")
	s.UpdateDiscovery([]UpstreamMeta{comss, other})

	now := time.Now()
	q := DNSMessage{ID: 10, QName: "reuse.example", QType: 1}
	s.RecordQuery(now, "UDP", reusedPort, 51000, 10, q)
	s.RecordQuery(now.Add(400*time.Millisecond), "UDP", fallbackPort, 51001, 11, DNSMessage{ID: 11, QName: q.QName, QType: 1})
	s.RecordResponse(now.Add(450*time.Millisecond), reusedPort, 51000, 10,
		DNSMessage{ID: 10, QR: true, QName: q.QName, QType: 1, RCode: 2},
		portReuseLogger(t, "direct-reuse.log"))

	old := s.upstreams[reusedPort]
	if old == nil || old.requests != 1 || old.servfail != 1 || old.fallbacks != 1 {
		t.Fatalf("test setup failed, old resolver counters: %#v", old)
	}
	if len(s.errors) == 0 || s.errors[len(s.errors)-1].Upstream != "Comss DoT" {
		t.Fatalf("test setup failed, expected self-described Comss error: %#v", s.errors)
	}

	quad9 := portReuseMeta(reusedPort, "Quad9 DoT", "9.9.9.9")
	s.UpdateDiscovery([]UpstreamMeta{quad9, other})

	got := s.upstreams[reusedPort]
	if got == nil {
		t.Fatal("Quad9 resolver missing after discovery")
	}
	if got.meta.Name != "Quad9 DoT" {
		t.Fatalf("wrong resolver metadata after reuse: %#v", got.meta)
	}
	if got.requests != 0 || got.responses != 0 || got.servfail != 0 || got.timeouts != 0 || got.fallbacks != 0 {
		t.Fatalf("new resolver inherited cumulative telemetry: %#v", got)
	}
	assertNoPortAttribution(t, s, reusedPort)

	if len(s.errors) == 0 {
		t.Fatal("historical self-described error was destroyed")
	}
	last := s.errors[len(s.errors)-1]
	if last.Upstream != "Comss DoT" || last.Port != reusedPort {
		t.Fatalf("historical error was relabeled after port reuse: %#v", last)
	}
}

func TestPortReuseAfterDiscoveryGapDoesNotReviveOldWindow(t *testing.T) {
	const port uint16 = 40506

	s := NewStore(32, 32)
	comss := portReuseMeta(port, "Comss DoT", "dns.comss.one")
	s.UpdateDiscovery([]UpstreamMeta{comss})

	now := time.Now()
	s.RecordQuery(now, "UDP", port, 52000, 20, DNSMessage{ID: 20, QName: "gap.example", QType: 1})
	s.RecordResponse(now.Add(20*time.Millisecond), port, 52000, 20,
		DNSMessage{ID: 20, QR: true, QName: "gap.example", QType: 1, RCode: 2},
		portReuseLogger(t, "gap-reuse.log"))

	s.UpdateDiscovery(nil)
	if _, ok := s.upstreams[port]; ok {
		t.Fatalf("resolver port %d should be absent during discovery gap", port)
	}

	quad9 := portReuseMeta(port, "Quad9 DoT", "9.9.9.9")
	s.UpdateDiscovery([]UpstreamMeta{quad9})

	got := s.upstreams[port]
	if got == nil || got.meta.Name != "Quad9 DoT" {
		t.Fatalf("Quad9 missing after discovery gap: %#v", got)
	}
	if got.requests != 0 || got.servfail != 0 || got.fallbacks != 0 || got.timeouts != 0 {
		t.Fatalf("new resolver revived old cumulative telemetry: %#v", got)
	}
	assertNoPortAttribution(t, s, port)

	if len(s.errors) == 0 || s.errors[len(s.errors)-1].Upstream != "Comss DoT" {
		t.Fatalf("historical error was lost or relabeled: %#v", s.errors)
	}
}

func TestPortReuseMutableMetadataKeepsSameResolverGeneration(t *testing.T) {
	const port uint16 = 40506

	s := NewStore(32, 32)
	first := UpstreamMeta{
		Port:              port,
		Profile:           "System",
		Protocol:          "DoT",
		Target:            "9.9.9.9",
		Name:              "Quad9 old label",
		SNI:               "dns.quad9.net",
		Domain:            "",
		Interface:         "ISP",
		LinuxInterface:    "eth3",
		PolicyMark:        100,
		PolicyTable:       100,
		PolicyDescription: "old policy text",
		ProfileDNSPort:    53,
		ProceedMS:         500,
		TimeoutMS:         1000,
	}
	s.UpdateDiscovery([]UpstreamMeta{first})

	now := time.Now()
	s.RecordQuery(now, "UDP", port, 53000, 30, DNSMessage{ID: 30, QName: "same.example", QType: 1})
	before := s.upstreams[port]

	second := first
	second.Name = "Quad9 renamed"
	second.LinuxInterface = "eth9"
	second.PolicyMark = 900
	second.PolicyTable = 901
	second.PolicyDescription = "new policy text"
	second.ProfileDNSPort = 5353
	second.ProceedMS = 900
	second.TimeoutMS = 2500
	s.UpdateDiscovery([]UpstreamMeta{second})

	after := s.upstreams[port]
	if before != after {
		t.Fatal("mutable metadata change incorrectly created a new resolver generation")
	}
	if after.requests != 1 || after.meta.Name != "Quad9 renamed" || len(s.pending) != 1 {
		t.Fatalf("same resolver telemetry was reset: state=%#v pending=%d", after, len(s.pending))
	}
	found := false
	for i := range s.history {
		if r := s.history[i].upstreams[port]; r != nil && r.requests == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("same resolver window telemetry was unexpectedly cleared")
	}
}

func TestPortReuseClearsClientAndPendingResolverAttribution(t *testing.T) {
	const port uint16 = 40506
	const clientIP = "192.0.2.10"

	s := NewStore(64, 64)
	comss := portReuseMeta(port, "Comss DoT", "dns.comss.one")
	s.UpdateDiscovery([]UpstreamMeta{comss})
	s.UpdateClientRegistry([]ClientInfo{{
		IP: clientIP, MAC: "00:11:22:33:44:55", Name: "test-client",
		Policy: "System", Access: "Ethernet", Active: true,
	}})

	now := time.Now()
	q := DNSMessage{ID: 40, QName: "client.example", QType: 1}
	s.RecordClientQuery(now, "UDP", clientIP, "00:11:22:33:44:55", 40, q)
	s.RecordQuery(now.Add(10*time.Millisecond), "UDP", port, 54000, 40, q)

	var clientKey string
	for key, st := range s.clientStats {
		if st != nil && st.info.IP == clientIP {
			clientKey = key
			if st.resolvers[port] != 1 {
				t.Fatalf("test setup failed, client resolver count=%d", st.resolvers[port])
			}
			break
		}
	}
	if clientKey == "" {
		t.Fatal("test client state missing")
	}

	quad9 := portReuseMeta(port, "Quad9 DoT", "9.9.9.9")
	s.UpdateDiscovery([]UpstreamMeta{quad9})

	for key, pending := range s.pending {
		if key.proxyPort == port || pending.port == port {
			t.Fatalf("old resolver pending query survived port reuse: %#v %#v", key, pending)
		}
	}
	if _, ok := s.clientStats[clientKey].resolvers[port]; ok {
		t.Fatal("client TopResolvers attribution survived port reuse")
	}

	// Complete the already-captured client-side transaction after the reuse.
	// It may still be marked as forwarded, but it must not materialize Quad9
	// merely because the numeric ndnproxy port now belongs to Quad9.
	s.RecordClientResponse(now.Add(50*time.Millisecond), "UDP", clientIP, "00:11:22:33:44:55", 40,
		DNSMessage{ID: 40, QR: true, QName: q.QName, QType: 1, RCode: 0})

	view, flows, ok := s.ClientDetail(clientIP, 10)
	if !ok {
		t.Fatal("client detail missing")
	}
	for _, named := range view.TopResolvers {
		if named.Name == "Quad9 DoT" {
			t.Fatalf("old client resolver count was relabeled as Quad9: %#v", view.TopResolvers)
		}
	}
	if len(flows) == 0 {
		t.Fatal("expected completed client flow")
	}
	if flows[0].Resolver == "Quad9 DoT" || flows[0].ResolverPort == port {
		t.Fatalf("old client flow was relabeled as Quad9: %#v", flows[0])
	}
}
