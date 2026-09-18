package main

import "testing"

func TestSafeConfigLibraryName(t *testing.T) {
	valid := []string{"youtube.conf", "gaming-test.conf", "rkn_2.conf", "NFQWS2.CONF"}
	for _, name := range valid {
		if !safeConfigLibraryName(name) {
			t.Fatalf("expected valid config name: %q", name)
		}
	}
	invalid := []string{"", ".hidden.conf", "../bad.conf", "bad", "bad.cfg", "a b.conf", "x/y.conf"}
	for _, name := range invalid {
		if safeConfigLibraryName(name) {
			t.Fatalf("expected invalid config name: %q", name)
		}
	}
}

func TestValidateLibraryConfigAllowsEmptyDraft(t *testing.T) {
	if err := validateLibraryConfig(""); err != nil {
		t.Fatalf("empty library draft should be accepted: %v", err)
	}
	if err := validateConfig(""); err == nil {
		t.Fatal("empty canonical config must remain rejected")
	}
}

func TestValidateLibraryConfigRejectsNUL(t *testing.T) {
	if err := validateLibraryConfig("A=1\x00B=2"); err == nil {
		t.Fatal("NUL-containing stored config must be rejected")
	}
}
