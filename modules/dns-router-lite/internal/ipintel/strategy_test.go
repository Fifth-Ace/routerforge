package ipintel

import (
	"net/netip"
	"reflect"
	"sync"
	"testing"
	"time"
)

func ips() []netip.Addr {
	return []netip.Addr{netip.MustParseAddr("203.0.113.1"), netip.MustParseAddr("203.0.113.2"), netip.MustParseAddr("203.0.113.3")}
}
func TestOriginalDefaultAndNoInputMutation(t *testing.T) {
	p := New(1)
	a := ips()
	got := p.Order("", "x", "", a, nil)
	if !reflect.DeepEqual(got, a) {
		t.Fatal(got)
	}
	got[0] = netip.IPv4Unspecified()
	if a[0] == got[0] {
		t.Fatal("mutated source")
	}
}
func TestRoundRobinCyclePerKey(t *testing.T) {
	p := New(1)
	a := ips()
	for i := 0; i < 6; i++ {
		g := p.Order(RoundRobin, "q|A", "", a, nil)
		if g[0] != a[i%3] {
			t.Fatalf("cycle %d: %v", i, g)
		}
	}
	if p.Order(RoundRobin, "q|AAAA", "", a, nil)[0] != a[0] {
		t.Fatal("keys mixed")
	}
}
func TestRandomPermutation(t *testing.T) {
	p := New(7)
	a := ips()
	for i := 0; i < 30; i++ {
		g := p.Order(Random, "x", "", a, nil)
		seen := map[netip.Addr]bool{}
		for _, v := range g {
			seen[v] = true
		}
		if len(seen) != len(a) {
			t.Fatal("lost record")
		}
	}
}
func TestFastestPreservesUnknown(t *testing.T) {
	p := New(1)
	a := ips()
	h := map[netip.Addr]Health{a[2]: {Checked: true, Healthy: true, RTT: 9 * time.Millisecond}, a[0]: {Checked: true, Healthy: true, RTT: 40 * time.Millisecond}}
	g := p.Order(Fastest, "x", "", a, h)
	if g[0] != a[2] || g[1] != a[0] || g[2] != a[1] {
		t.Fatal(g)
	}
}
func TestHealthAwareAndAdaptive(t *testing.T) {
	p := New(0)
	a := ips()
	h := map[netip.Addr]Health{a[0]: {Checked: true, Healthy: false}, a[2]: {Checked: true, Healthy: true, RTT: 5 * time.Millisecond}}
	for _, mode := range []Mode{HealthAware, Adaptive} {
		g := p.Order(mode, "x", "", a, h)
		if g[0] != a[2] || g[2] != a[0] {
			t.Fatalf("%s: %v", mode, g)
		}
	}
}
func TestStickyClientDeterministic(t *testing.T) {
	p := New(1)
	a := ips()
	x := p.Order(Sticky, "q", "verifiedMAC", a, nil)
	for i := 0; i < 10; i++ {
		if !reflect.DeepEqual(x, p.Order(Sticky, "q", "verifiedMAC", a, nil)) {
			t.Fatal("unstable")
		}
	}
}
func TestWeightedPrefersHeavier(t *testing.T) {
	p := New(1)
	a := ips()
	h := map[netip.Addr]Health{a[0]: {Weight: 9}, a[1]: {Weight: 1}, a[2]: {Weight: 1}}
	counts := map[netip.Addr]int{}
	for i := 0; i < 110; i++ {
		g := p.Order(Weighted, "q", "", a, h)
		counts[g[0]]++
	}
	if counts[a[0]] <= counts[a[1]] {
		t.Fatal(counts)
	}
}
func TestHealthyOnlyAllDownFallsBack(t *testing.T) {
	p := New(1)
	a := ips()
	h := map[netip.Addr]Health{a[0]: {Checked: true, Healthy: false}, a[1]: {Checked: true, Healthy: true}, a[2]: {Checked: true, Healthy: false}}
	g := p.Order(HealthyOnly, "q", "", a, h)
	if len(g) != 1 || g[0] != a[1] {
		t.Fatal(g)
	}
	h[a[1]] = Health{Checked: true, Healthy: false}
	if !reflect.DeepEqual(p.Order(HealthyOnly, "q", "", a, h), a) {
		t.Fatal("empty-on-failure")
	}
}
func TestConcurrentNoRace(t *testing.T) {
	p := New(1)
	a := ips()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p.Order([]Mode{RoundRobin, Random, Adaptive, Weighted}[i%4], "q", "", a, nil)
		}(i)
	}
	wg.Wait()
}
func TestInvalidFallsBack(t *testing.T) {
	p := New(1)
	if !reflect.DeepEqual(p.Order("fake", "x", "", ips(), nil), ips()) {
		t.Fatal("bad mode")
	}
}

func TestHealthyOnlyNeverDropsUnknown(t *testing.T) {
	p := New(1)
	a := ips()
	h := map[netip.Addr]Health{a[0]: {Checked: true, Healthy: false}, a[1]: {Checked: true, Healthy: true}}
	g := p.Order(HealthyOnly, "q", "", a, h)
	want := []netip.Addr{a[1], a[2]}
	if !reflect.DeepEqual(g, want) {
		t.Fatalf("unknown IP must survive: got %v want %v", g, want)
	}
}
