package dnsrouter

import (
	"reflect"
	"testing"
)

func strictGroupFixture() Config {
	c := fixture()
	c.Upstreams = append(c.Upstreams, Upstream{ID: "third", Transport: "udp", Address: "1.1.1.1:53"})
	c.Upstreams[0].Fallbacks = []string{"third"}
	c.Groups = []DNSGroup{{ID: "only-dns-two", Name: "Только выбранные серверы", Servers: []string{"two"}}}
	c.Rules[0].Upstream = "two"
	c.Rules[0].GroupID = "only-dns-two"
	return c
}
func TestStrictGroupNeverUsesForeignFallback(t *testing.T) {
	e, err := Compile(strictGroupFixture())
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.Evaluate("www.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if d.GroupID != "only-dns-two" || d.Upstream != "two" {
		t.Fatalf("wrong decision: %+v", d)
	}
	chain, err := e.UpstreamChainForDecision(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 1 || chain[0].ID != "two" {
		t.Fatalf("DNS leaked into another group: %+v", chain)
	}
}
func TestExplicitGroupBackupOrder(t *testing.T) {
	c := strictGroupFixture()
	c.Groups[0].Servers = []string{"two", "one"}
	e, err := Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := e.Evaluate("www.example.com")
	chain, err := e.UpstreamChainForDecision(d)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, u := range chain {
		ids = append(ids, u.ID)
	}
	if !reflect.DeepEqual(ids, []string{"two", "one"}) {
		t.Fatalf("bad chain / implicit fallback: %v", ids)
	}
}
func TestStrictGroupRejectsInvalidReferences(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"unknown-group", func(c *Config) { c.Rules[0].GroupID = "missing" }},
		{"wrong-primary", func(c *Config) { c.Rules[0].Upstream = "one" }},
		{"missing-server", func(c *Config) { c.Groups[0].Servers = []string{"missing"} }},
		{"duplicate-server", func(c *Config) { c.Groups[0].Servers = []string{"two", "two"} }},
		{"empty-group", func(c *Config) { c.Groups[0].Servers = nil }},
		{"duplicate-group", func(c *Config) { c.Groups = append(c.Groups, c.Groups[0]) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := strictGroupFixture()
			tc.mutate(&c)
			if _, err := Compile(c); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
func TestLegacyConfigUnchanged(t *testing.T) {
	e, err := Compile(fixture())
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.Evaluate("www.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if d.GroupID != "" {
		t.Fatal("legacy decision gained group")
	}
	chain, err := e.UpstreamChainForDecision(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 1 || chain[0].ID != "one" {
		t.Fatalf("legacy chain changed %+v", chain)
	}
}
func TestNoGroupForUnmatchedDomain(t *testing.T) {
	e, err := Compile(strictGroupFixture())
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.Evaluate("other.net")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.UpstreamChainForDecision(d); err == nil {
		t.Fatal("unmatched domain reached upstream")
	}
}

func TestManagedListPreservesStrictGroup(t *testing.T) {
	c := strictGroupFixture()
	list := DomainList{ID: "only-video", Name: "Video", Kind: "paste", GroupID: "only-dns-two", Upstream: "two", Match: "suffix", Priority: 100}
	preview, err := PreviewManagedList(c, ManagedListRequest{List: list, Text: "video.example.net\n"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Config.Lists) != 1 || preview.Config.Lists[0].GroupID != "only-dns-two" {
		t.Fatalf("list group lost %+v", preview.Config.Lists)
	}
	e, err := Compile(preview.Config)
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.Evaluate("sub.video.example.net")
	if err != nil {
		t.Fatal(err)
	}
	if d.GroupID != "only-dns-two" || d.Upstream != "two" {
		t.Fatalf("group route lost %+v", d)
	}
}
func TestManagedListRejectsCrossGroupDuplicate(t *testing.T) {
	c := strictGroupFixture()
	c.Groups = append(c.Groups, DNSGroup{ID: "second-group", Name: "Other group", Servers: []string{"two"}})
	list := DomainList{ID: "second-list", Name: "Second", Kind: "paste", Upstream: "two", GroupID: "second-group", Match: "suffix", Priority: 10}
	_, err := PreviewManagedList(c, ManagedListRequest{List: list, Text: "example.com\n"})
	if err == nil {
		t.Fatal("duplicate strict domain silently accepted")
	}
}
func TestEqualPriorityGroupCollisionRejected(t *testing.T) {
	c := strictGroupFixture()
	c.Groups = append(c.Groups, DNSGroup{ID: "second-group", Name: "Other group", Servers: []string{"one"}})
	c.Rules = append(c.Rules, Rule{ID: "collision", Domain: "example.com", Match: "suffix", Upstream: "one", GroupID: "second-group", Priority: 10})
	if _, err := Compile(c); err == nil {
		t.Fatal("ambiguous strict group collision accepted")
	}
}
