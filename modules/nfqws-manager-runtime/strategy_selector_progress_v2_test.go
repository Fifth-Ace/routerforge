package main

import "testing"

func TestV2SelectorSessionValid(t *testing.T) {
	if !v2SelectorSessionValid("ui-0123456789abcdef") {
		t.Fatal("valid UI selector session rejected")
	}
	for _, value := range []string{"", "short", "../bad", "UPPERCASE-SESSION"} {
		if v2SelectorSessionValid(value) {
			t.Fatalf("invalid selector session accepted: %q", value)
		}
	}
}
