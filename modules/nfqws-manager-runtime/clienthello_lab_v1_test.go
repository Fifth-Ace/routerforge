package main

import (
	"testing"
)

func TestGenerateClientHelloRoundTrip(t *testing.T) {
	data, err := v2GenerateClientHello("example.com")
	if err != nil {
		t.Fatal(err)
	}
	info := v2ValidateClientHello(data)
	if !info.Valid {
		t.Fatalf("invalid generated ClientHello: %+v", info)
	}
	if info.SNI != "example.com" {
		t.Fatalf("sni=%q", info.SNI)
	}
	if info.Size != len(data) || info.Size < 100 {
		t.Fatalf("size=%d", info.Size)
	}
}

func TestValidateClientHelloRejectsGarbage(t *testing.T) {
	info := v2ValidateClientHello([]byte{1, 2, 3})
	if info.Valid {
		t.Fatal("garbage accepted")
	}
}

func TestGenerateClientHelloRejectsIP(t *testing.T) {
	if _, err := v2GenerateClientHello("1.1.1.1"); err == nil {
		t.Fatal("IP SNI accepted")
	}
}

func TestGenerateClientHelloTLS13Options(t *testing.T) {
	data, err := v2GenerateClientHelloWithOptions("example.com", []string{"h2"}, 0x0304)
	if err != nil {
		t.Fatal(err)
	}
	info := v2ValidateClientHello(data)
	if !info.Valid || info.SNI != "example.com" {
		t.Fatalf("generated TLS1.3 ClientHello invalid: %+v", info)
	}
}

func TestGenerateClientHelloRejectsInvalidALPN(t *testing.T) {
	if _, err := v2GenerateClientHelloWithOptions("example.com", []string{"h2", "bad value"}, 0x0303); err == nil {
		t.Fatal("invalid ALPN accepted")
	}
}
