package dnsrouter

import "testing"

func TestP243ScopeValidation(t *testing.T) {
	c := Config{Service: &ServiceConfig{IPIntelligence: &IPIntelligenceConfig{Enabled: true, Scope: "domains", Domains: []string{"example.org"}}}}
	if err := validateIPIntelligenceScope(c); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"localhost", "bad name", "Example.org", "example.org.", "home.arpa"} {
		c.Service.IPIntelligence.Domains = []string{s}
		if err := validateIPIntelligenceScope(c); err == nil {
			t.Fatal("accepted", s)
		}
	}
	c.Service.IPIntelligence.Domains = nil
	c.Service.IPIntelligence.Scope = "lists"
	c.Lists = []DomainList{{ID: "foo"}}
	c.Service.IPIntelligence.ListIDs = []string{"foo"}
	if err := validateIPIntelligenceScope(c); err != nil {
		t.Fatal(err)
	}
	c.Service.IPIntelligence.ListIDs = []string{"not-exist"}
	if err := validateIPIntelligenceScope(c); err == nil {
		t.Fatal("accepted missing list")
	}
	c.Service.IPIntelligence.Scope = "all"
	c.Service.IPIntelligence.ListIDs = nil
	if err := validateIPIntelligenceScope(c); err != nil {
		t.Fatal(err)
	}
	c.Service.IPIntelligence.Scope = ""
	if err := validateIPIntelligenceScope(c); err != nil {
		t.Fatal("legacy scope drift", err)
	}
}
