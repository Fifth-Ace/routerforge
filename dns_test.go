package main

import "testing"

func TestBuildAndParseDNSQuery(t *testing.T) {
	q := buildDNSQuery(0x1234, "chatgpt.com", 65)
	d, ok := parseDNSMessage(q)
	if !ok {
		t.Fatal("query did not parse")
	}
	if d.ID != 0x1234 || d.QName != "chatgpt.com" || d.QType != 65 || d.QR {
		t.Fatalf("unexpected parse: %#v", d)
	}
}
