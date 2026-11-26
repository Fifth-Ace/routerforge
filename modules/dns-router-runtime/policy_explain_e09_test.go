package dnsrouter

import (
	"testing"
	"time"
)

func TestE09NativeExplanation(t *testing.T) {
	c := Config{SchemaVersion: SchemaVersion, Upstreams: []Upstream{{ID: "one", Transport: "udp", Address: "192.0.2.1:53"}, {ID: "two", Transport: "udp", Address: "192.0.2.2:53"}}, Rules: []Rule{{ID: "parent", Domain: "example.org", Match: "suffix", Upstream: "one"}, {ID: "z", Domain: "a.example.org", Match: "exact", Priority: 4, Upstream: "one"}, {ID: "a", Domain: "a.example.org", Match: "exact", Priority: 4, Upstream: "two"}}}
	e, err := Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	x, err := e.ExplainE09("a.example.org", "")
	if err != nil || x.Decision.RuleID != "a" || x.Shared.Total != 3 || len(x.AllowedUpstreams) != 1 || x.AllowedUpstreams[0] != "two" {
		t.Fatalf("%+v %v", x, err)
	}
	x, err = e.ExplainE09("my.keenetic.net", "")
	if err != nil || x.Decision.Layer != "local_namespace" || len(x.AllowedUpstreams) != 0 {
		t.Fatalf("%+v %v", x, err)
	}
	if _, err = e.ExplainE09("bad..example.org", ""); err == nil {
		t.Fatal("bad domain accepted")
	}
	if _, err = e.ExplainE09("example.org", "bad"); err == nil {
		t.Fatal("bad MAC accepted")
	}
}
func TestE09PolicyAudit(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	c := Config{Rules: []Rule{{ID: "a", ListID: "list"}}, Lists: []DomainList{{ID: "list", UpdatedAt: "2026-11-26T11:00:00Z"}, {ID: "future", UpdatedAt: "2026-11-27T11:00:00Z"}}, Service: &ServiceConfig{Privacy: &DNSPrivacyConfig{Enabled: true, DNSSECMode: "require_ad"}, DNS64: &DNS64Config{Enabled: true, Prefix: "64:ff9b::/96", NAT64Confirmed: true}, Performance: &DNSPerformanceConfig{Enabled: true, CacheMinTTL: 20, CacheMaxTTL: 90}}}
	x := AuditPolicyE09(c, now)
	if x.LocalValidation || x.DNSSECMode != "require_ad" || !x.NAT64Declared || x.Lists[0].Rules != 1 || x.Lists[0].AgeSeconds == nil || *x.Lists[0].AgeSeconds != 3600 || x.Lists[1].AgeSeconds != nil || x.CacheMinTTL == nil || *x.CacheMinTTL != 20 {
		t.Fatalf("%+v", x)
	}
}

func TestE09DeviceRuleAndStrictChain(t *testing.T) {
	mac := "02:11:22:33:44:55"
	c := Config{SchemaVersion: SchemaVersion, Upstreams: []Upstream{{ID: "one", Transport: "udp", Address: "192.0.2.1:53"}, {ID: "two", Transport: "udp", Address: "192.0.2.2:53"}, {ID: "three", Transport: "udp", Address: "192.0.2.3:53"}}, Groups: []DNSGroup{{ID: "explicit", Name: "Explicit", Servers: []string{"two", "three"}}}, Rules: []Rule{{ID: "common", Domain: "example.org", Match: "suffix", Upstream: "one"}}, Advanced: &AdvancedDNS{Rules: []DeviceDomainRule{{ID: "device", MAC: mac, Domain: "example.org", Match: "suffix", Target: DeviceTarget{Upstream: "two", GroupID: "explicit"}}}}}
	e, err := Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	x, err := e.ExplainE09("example.org", mac)
	if err != nil || !x.SimulatedMAC || x.Decision.Layer != "device_domain" || x.Decision.RuleID != "device" || x.Device.Total != 1 || x.Shared.Total != 1 || len(x.AllowedUpstreams) != 2 || x.AllowedUpstreams[0] != "two" || x.AllowedUpstreams[1] != "three" {
		t.Fatalf("%+v %v", x, err)
	}
}
