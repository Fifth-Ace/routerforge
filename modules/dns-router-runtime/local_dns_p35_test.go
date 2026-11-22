package dnsrouter

import (
	"net/netip"
	"strings"
	"testing"
)

func TestP35DisabledInert(t *testing.T) {
	if err := validateLocalDNSP35(nil); err != nil {
		t.Fatal(err)
	}
	if err := validateLocalDNSP35(&LocalDNSConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := validateLocalDNSP35(&LocalDNSConfig{Records: []LocalDNSRecord{{Name: "nas.rf.home.arpa", Type: "A", Value: "192.168.10.8", TTL: 300}}}); err == nil {
		t.Fatal("disabled records accepted")
	}
}
func TestP35ValidationAndProtectedNamespaces(t *testing.T) {
	valid := &LocalDNSConfig{Enabled: true, Records: []LocalDNSRecord{
		{Name: "nas.rf.home.arpa", Type: "A", Value: "192.168.10.8", TTL: 300},
		{Name: "alias.rf.home.arpa", Type: "CNAME", Value: "nas.rf.home.arpa", TTL: 300},
		{Name: "8.10.168.192.in-addr.arpa", Type: "PTR", Value: "nas.rf.home.arpa", TTL: 300},
	}}
	if err := validateLocalDNSP35(valid); err != nil {
		t.Fatal(err)
	}
	if err := validateLocalDNSP35(valid, "192.168.10.8:55358"); err == nil {
		t.Fatal("PTR of the DNS listener must remain on the stock resolver")
	}
	for _, r := range []LocalDNSRecord{
		{Name: "router.lan", Type: "A", Value: "192.168.10.8", TTL: 300},
		{Name: "my.keenetic.net", Type: "A", Value: "192.168.10.8", TTL: 300},
		{Name: "nas.home.arpa", Type: "A", Value: "192.168.10.8", TTL: 300},
		{Name: "nas.rf.home.arpa", Type: "A", Value: "8.8.8.8", TTL: 300},
		{Name: "nas.rf.home.arpa", Type: "AAAA", Value: "fe80::1", TTL: 300},
		{Name: "nas.rf.home.arpa", Type: "A", Value: "192.168.10.8", TTL: 0},
		{Name: "random.in-addr.arpa", Type: "PTR", Value: "nas.rf.home.arpa", TTL: 300},
	} {
		if err := validateLocalDNSP35(&LocalDNSConfig{Enabled: true, Records: []LocalDNSRecord{r}}); err == nil {
			t.Fatalf("accepted unsafe record %#v", r)
		}
	}
	cycle := &LocalDNSConfig{Enabled: true, Records: []LocalDNSRecord{{Name: "a.rf.home.arpa", Type: "CNAME", Value: "b.rf.home.arpa", TTL: 300}, {Name: "b.rf.home.arpa", Type: "CNAME", Value: "a.rf.home.arpa", TTL: 300}}}
	if err := validateLocalDNSP35(cycle); err == nil {
		t.Fatal("accepted CNAME cycle")
	}
}
func TestP35ReverseIPv4IPv6(t *testing.T) {
	n, e := ReverseDNSNameP35(netip.MustParseAddr("192.168.10.8"))
	if e != nil || n != "8.10.168.192.in-addr.arpa" {
		t.Fatalf("%s %v", n, e)
	}
	n, e = ReverseDNSNameP35(netip.MustParseAddr("fd00::1"))
	if e != nil || !strings.HasSuffix(n, ".ip6.arpa") || len(strings.Split(n, ".")) != 34 {
		t.Fatalf("%s %v", n, e)
	}
	if _, e = ReverseDNSNameP35(netip.MustParseAddr("1.1.1.1")); e == nil {
		t.Fatal("public PTR accepted")
	}
}
