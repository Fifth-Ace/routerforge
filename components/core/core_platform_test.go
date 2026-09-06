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
