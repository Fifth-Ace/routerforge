package dnsrouter

import (
	"fmt"
	"strings"
	"testing"
)

func fixture() Config {
	return Config{SchemaVersion: 1,
		Upstreams: []Upstream{{ID: "one", Transport: "udp", Address: "9.9.9.9:53"}, {ID: "two", Transport: "tcp", Address: "[2606:4700:4700::1111]:53"}},
		Rules:     []Rule{{ID: "parent", Domain: "example.com", Match: "suffix", Upstream: "one", Priority: 10}, {ID: "child", Domain: "api.example.com", Match: "exact", Upstream: "two", Priority: 20}, {ID: "subonly", Domain: "foo.org", Match: "subdomains", Upstream: "one", Priority: 1}},
	}
}

func TestRoutingAndLocalSafety(t *testing.T) {
	e, err := Compile(fixture())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ domain, upstream, source, rule string }{
		{"EXAMPLE.COM.", "one", "policy", "parent"},
		{"x.example.com", "one", "policy", "parent"},
		{"api.example.com", "two", "policy", "child"},
		{"foo.org", "", "system", ""},
		{"x.foo.org", "one", "policy", "subonly"},
		{"x.local", "", "system", ""},
		{"router.lan", "", "system", ""},
		{"1.168.192.in-addr.arpa", "", "system", ""},
		{"unmatched.net", "", "system", ""},
	}
	for _, tc := range cases {
		t.Run(tc.domain, func(t *testing.T) {
			d, err := e.Evaluate(tc.domain)
			if err != nil {
				t.Fatal(err)
			}
			if d.Upstream != tc.upstream || d.Source != tc.source || d.RuleID != tc.rule {
				t.Fatalf("unexpected decision: %+v", d)
			}
		})
	}
}

func TestInvalidConfigsFailClosed(t *testing.T) {
	mutators := []func(*Config){
		func(c *Config) { c.SchemaVersion = 9 },
		func(c *Config) { c.Rules[0].Domain = "bad..zone" },
		func(c *Config) { c.Rules[0].Domain = "local" },
		func(c *Config) { c.Rules[0].Domain = "home.arpa" },
		func(c *Config) { c.Rules[0].Upstream = "missing" },
		func(c *Config) { c.Rules[0].Match = "*" },
		func(c *Config) { c.Upstreams[0].Address = "127.0.0.1:53" },
		func(c *Config) { c.Upstreams[0].Transport = "doh" },
		func(c *Config) { c.Rules = append(c.Rules, c.Rules[0]) },
		func(c *Config) { c.Upstreams = append(c.Upstreams, c.Upstreams[0]) },
		func(c *Config) { c.Rules[0].Priority = -1 },
	}
	for i, mutate := range mutators {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			c := fixture()
			mutate(&c)
			if _, err := Compile(c); err == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}
}

func TestPrecedenceAndDeterminism(t *testing.T) {
	c := fixture()
	c.Rules = append(c.Rules, Rule{ID: "alpha", Domain: "example.com", Match: "suffix", Upstream: "two", Priority: 0})
	e, err := Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.Evaluate("a.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if d.RuleID != "alpha" {
		t.Fatalf("priority tie lost: %+v", d)
	}
	d, err = e.Evaluate("api.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if d.RuleID != "child" {
		t.Fatalf("specificity lost: %+v", d)
	}
}

func TestTenThousandRules(t *testing.T) {
	c := Config{SchemaVersion: 1, Upstreams: []Upstream{{ID: "one", Transport: "udp", Address: "9.9.9.9:53"}}}
	for i := 0; i < MaxRules; i++ {
		c.Rules = append(c.Rules, Rule{ID: fmt.Sprintf("r%d", i), Domain: fmt.Sprintf("host%d.example.com", i), Match: "exact", Upstream: "one"})
	}
	e, err := Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{0, 4231, 9999} {
		d, err := e.Evaluate(fmt.Sprintf("host%d.example.com", i))
		if err != nil || !d.Matched {
			t.Fatalf("rule %d: %+v %v", i, d, err)
		}
	}
	if _, err := e.Evaluate(strings.Repeat("x", 64) + ".example.com"); err == nil {
		t.Fatal("invalid label accepted")
	}
}

func BenchmarkTenThousandRules(b *testing.B) {
	c := Config{SchemaVersion: 1, Upstreams: []Upstream{{ID: "one", Transport: "udp", Address: "9.9.9.9:53"}}}
	for i := 0; i < MaxRules; i++ {
		c.Rules = append(c.Rules, Rule{ID: fmt.Sprintf("r%d", i), Domain: fmt.Sprintf("host%d.example.com", i), Match: "exact", Upstream: "one"})
	}
	e, err := Compile(c)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := e.Evaluate("host9000.example.com")
		if err != nil {
			b.Fatal(err)
		}
	}
}
