package main

import (
	"testing"
	"time"
)

func TestClientForwardedResponseOutcome(t *testing.T) {
	s := NewStore(32, 16)
	s.UpdateDiscovery([]UpstreamMeta{{Port: 40510, Profile: "System", Name: "Xbox DNS DoH", Protocol: "DoH"}})
	s.UpdateClientRegistry([]ClientInfo{{IP: "192.168.10.20", MAC: "02:00:00:00:00:20", Name: "Workstation", Policy: "System", Access: "Ethernet · port 1", Active: true}})
	now := time.Now()
	q := DNSMessage{ID: 123, QName: "chatgpt.com", QType: 1}
	s.RecordClientQuery(now, "UDP", "192.168.10.20", "02:00:00:00:00:20", q.ID, q)
	s.RecordQuery(now.Add(2*time.Millisecond), "UDP", 40510, 53000, q.ID, q)
	r := q
	r.QR = true
	r.RCode = 0
	s.RecordResponse(now.Add(20*time.Millisecond), 40510, 53000, q.ID, r, nil)
	s.RecordClientResponse(now.Add(24*time.Millisecond), "UDP", "192.168.10.20", "02:00:00:00:00:20", q.ID, r)

	clients := s.Clients(10)
	if len(clients) != 1 || clients[0].Forwarded != 1 || clients[0].ClientResponses != 1 || clients[0].CacheLocal != 0 {
		t.Fatalf("unexpected client counters: %#v", clients)
	}
	_, events, ok := s.ClientDetail("192.168.10.20", 10)
	if !ok || len(events) != 1 || events[0].Outcome != "FORWARDED" || events[0].Resolver != "Xbox DNS DoH" || events[0].RCode != "NOERROR" {
		t.Fatalf("unexpected client event: %#v", events)
	}
}

func TestClientCacheLocalResponseOutcome(t *testing.T) {
	s := NewStore(32, 16)
	s.UpdateClientRegistry([]ClientInfo{{IP: "192.168.10.30", MAC: "02:00:00:00:00:30", Name: "Desktop-PC", Policy: "System", Active: true}})
	now := time.Now()
	q := DNSMessage{ID: 55, QName: "example.com", QType: 1}
	s.RecordClientQuery(now, "UDP", "192.168.10.30", "02:00:00:00:00:30", q.ID, q)
	r := q
	r.QR = true
	s.RecordClientResponse(now.Add(700*time.Microsecond), "UDP", "192.168.10.30", "02:00:00:00:00:30", q.ID, r)

	clients := s.Clients(10)
	if len(clients) != 1 || clients[0].CacheLocal != 1 || clients[0].Forwarded != 0 || clients[0].ClientResponses != 1 {
		t.Fatalf("unexpected cache/local counters: %#v", clients)
	}
	_, events, _ := s.ClientDetail("192.168.10.30", 10)
	if len(events) != 1 || events[0].Outcome != "CACHE_LOCAL" {
		t.Fatalf("unexpected cache/local event: %#v", events)
	}
}

func TestClientNoResponseTimeout(t *testing.T) {
	s := NewStore(32, 16)
	s.UpdateClientRegistry([]ClientInfo{{IP: "192.168.10.50", MAC: "02:00:00:00:00:50", Name: "test", Policy: "Policy1", Active: true}})
	now := time.Now()
	q := DNSMessage{ID: 77, QName: "timeout.example", QType: 1}
	s.RecordClientQuery(now, "UDP", "192.168.10.50", "02:00:00:00:00:50", q.ID, q)
	s.CleanupTransient(now.Add(11*time.Second), nil)

	clients := s.Clients(10)
	if len(clients) != 1 || clients[0].ClientTimeouts != 1 {
		t.Fatalf("unexpected timeout counters: %#v", clients)
	}
	_, events, _ := s.ClientDetail("192.168.10.50", 10)
	if len(events) != 1 || events[0].Outcome != "CLIENT_TIMEOUT" {
		t.Fatalf("unexpected timeout event: %#v", events)
	}
}

func TestMeshAccessParsing(t *testing.T) {
	got := parseHotspot(`
             host:
                  mac: 02:00:00:00:00:83
                   ip: 192.168.10.83
                 name: Gaming-PC
            interface:
                       id: Bridge0
                     name: Home
               policy: Policy2
               active: yes
                  mws:
                      cid: 165cd48c-f5ac-11ec-8f23-738be0053a18
                     port: 1
`)
	if len(got) != 1 || !got[0].Mesh || got[0].MeshCID == "" || got[0].Access != "Ethernet · Mesh · port 1" {
		t.Fatalf("mesh parse failed: %#v", got)
	}
}
