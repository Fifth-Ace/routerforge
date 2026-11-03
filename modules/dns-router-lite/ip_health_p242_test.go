package lite

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/Fifth-Ace/routerforge/modules/dns-router-lite/internal/ipintel"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

func p242ViewFor(t *testing.T, m *IPHealthMonitor, address string) IPHealthView {
	t.Helper()
	for _, row := range m.Snapshot() {
		if row.Address == address {
			return row
		}
	}
	t.Fatalf("IP not present in monitor: %s", address)
	return IPHealthView{}
}
func p242Monitor() *IPHealthMonitor {
	return newIPHealthMonitor(&dr.IPHealthConfig{Enabled: true, Protocol: "tcp", Port: 443, IntervalSeconds: 60, TimeoutMS: 1000, FailureThreshold: 2})
}
func p242Answer(addresses ...string) (*dns.Msg, *dns.Msg) {
	q := new(dns.Msg)
	q.SetQuestion("www.example.org.", dns.TypeA)
	a := new(dns.Msg)
	a.SetReply(q)
	for _, ip := range addresses {
		a.Answer = append(a.Answer, &dns.A{Hdr: dns.RR_Header{Name: q.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: netip.MustParseAddr(ip).AsSlice()})
	}
	return q, a
}
func TestP242DenyPrivateReservedAndMappedAddresses(t *testing.T) {
	denied := []string{"0.0.0.0", "10.0.0.1", "100.100.100.100", "127.0.0.1", "169.254.1.2", "172.16.1.1", "192.168.1.1", "192.0.2.1", "198.18.0.1", "198.51.100.3", "203.0.113.3", "224.0.0.1", "255.255.255.255", "::1", "fe80::1", "fc00::1", "2001:db8::1", "::ffff:8.8.8.8", "2002::1", "64:ff9b::808:808"}
	for _, s := range denied {
		if ipProbePermitted(netip.MustParseAddr(s)) {
			t.Fatalf("reserved IP permitted: %s", s)
		}
	}
	for _, s := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111", "2001:4860:4860::8888"} {
		if !ipProbePermitted(netip.MustParseAddr(s)) {
			t.Fatalf("public IP denied: %s", s)
		}
	}
}
func TestP242ObserveQueueNonBlockingAndDedupe(t *testing.T) {
	m := p242Monitor()
	q, a := p242Answer("1.1.1.1", "8.8.8.8", "10.0.0.1")
	for i := 0; i < 800; i++ {
		if got := m.Observe(q, a); len(got) != 0 {
			t.Fatal("unprobed addresses should remain UNKNOWN")
		}
	}
	if len(m.entries) != 2 || len(m.queue) != 2 {
		t.Fatalf("queue/map not deduplicated: %d/%d", len(m.queue), len(m.entries))
	}
	// Simulate more than 512 distinct domains - memory stays bounded.
	for i := 0; i < 800; i++ {
		q.Question[0].Name = "host" + ipHealthTestDigits(i) + ".example.org."
		m.Observe(q, a)
	}
	if len(m.entries) > maxIPHealthEntries || len(m.queue) > maxIPHealthQueued {
		t.Fatal("unbounded health tracker")
	}
}
func ipHealthTestDigits(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
func TestP242FailuresRecoveriesAndStaleSamples(t *testing.T) {
	m := p242Monitor()
	target := ipProbeTarget{IP: netip.MustParseAddr("1.1.1.1"), Domain: "www.example.org", Port: 443, Protocol: "tcp"}
	q, a := p242Answer("1.1.1.1", "8.8.8.8")
	m.Observe(q, a)
	failed := true
	m.probe = func(context.Context, ipProbeTarget) (time.Duration, error) {
		if failed {
			return 20 * time.Millisecond, errors.New("test unavailable")
		}
		return 10 * time.Millisecond, nil
	}
	m.measure(context.Background(), target)
	if h := m.Observe(q, a); len(h) != 0 {
		t.Fatal("one failed probe must not mark DOWN")
	}
	if p242ViewFor(t, m, "1.1.1.1").LastError == "" {
		t.Fatal("missing last probe error")
	}
	m.measure(context.Background(), target)
	if h := m.Observe(q, a); !h[target.IP].Checked || h[target.IP].Healthy {
		t.Fatalf("expected DOWN after failure threshold: %v", h)
	}
	failed = false
	m.measure(context.Background(), target)
	if h := m.Observe(q, a); h[target.IP].Healthy {
		t.Fatal("early recovery marked UP")
	}
	m.measure(context.Background(), target)
	if h := m.Observe(q, a); !h[target.IP].Healthy || h[target.IP].RTT <= 0 {
		t.Fatalf("recovery failed: %v", h)
	}
	if p242ViewFor(t, m, "1.1.1.1").LastSuccess == "" {
		t.Fatal("missing last success timestamp")
	}
	m.mu.Lock()
	m.entries[target].CheckedAt = time.Now().Add(-12 * time.Minute)
	m.mu.Unlock()
	if h := m.Observe(q, a); h[target.IP].Checked {
		t.Fatal("stale observations influenced DNS")
	}
}
func TestP242HealthFeedsOrderingWithoutBlockingDNS(t *testing.T) {
	m := p242Monitor()
	q, a := p242Answer("1.1.1.1", "8.8.8.8")
	m.Observe(q, a)
	first := ipProbeTarget{IP: netip.MustParseAddr("1.1.1.1"), Domain: "www.example.org", Port: 443, Protocol: "tcp"}
	second := ipProbeTarget{IP: netip.MustParseAddr("8.8.8.8"), Domain: "www.example.org", Port: 443, Protocol: "tcp"}
	m.mu.Lock()
	m.entries[first].Checked = true
	m.entries[first].Healthy = false
	m.entries[first].CheckedAt = time.Now()
	m.entries[second].Checked = true
	m.entries[second].Healthy = true
	m.entries[second].CheckedAt = time.Now()
	m.entries[second].RTT = 20 * time.Millisecond
	m.mu.Unlock()
	h := m.Observe(q, a)
	if !h[first.IP].Checked || h[first.IP].Healthy || !h[second.IP].Healthy {
		t.Fatalf("metrics: %v", h)
	}
	out := reorderDNSAnswer(ipintel.New(5), q, a, ipintel.HealthAware, "default", "", h)
	ips := p241IPList(out, dns.TypeA)
	if ips[0] != "8.8.8.8" || ips[1] != "1.1.1.1" {
		t.Fatalf("health priority: %v", ips)
	}
	filtered := reorderDNSAnswer(ipintel.New(5), q, a, ipintel.HealthyOnly, "default", "", h)
	if x := p241IPList(filtered, dns.TypeA); len(x) != 1 || x[0] != "8.8.8.8" {
		t.Fatalf("healthy only: %v", x)
	}
	if x := p241IPList(a, dns.TypeA); len(x) != 2 || x[0] != "1.1.1.1" {
		t.Fatal("cached answer mutated")
	}
}
func TestP242StopCancelsWorkers(t *testing.T) {
	m := p242Monitor()
	m.probe = func(ctx context.Context, _ ipProbeTarget) (time.Duration, error) { <-ctx.Done(); return 0, ctx.Err() }
	m.Start()
	q, a := p242Answer("1.1.1.1")
	m.Observe(q, a)
	done := make(chan struct{})
	go func() { m.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("worker shutdown blocked")
	}
}
func TestP242ObserveConcurrentBounded(t *testing.T) {
	m := p242Monitor()
	q, a := p242Answer("1.1.1.1", "8.8.8.8")
	var wg sync.WaitGroup
	for i := 0; i < 128; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); m.Observe(q, a); _ = m.Snapshot() }()
	}
	wg.Wait()
	if len(m.entries) != 2 || len(m.queue) != 2 {
		t.Fatal("concurrent queue duplication")
	}
}
