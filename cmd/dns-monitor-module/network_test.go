package main

import "testing"

func TestDecodeIPv4RouteHex(t *testing.T) {
	got, ok := decodeIPv4RouteHex("0101A8C0")
	if !ok || got != "192.168.1.1" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	got, ok = decodeIPv4RouteHex("00000000")
	if !ok || got != "0.0.0.0" {
		t.Fatalf("default got %q ok=%v", got, ok)
	}
}
