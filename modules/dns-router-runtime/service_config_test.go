package dnsrouter

import "testing"

func TestServiceDefaultValidationAndDNSServiceNames(t *testing.T) {
	c := EmptyConfig()
	c.Upstreams = []Upstream{{ID: "default", Transport: "udp", Address: "9.9.9.9:53"}}
	c.Rules = []Rule{{ID: "srv", Domain: "example.org", Match: "suffix", Upstream: "default"}}
	c.Service = &ServiceConfig{Listen: "127.0.0.1:55358", DefaultUpstream: "default", CacheBytes: 1 << 20, AllowedNetworks: []string{"127.0.0.0/8"}}
	e, err := Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.EvaluateDNSQuestion("_sip._tcp.example.org.")
	if err != nil || d.Upstream != "default" {
		t.Fatal(d, err)
	}
	if _, err = e.Evaluate("_sip._tcp.example.org"); err == nil {
		t.Fatal("offline host name validation was weakened")
	}
	c.Service.AllowedNetworks = []string{"192.168.1.1/8"}
	if _, err = Compile(c); err == nil {
		t.Fatal("public client network admitted")
	}
	c.Service.AllowedNetworks = []string{"127.0.0.0/8"}
	c.Service.DefaultUpstream = "missing"
	if _, err = Compile(c); err == nil {
		t.Fatal("missing default admitted")
	}
}
