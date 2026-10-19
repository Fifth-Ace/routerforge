package main

import "testing"

func TestTLSWaiverRiskClassification(t *testing.T) {
	cases := []struct {
		name string
		cfg  configSummary
		want bool
	}{
		{"https-verified", configSummary{URL: "https://lapi.example", TLSVerify: true}, false},
		{"http", configSummary{URL: "http://lapi.example", TLSVerify: true}, true},
		{"https-no-verify", configSummary{URL: "https://lapi.example", TLSVerify: false}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if actual := insecureLAPI(tc.cfg); actual != tc.want {
				t.Fatalf("insecureLAPI()=%v want=%v", actual, tc.want)
			}
		})
	}
}

func TestTLSWaiverBindsURLAndVerification(t *testing.T) {
	base := configSummary{URL: "http://192.0.2.10:8080", TLSVerify: true}
	if waiverFingerprint(base) != waiverFingerprint(base) {
		t.Fatal("fingerprint must be stable")
	}
	if waiverFingerprint(base) == waiverFingerprint(configSummary{URL: "http://192.0.2.11:8080", TLSVerify: true}) {
		t.Fatal("consent must not carry over to a different LAPI URL")
	}
	if waiverFingerprint(base) == waiverFingerprint(configSummary{URL: base.URL, TLSVerify: false}) {
		t.Fatal("consent must not carry over to a different TLS verification mode")
	}
}
