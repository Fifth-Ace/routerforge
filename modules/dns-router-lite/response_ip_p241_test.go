package lite

import (
	"bytes"
	"context"
	"net/netip"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/Fifth-Ace/routerforge/modules/dns-router-lite/internal/ipintel"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

func p241Question(name string, kind uint16) *dns.Msg {
	q := new(dns.Msg)
	q.SetQuestion(name, kind)
	return q
}
func p241Reply(q *dns.Msg, records ...string) *dns.Msg {
	a := new(dns.Msg)
	a.SetReply(q)
	for _, record := range records {
		rr, err := dns.NewRR(record)
		if err != nil {
			panic(err)
		}
		a.Answer = append(a.Answer, rr)
	}
	return a
}
func p241IPList(a *dns.Msg, kind uint16) []string {
	out := []string{}
	for _, rr := range a.Answer {
		if ip, ok := responseIP(rr, kind); ok {
			out = append(out, ip.String())
		}
	}
	return out
}

// p241SnapshotWire compares the actual DNS wire message, including record
// order, contents and TTL. dns.Msg.Copy() allocates non-nil empty Ns/Extra
// slices, so reflect.DeepEqual(original, original.Copy()) is NOT a valid
// mutation check when the original has nil sections.
func p241SnapshotWire(t *testing.T, a *dns.Msg) []byte {
	t.Helper()
	wire, err := a.Pack()
	if err != nil {
		t.Fatal(err)
	}
	return wire
}
func p241TwoA() (*dns.Msg, *dns.Msg) {
	q := p241Question("www.example.", dns.TypeA)
	a := p241Reply(q,
		"www.example. 60 IN CNAME edge.example.",
		"edge.example. 60 IN A 203.0.113.10",
		"edge.example. 60 IN A 203.0.113.20",
		"edge.example. 60 IN A 203.0.113.30")
	return q, a
}
func TestP241RoundRobinDeepCopyCacheAndCNAME(t *testing.T) {
	q, a := p241TwoA()
	p := ipintel.New(9)
	old := p241SnapshotWire(t, a)
	want := [][]string{
		{"203.0.113.10", "203.0.113.20", "203.0.113.30"},
		{"203.0.113.20", "203.0.113.30", "203.0.113.10"},
		{"203.0.113.30", "203.0.113.10", "203.0.113.20"},
		{"203.0.113.10", "203.0.113.20", "203.0.113.30"},
	}
	for i := range want {
		got := reorderDNSAnswer(p, q, a, ipintel.RoundRobin, "default", "", nil)
		if got == a {
			t.Fatal("response must be a copy")
		}
		if !reflect.DeepEqual(p241IPList(got, dns.TypeA), want[i]) {
			t.Fatalf("response %d: %v", i, p241IPList(got, dns.TypeA))
		}
		if !reflect.DeepEqual(got.Answer[0], a.Answer[0]) {
			t.Fatal("CNAME was changed")
		}
		for _, rr := range got.Answer {
			if rr.Header().Ttl != 60 {
				t.Fatal("TTL changed")
			}
		}
		if !bytes.Equal(p241SnapshotWire(t, a), old) {
			t.Fatal("cached upstream answer mutated")
		}
	}
}

// Regression: modifying an outgoing reordered record must never change
// the original upstream message, even through its nested net.IP byte slice.
func TestP241ReorderedAnswerOwnsIndependentIPBytes(t *testing.T) {
	q, original := p241TwoA()
	before := p241SnapshotWire(t, original)
	modified := reorderDNSAnswer(ipintel.New(1), q, original, ipintel.RoundRobin, "default", "", nil)
	if modified == original {
		t.Fatal("transformed reply shares the original message pointer")
	}
	address, ok := modified.Answer[1].(*dns.A)
	if !ok || len(address.A) == 0 {
		t.Fatal("expected first A record after CNAME")
	}
	address.A[0] ^= 1
	if bytes.Equal(p241SnapshotWire(t, modified), before) {
		t.Fatal("mutating output should change the output")
	}
	if !bytes.Equal(p241SnapshotWire(t, original), before) {
		t.Fatal("returned RR aliases the upstream source")
	}
}
func TestP241AAAAAndNonAddressUnchanged(t *testing.T) {
	q := p241Question("a.example.", dns.TypeAAAA)
	a := p241Reply(q, "a.example. 120 IN AAAA 2001:db8::1", "a.example. 120 IN AAAA 2001:db8::2")
	p := ipintel.New(1)
	x := reorderDNSAnswer(p, q, a, ipintel.RoundRobin, "default", "", nil)
	y := reorderDNSAnswer(p, q, a, ipintel.RoundRobin, "default", "", nil)
	if p241IPList(x, dns.TypeAAAA)[0] != "2001:db8::1" || p241IPList(y, dns.TypeAAAA)[0] != "2001:db8::2" {
		t.Fatal("AAAA rotation failed")
	}
	txt := p241Question("a.example.", dns.TypeTXT)
	answer := p241Reply(txt, "a.example. 60 IN TXT \"hello\"")
	if reorderDNSAnswer(p, txt, answer, ipintel.RoundRobin, "default", "", nil) != answer {
		t.Fatal("TXT must not be touched")
	}
	if reorderDNSAnswer(p, q, a, ipintel.Original, "default", "", nil) != a {
		t.Fatal("Original must not copy/modify")
	}
	if reorderDNSAnswer(nil, q, a, ipintel.RoundRobin, "default", "", nil) != a {
		t.Fatal("nil planner must be inert")
	}
}
func TestP241DNSSECGuardAndNegativeReplies(t *testing.T) {
	q, a := p241TwoA()
	p := ipintel.New(1)
	m := map[netip.Addr]ipintel.Health{
		netip.MustParseAddr("203.0.113.10"): {Checked: true, Healthy: false},
		netip.MustParseAddr("203.0.113.20"): {Checked: true, Healthy: true},
		netip.MustParseAddr("203.0.113.30"): {Checked: true, Healthy: false},
	}
	filtered := reorderDNSAnswer(p, q, a, ipintel.HealthyOnly, "", "", m)
	if len(p241IPList(filtered, dns.TypeA)) != 1 {
		t.Fatal("unsigned eligible RRset not filtered")
	}
	if len(p241IPList(a, dns.TypeA)) != 3 {
		t.Fatal("source reply changed")
	}
	q.SetEdns0(1232, true)
	preserved := reorderDNSAnswer(p, q, a, ipintel.HealthyOnly, "", "", m)
	if len(p241IPList(preserved, dns.TypeA)) != 3 {
		t.Fatal("DNSSEC DO reply was filtered")
	}
	q = p241Question("www.example.", dns.TypeA)
	a.AuthenticatedData = true
	preserved = reorderDNSAnswer(p, q, a, ipintel.HealthyOnly, "", "", m)
	if len(p241IPList(preserved, dns.TypeA)) != 3 {
		t.Fatal("authenticated reply was filtered")
	}
	a.AuthenticatedData = false
	a.Rcode = dns.RcodeNameError
	if reorderDNSAnswer(p, q, a, ipintel.RoundRobin, "", "", nil) != a {
		t.Fatal("negative reply must not change")
	}
	a.Rcode = dns.RcodeSuccess
	a.Answer = append(a.Answer, &dns.RRSIG{Hdr: dns.RR_Header{Name: "edge.example.", Rrtype: dns.TypeRRSIG, Class: dns.ClassINET, Ttl: 60}, TypeCovered: dns.TypeA})
	signed := reorderDNSAnswer(p, q, a, ipintel.HealthyOnly, "", "", m)
	if len(p241IPList(signed, dns.TypeA)) != 3 {
		t.Fatal("signed RRset was filtered")
	}
	a.Truncated = true
	if reorderDNSAnswer(p, q, a, ipintel.RoundRobin, "", "", nil) != a {
		t.Fatal("truncated reply must not change")
	}
}
func TestP241SeparateKeysAndTrustedSticky(t *testing.T) {
	q, a := p241TwoA()
	p := ipintel.New(9)
	first := reorderDNSAnswer(p, q, a, ipintel.RoundRobin, "policy-a", "", nil)
	second := reorderDNSAnswer(p, q, a, ipintel.RoundRobin, "policy-b", "", nil)
	if !reflect.DeepEqual(p241IPList(first, dns.TypeA), p241IPList(second, dns.TypeA)) {
		t.Fatal("policy keys share cursor")
	}
	sticky := reorderDNSAnswer(p, q, a, ipintel.Sticky, "policy-a", "02:11:22:33:44:55", nil)
	for i := 0; i < 10; i++ {
		same := reorderDNSAnswer(p, q, a, ipintel.Sticky, "policy-a", "02:11:22:33:44:55", nil)
		if !reflect.DeepEqual(p241IPList(sticky, dns.TypeA), p241IPList(same, dns.TypeA)) {
			t.Fatal("verified sticky not deterministic")
		}
	}
}
func TestP241ConcurrentResponseIsolation(t *testing.T) {
	q, a := p241TwoA()
	snapshot := p241SnapshotWire(t, a)
	p := ipintel.New(1)
	var wg sync.WaitGroup
	for i := 0; i < 128; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v := reorderDNSAnswer(p, q, a, ipintel.RoundRobin, "x", "", nil)
			if len(v.Answer) != 4 {
				t.Error("lost records")
			}
		}()
	}
	wg.Wait()
	if !bytes.Equal(p241SnapshotWire(t, a), snapshot) {
		t.Fatal("concurrent caller modified upstream result")
	}
}

// End-to-end in-process UDP/TCP fixture: two successive client requests must
// see distinct first A records even when the upstream answer is DNSProxy-cached.
type p241Upstream struct{ calls atomic.Int32 }

func (u *p241Upstream) Address() string { return "p241-fixture" }
func (u *p241Upstream) Close() error    { return nil }
func (u *p241Upstream) Exchange(ctx context.Context, q *dns.Msg) (*dns.Msg, error) {
	u.calls.Add(1)
	if q.Question[0].Qtype == dns.TypeAAAA {
		return p241Reply(q,
			q.Question[0].Name+" 60 IN AAAA 2001:db8::10",
			q.Question[0].Name+" 60 IN AAAA 2001:db8::20"), nil
	}
	return p241Reply(q,
		q.Question[0].Name+" 60 IN A 203.0.113.10",
		q.Question[0].Name+" 60 IN A 203.0.113.20"), nil
}
func TestP241EngineActualUDPAndTCPWithCache(t *testing.T) {
	c := fixtureConfig(t)
	c.Service.CacheBytes = 65536
	c.Service.IPIntelligence = &dr.IPIntelligenceConfig{Enabled: true, Mode: "round_robin"}
	u := &p241Upstream{}
	factory := func(dr.Upstream) (upstream.Upstream, error) { return u, nil }
	e := startFixture(t, c, factory)
	q := p241Question("multi.outside.example.", dns.TypeA)
	expected := []string{"203.0.113.10", "203.0.113.20", "203.0.113.10", "203.0.113.20"}
	for i, kind := range []string{"udp", "tcp", "udp", "tcp"} {
		got, _, err := (&dns.Client{Net: kind, Timeout: 3 * time.Second}).Exchange(q, c.Service.Listen)
		if err != nil {
			t.Fatal(err)
		}
		if first := p241IPList(got, dns.TypeA)[0]; first != expected[i] {
			t.Fatalf("%s request %d: got %s want %s", kind, i, first, expected[i])
		}
	}
	v6 := p241Question("multi.outside.example.", dns.TypeAAAA)
	for i, kind := range []string{"udp", "tcp"} {
		got, _, err := (&dns.Client{Net: kind, Timeout: 3 * time.Second}).Exchange(v6, c.Service.Listen)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"2001:db8::10", "2001:db8::20"}[i]
		if first := p241IPList(got, dns.TypeAAAA)[0]; first != want {
			t.Fatalf("AAAA %s got %s want %s", kind, first, want)
		}
	}
	if e.cacheHits.Load() == 0 {
		t.Fatal("cached client DNS answers were not rotated")
	}
	if u.calls.Load() > 3 {
		t.Fatal("IP ordering bypassed DNSProxy cache")
	}
}
func TestP241DisabledPreservesCacheAndAnswer(t *testing.T) {
	c := fixtureConfig(t)
	u := &p241Upstream{}
	e := startFixture(t, c, func(dr.Upstream) (upstream.Upstream, error) { return u, nil })
	q := p241Question("multi.outside.example.", dns.TypeA)
	for _, kind := range []string{"udp", "tcp", "udp"} {
		a, _, err := (&dns.Client{Net: kind, Timeout: 3 * time.Second}).Exchange(q, c.Service.Listen)
		if err != nil {
			t.Fatal(err)
		}
		if got := p241IPList(a, dns.TypeA); !reflect.DeepEqual(got, []string{"203.0.113.10", "203.0.113.20"}) {
			t.Fatal(got)
		}
	}
	if e.cacheHits.Load() == 0 {
		t.Fatal("old cache path changed")
	}
}
