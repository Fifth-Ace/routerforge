package main

import "testing"

func TestSafeNeighborConfigName(t *testing.T) {
	valid := []string{"old.conf", "youtube-test.conf", "backup_2.cfg", "NFQWS2-OLD.CONF"}
	for _, name := range valid {
		if !safeNeighborConfigName(name) {
			t.Fatalf("expected valid neighbor config name: %q", name)
		}
	}
	invalid := []string{"", ".hidden.conf", "../bad.conf", "bad", "bad.txt", "a b.conf", "x/y.conf"}
	for _, name := range invalid {
		if safeNeighborConfigName(name) {
			t.Fatalf("expected invalid neighbor config name: %q", name)
		}
	}
}

func TestNeighborConfigPathRejectsCanonical(t *testing.T) {
	if _, err := neighborConfigPath("nfqws2.conf"); err == nil {
		t.Fatal("canonical nfqws2.conf must not be importable as a neighbor")
	}
	if _, err := neighborConfigPath("youtube.conf"); err != nil {
		t.Fatalf("regular neighbor config should be accepted: %v", err)
	}
}
