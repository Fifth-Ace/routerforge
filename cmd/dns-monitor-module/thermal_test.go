package main

import "testing"

func TestNormalizeTemperature(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
		ok   bool
	}{
		{76000, 76, true},
		{55000, 55, true},
		{42.5, 42.5, true},
		{250000, 0, false},
	}
	for _, tc := range cases {
		got, ok := normalizeTemperature(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Fatalf("normalizeTemperature(%v)=(%v,%v) want (%v,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestThermalCategory(t *testing.T) {
	cases := map[string]string{
		"cpu-thermal": "soc",
		"phy0 temp":   "wifi",
		"nvme0":       "storage",
		"pmic":        "board",
		"mystery":     "other",
	}
	for input, want := range cases {
		if got := thermalCategory(input, ""); got != want {
			t.Fatalf("thermalCategory(%q)=%q want %q", input, got, want)
		}
	}
}

func TestParseSmartctlTemperature(t *testing.T) {
	ata := "194 Temperature_Celsius     0x0022   114   099   000    Old_age   Always       -       36"
	if got, ok := parseSmartctlTemperature(ata); !ok || got != 36 {
		t.Fatalf("ATA parse=(%v,%v)", got, ok)
	}
	nvme := "Temperature:                        41 Celsius"
	if got, ok := parseSmartctlTemperature(nvme); !ok || got != 41 {
		t.Fatalf("NVMe parse=(%v,%v)", got, ok)
	}
}
