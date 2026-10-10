package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEngineConfigRoundTripAndStaleReject(t *testing.T) {
	p := filepath.Join(t.TempDir(), "knockd.conf")
	if err := os.WriteFile(p, []byte("[options]\ninterface=eth0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before, _, err := readRegularConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	newSHA, err := saveExistingEngineConfig(p, "knockd", configSHA(before), []byte("[options]\ninterface=eth1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = saveExistingEngineConfig(p, "knockd", configSHA(before), []byte("[options]\ninterface=eth2\n")); err == nil {
		t.Fatal("expected conflict")
	}
	if _, err = restoreExistingEngineConfig(p, "knockd", newSHA); err != nil {
		t.Fatal(err)
	}
	now, _, err := readRegularConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(now) != string(before) {
		t.Fatalf("restore mismatch: %q", now)
	}
}
func TestEngineConfigRejectBad(t *testing.T) {
	for _, x := range [][]byte{nil, []byte("bad"), []byte("a\n\x00")} {
		if validEngineConfig("fwknopd", x) == nil {
			t.Fatal("accepted invalid data")
		}
	}
	if validEngineConfig("wrong", []byte("a\nb")) == nil {
		t.Fatal("unknown engine")
	}
}
