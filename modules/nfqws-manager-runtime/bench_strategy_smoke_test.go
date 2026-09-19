package main

import (
	"strings"
	"testing"
)

func TestProfileAllowsBenchServerName(t *testing.T) {
	profile := benchStrategyProfile{HostlistDomains: []string{"googlevideo.com"}}
	for _, good := range []string{"googlevideo.com", "www.googlevideo.com", "rr1---sn-x.googlevideo.com"} {
		if !profileAllowsBenchServerName(profile, good) {
			t.Fatalf("covered host rejected: %q", good)
		}
	}
	for _, bad := range []string{"googlevideo.com.example.org", "example.com"} {
		if profileAllowsBenchServerName(profile, bad) {
			t.Fatalf("uncovered host accepted: %q", bad)
		}
	}
}

func TestParseIPTablesSavePacketCounter(t *testing.T) {
	line := `[17:2048] -A POSTROUTING -p tcp -m comment --comment "routerforge-bench:session-1234:out-queue" -j NFQUEUE --queue-num 30000`
	got, ok := parseIPTablesSavePacketCounter(line)
	if !ok || got != 17 {
		t.Fatalf("counter=%d ok=%v", got, ok)
	}
}

func TestValidateBenchTLSStrategySmokeRequest(t *testing.T) {
	good := benchTLSStrategySmokeRequest{
		ProfileIndex:         0,
		ServerName:           "www.googlevideo.com",
		ExpectedConfigSHA256: strings.Repeat("a", 64),
		Confirm:              benchTLSStrategySmokeConfirm,
	}
	if err := validateBenchTLSStrategySmokeRequest(good); err != nil {
		t.Fatalf("good request rejected: %v", err)
	}
	bad := good
	bad.ServerName = "1.1.1.1"
	if err := validateBenchTLSStrategySmokeRequest(bad); err == nil {
		t.Fatal("IP literal accepted as server_name")
	}
	bad = good
	bad.Confirm = "WRONG"
	if err := validateBenchTLSStrategySmokeRequest(bad); err == nil {
		t.Fatal("wrong confirm accepted")
	}
}

func TestNormalizeBenchServerName(t *testing.T) {
	got, err := normalizeBenchServerName("WWW.GoogleVideo.Com.")
	if err != nil {
		t.Fatal(err)
	}
	if got != "www.googlevideo.com" {
		t.Fatalf("host=%q", got)
	}
}
