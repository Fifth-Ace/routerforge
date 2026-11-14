package lite

import (
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
	"testing"
	"time"
)

func p33Question(id uint16, name string) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(name, dns.TypeA)
	m.Id = id
	return m
}
func p33Answer(q *dns.Msg, ttl uint32, ip string) *dns.Msg {
	r := new(dns.Msg)
	r.SetReply(q)
	rr, err := dns.NewRR(q.Question[0].Name + " " + "120 IN A " + ip)
	if err != nil {
		panic(err)
	}
	rr.Header().Ttl = ttl
	r.Answer = []dns.RR{rr}
	return r
}
func TestP33PolicyCacheIsolationAndTTL(t *testing.T) {
	now := time.Now()
	c := newPolicyCacheP33(1 << 20)
	route := dr.ClientDecision{Decision: dr.Decision{Upstream: "dns-1", RuleID: "rule-1"}, Layer: "device"}
	q := p33Question(1, "example.net.")
	a, ok := policyCacheKeyP33(route, "192.168.10.20", "02:00:00:00:00:01", q)
	if !ok {
		t.Fatal("eligible A request rejected")
	}
	c.Put(a, p33Answer(q, 5, "1.1.1.1"), now, 0, 0)
	fresh := p33Question(987, "example.net.")
	got, hit := c.Get(a, fresh, now.Add(2*time.Second))
	if !hit || got.Id != 987 || got.Answer[0].Header().Ttl != 3 {
		t.Fatalf("cache TTL/ID: %v, %v", hit, got)
	}
	if _, hit = c.Get(a, fresh, now.Add(6*time.Second)); hit {
		t.Fatal("expired entry returned")
	}
	other, valid := policyCacheKeyP33(route, "192.168.10.21", "02:00:00:00:00:02", q)
	if !valid || other == a {
		t.Fatal("client isolation failed")
	}
	altered := route
	altered.Upstream = "dns-2"
	different, _ := policyCacheKeyP33(altered, "192.168.10.20", "02:00:00:00:00:01", q)
	if different == a {
		t.Fatal("route isolation failed")
	}
	if _, hit = c.Get(other, q, now); hit {
		t.Fatal("different client hit")
	}
	c.Clear()
	if c.Stats()["entries"] != 0 {
		t.Fatal("clear left entries")
	}
}
func TestP33PrivacyAndBounds(t *testing.T) {
	route := dr.ClientDecision{}
	q := p33Question(1, "example.net.")
	q.SetEdns0(1232, true)
	if _, ok := policyCacheKeyP33(route, "192.168.10.20", "", q); ok {
		t.Fatal("DNSSEC DO must bypass cache")
	}
	q.SetEdns0(1232, false)
	q.IsEdns0().Option = append(q.IsEdns0().Option, &dns.EDNS0_SUBNET{Code: dns.EDNS0SUBNET})
	if _, ok := policyCacheKeyP33(route, "192.168.10.20", "", q); ok {
		t.Fatal("ECS must bypass cache")
	}
	q2 := p33Question(2, "example.net.")
	r := p33Answer(q2, 120, "1.1.1.1")
	r.AuthenticatedData = true
	if cacheTTLForP33(r, 0, 0) != 0 {
		t.Fatal("AD must bypass cache")
	}
	c := newPolicyCacheP33(2048)
	k, ok := policyCacheKeyP33(route, "192.168.10.20", "", q2)
	if !ok {
		t.Fatal("key not permitted")
	}
	c.Put(k, p33Answer(q2, 120, "1.1.1.1"), time.Now(), 0, 0)
	if c.Stats()["estimated_bytes"].(int) > c.Stats()["budget_bytes"].(int) {
		t.Fatal("memory budget exceeded")
	}
}
