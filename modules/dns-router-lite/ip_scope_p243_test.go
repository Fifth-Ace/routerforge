package lite

import (
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"testing"
)

func TestP243ScopeGlobalDomainsAndBoundary(t *testing.T) {
	all := newIPScopeMatcher(dr.Config{})
	if !all.Matches("example.org.") {
		t.Fatal("legacy global scope must remain active")
	}
	c := dr.Config{Service: &dr.ServiceConfig{IPIntelligence: &dr.IPIntelligenceConfig{Enabled: true, Scope: "domains", Domains: []string{"example.org"}}}}
	m := newIPScopeMatcher(c)
	for _, n := range []string{"example.org.", "WWW.Example.Org.", "a.b.example.org."} {
		if !m.Matches(n) {
			t.Fatal("missed", n)
		}
	}
	for _, n := range []string{"badexample.org.", "example.org.evil.", "other.org."} {
		if m.Matches(n) {
			t.Fatal("incorrectly matched", n)
		}
	}
}
func TestP243ScopeListExactSuffixSubdomains(t *testing.T) {
	c := dr.Config{Service: &dr.ServiceConfig{IPIntelligence: &dr.IPIntelligenceConfig{Enabled: true, Scope: "lists", ListIDs: []string{"selected"}}}, Rules: []dr.Rule{
		{ListID: "selected", Domain: "one.example", Match: "exact"},
		{ListID: "selected", Domain: "two.example", Match: "suffix"},
		{ListID: "selected", Domain: "three.example", Match: "subdomains"},
		{ListID: "foreign", Domain: "four.example", Match: "suffix"},
	}}
	m := newIPScopeMatcher(c)
	expected := map[string]bool{"one.example.": true, "a.one.example.": false, "two.example.": true, "a.two.example.": true, "three.example.": false, "a.three.example.": true, "four.example.": false}
	for n, want := range expected {
		if got := m.Matches(n); got != want {
			t.Fatalf("%s got=%v want=%v", n, got, want)
		}
	}
}
