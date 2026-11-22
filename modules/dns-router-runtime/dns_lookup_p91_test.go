package dnsrouter

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

func TestP91DNSLookupFillsIPAndCertificate(t *testing.T) {
	origLookup, origInspect := lookupIPsP91, inspectTLSP91
	defer func() { lookupIPsP91 = origLookup; inspectTLSP91 = origInspect }()
	lookupIPsP91 = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("1.1.1.1"), net.ParseIP("8.8.8.8")}, nil
	}
	inspectTLSP91 = func(_ context.Context, req CertificateInspectRequestP90) (CertificateInspectResultP90, error) {
		if req.Address != "1.1.1.1:443" {
			return CertificateInspectResultP90{}, errors.New("wrong ip")
		}
		return CertificateInspectResultP90{Verified: true, PEM: "public-leaf", ServerName: req.ServerName}, nil
	}
	got, err := LookupDNSServerP91(context.Background(), DNSLookupRequestP91{Transport: "doh", ServerName: "cloudflare-dns.com"})
	if err != nil || got.Address != "1.1.1.1:443" || got.Certificate == nil || len(got.IPs) != 2 {
		t.Fatalf("result=%+v, error=%v", got, err)
	}
}

func TestP91LookupFailsClosedAndPlainUDP(t *testing.T) {
	orig := lookupIPsP91
	defer func() { lookupIPsP91 = orig }()
	lookupIPsP91 = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.2.0.1"), net.ParseIP("9.9.9.9")}, nil
	}
	for _, r := range []DNSLookupRequestP91{{Transport: "doh", ServerName: "https://bad.example"}, {Transport: "dot", ServerName: "127.0.0.1"}, {Transport: "bad", ServerName: "dns.example"}, {Transport: "udp", ServerName: "dns.example", PreferredIP: "1.1.1.1"}} {
		if _, err := LookupDNSServerP91(context.Background(), r); err == nil {
			t.Fatalf("accepted %+v", r)
		}
	}
	got, err := LookupDNSServerP91(context.Background(), DNSLookupRequestP91{Transport: "udp", ServerName: "dns.example"})
	if err != nil || got.Address != "10.2.0.1:53" || got.Certificate != nil {
		t.Fatalf("udp=%+v / %v", got, err)
	}
	if !strings.HasPrefix(got.Address, "10.2.0.1") {
		t.Fatal("incorrect IP")
	}
}
