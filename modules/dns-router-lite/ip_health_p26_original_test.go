package lite

import (
	"bytes"
	"testing"

	"github.com/Fifth-Ace/routerforge/modules/dns-router-lite/internal/ipintel"
	"github.com/miekg/dns"
)

func TestP26OriginalMonitorsWithoutChangingAAnswer(t *testing.T) {
	q, answer := p242Answer("1.1.1.1", "8.8.8.8")
	before, err := answer.Pack()
	if err != nil {
		t.Fatal(err)
	}
	monitor := p242Monitor()
	got := applyIPIntelligenceP26(ipintel.New(7), monitor, q, answer, ipintel.Original, "default", "")
	if got != answer {
		t.Fatal("Original must return the exact original answer")
	}
	after, err := answer.Pack()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("Original mutated the DNS wire response")
	}
	if len(monitor.entries) != 2 || len(monitor.queue) != 2 {
		t.Fatalf("Original did not enqueue public IP addresses: %d/%d", len(monitor.entries), len(monitor.queue))
	}
}

func TestP26OriginalMonitorsAAAAWithoutChangingAnswer(t *testing.T) {
	q := new(dns.Msg)
	q.SetQuestion("www.example.org.", dns.TypeAAAA)
	answer := new(dns.Msg)
	answer.SetReply(q)
	for _, ip := range []string{"2606:4700:4700::1111", "2001:4860:4860::8888"} {
		rr, err := dns.NewRR("www.example.org. 60 IN AAAA " + ip)
		if err != nil {
			t.Fatal(err)
		}
		answer.Answer = append(answer.Answer, rr)
	}
	monitor := p242Monitor()
	got := applyIPIntelligenceP26(ipintel.New(8), monitor, q, answer, ipintel.Original, "default", "")
	if got != answer {
		t.Fatal("Original must not copy/modify AAAA answer")
	}
	if len(monitor.entries) != 2 || len(monitor.queue) != 2 {
		t.Fatal("Original did not sample AAAA records")
	}
}

func TestP26DisabledMonitorAndRotationUnaffected(t *testing.T) {
	q, answer := p242Answer("1.1.1.1", "8.8.8.8")
	original := p241IPList(answer, dns.TypeA)
	planner := ipintel.New(12)
	first := applyIPIntelligenceP26(planner, nil, q, answer, ipintel.RoundRobin, "default", "")
	second := applyIPIntelligenceP26(planner, nil, q, answer, ipintel.RoundRobin, "default", "")
	a := p241IPList(first, dns.TypeA)
	b := p241IPList(second, dns.TypeA)
	if len(a) != 2 || len(b) != 2 || a[0] == b[0] {
		t.Fatalf("Round Robin stopped rotating: first=%v next=%v", a, b)
	}
	if now := p241IPList(answer, dns.TypeA); now[0] != original[0] || now[1] != original[1] {
		t.Fatal("Round Robin mutated cached original")
	}
	if got := applyIPIntelligenceP26(planner, nil, q, answer, ipintel.Original, "default", ""); got != answer {
		t.Fatal("disabled/Original must not modify answer")
	}
}
