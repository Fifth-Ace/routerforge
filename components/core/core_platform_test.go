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
func TestParseEntwareTargets(t *testing.T) {
	raw := "arch all 1\narch aarch64-3.10 10\narch aarch64-3.10 20\n"
	got := parseEntwareTargets(raw)
	if len(got) != 1 || got[0] != "aarch64-3.10" {
		t.Fatalf("targets=%v, want [aarch64-3.10]", got)
	}
}

func TestTargetResolutionFromArchitectureOutput(t *testing.T) {
	resolved := targetResolutionFromArchitectureOutput("arch all 1\narch mipsel-3.4 10\n")
	if resolved.Status != "resolved" || resolved.Target != "mipsel-3.4" {
		t.Fatalf("resolved=%#v", resolved)
	}
	ambiguous := targetResolutionFromArchitectureOutput("arch mips-3.4 10\narch mipsel-3.4 10\n")
	if ambiguous.Status != "ambiguous" || ambiguous.Target != "" || len(ambiguous.Candidates) != 2 {
		t.Fatalf("ambiguous=%#v", ambiguous)
	}
	unknown := targetResolutionFromArchitectureOutput("arch all 1\n")
	if unknown.Status != "unknown" || unknown.Target != "" {
		t.Fatalf("unknown=%#v", unknown)
	}
}
