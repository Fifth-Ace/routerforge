package main

import "testing"

func TestShortDeviceModel(t *testing.T) {
	cases := map[string]string{
		"Keenetic Ultra (KN-1812)": "KN-1812",
		"Keenetic Hopper KN-3811":  "KN-3811",
		"Netcraze Example Router":  "Netcraze Example Router",
		"":                         "Keenetic",
	}
	for input, want := range cases {
		if got := shortDeviceModel(input); got != want {
			t.Fatalf("shortDeviceModel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParsePlatformUptimeSeconds(t *testing.T) {
	if got := parsePlatformUptimeSeconds("12345.67 98765.43"); got != 12345 {
		t.Fatalf("uptime=%d, want 12345", got)
	}
	for _, value := range []string{"", "bad", "-1 0"} {
		if got := parsePlatformUptimeSeconds(value); got != 0 {
			t.Fatalf("parsePlatformUptimeSeconds(%q)=%d, want 0", value, got)
		}
	}
}

func TestParseNDMCDeviceModel(t *testing.T) {
	cases := map[string]string{
		"model: Keenetic Ultra (KN-1812)\n":            "Keenetic Ultra (KN-1812)",
		"device: KN-3811\n":                            "KN-3811",
		"some-prefix Keenetic Hopper KN-3811 suffix\n": "some-prefix Keenetic Hopper KN-3811 suffix",
		"version: 5.01.C.1.0-0\n":                      "",
	}
	for input, want := range cases {
		if got := parseNDMCDeviceModel(input); got != want {
			t.Fatalf("parseNDMCDeviceModel(%q) = %q, want %q", input, got, want)
		}
	}
}
