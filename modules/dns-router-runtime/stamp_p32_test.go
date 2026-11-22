package dnsrouter

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestP32DNSStampFailClosed(t *testing.T) {
	valid := []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 11}
	valid = append(valid, []byte("9.9.9.9:443")...)
	valid = append(valid, 1, 'x')
	stamp := "sdns://" + base64.RawURLEncoding.EncodeToString(valid)
	addr, err := DNSStampEndpointP32(stamp)
	if err != nil || addr != "9.9.9.9:443" {
		t.Fatalf("got %s %v", addr, err)
	}
	if err := validateDNSStampP32(stamp, "8.8.8.8:443"); err == nil {
		t.Fatal("mismatched pin accepted")
	}
	for _, s := range []string{"sdns://", "sdns://AAAA", strings.Repeat("x", 5000)} {
		if _, err := DNSStampEndpointP32(s); err == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}
func TestP32Options(t *testing.T) {
	if err := validateP32Options(Upstream{Transport: "doh", HTTP3Mode: "only", BootstrapIPs: []string{"1.1.1.1", "9.9.9.9"}}); err != nil {
		t.Fatal(err)
	}
	for _, u := range []Upstream{{Transport: "udp", HTTP3Mode: "only"}, {Transport: "dot", BootstrapIPs: []string{"127.0.0.1"}}, {Transport: "doh", HTTP3Mode: "broken"}, {Transport: "doh", BootstrapIPs: []string{"1.1.1.1", "1.1.1.1"}}, {Transport: "stamp", Stamp: "sdns://broken", Address: "1.1.1.1:443"}} {
		if err := validateP32Options(u); err == nil {
			t.Fatalf("accepted %+v", u)
		}
	}
}

func TestP32SpecialAddressRejected(t *testing.T) {
	for _, address := range []string{"127.0.0.1:443", "100.64.1.1:443", "192.0.2.1:443", "203.0.113.7:443", "192.168.1.1:443"} {
		data := append([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, byte(len(address))}, []byte(address)...)
		data = append(data, 1, 'x')
		if _, err := DNSStampEndpointP32("sdns://" + base64.RawURLEncoding.EncodeToString(data)); err == nil {
			t.Fatalf("accepted special address %s", address)
		}
	}
}
func TestP32RejectsLostTLSPins(t *testing.T) {
	address := "9.9.9.9:443"
	data := append([]byte{2, 0, 0, 0, 0, 0, 0, 0, 0, byte(len(address))}, []byte(address)...)
	data = append(data, 32)
	data = append(data, make([]byte, 32)...)
	data = append(data, 0, 1, 'x')
	if _, err := DNSStampEndpointP32("sdns://" + base64.RawURLEncoding.EncodeToString(data)); err == nil {
		t.Fatal("silent TLS pin downgrade")
	}
}
