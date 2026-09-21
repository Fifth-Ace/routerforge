package main

import "testing"

func TestV2StrategySafeID(t *testing.T) {
	if !v2StrategySafeID("s-deadbeef") {
		t.Fatal("safe id rejected")
	}
	for _, value := range []string{"", "../x", ".hidden", "x/y", "bad space"} {
		if v2StrategySafeID(value) {
			t.Fatalf("unsafe id accepted: %q", value)
		}
	}
}

func TestV2StrategyFingerprintDeterministic(t *testing.T) {
	a := []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=2"}
	if v2StrategyFingerprint(a) != v2StrategyFingerprint(append([]string{}, a...)) {
		t.Fatal("fingerprint is not deterministic")
	}
	b := append([]string{}, a...)
	b[len(b)-1] = "--lua-desync=multisplit:pos=3"
	if v2StrategyFingerprint(a) == v2StrategyFingerprint(b) {
		t.Fatal("different strategy args have the same test fingerprint")
	}
}

func TestV2StrategySourceWhitelist(t *testing.T) {
	if v2StrategySource("catalog") != "catalog" || v2StrategySource("import") != "import" {
		t.Fatal("known source not preserved")
	}
	if v2StrategySource("whatever") != "custom" {
		t.Fatal("unknown source must collapse to custom")
	}
}
