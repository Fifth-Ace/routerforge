package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDefaultIPv4Interface(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "route")
	content := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\n" +
		"br0\t0001A8C0\t00000000\t0001\t0\t0\t0\t00FFFFFF\n" +
		"ppp0\t00000000\t0101A8C0\t0003\t0\t0\t0\t00000000\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readDefaultIPv4Interface(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ppp0" {
		t.Fatalf("default interface = %q, want ppp0", got)
	}
}

func TestParseNDMCVersion(t *testing.T) {
	model, version, err := parseNDMCVersion("model: Keenetic Ultra\nrelease: 5.1.0\nserial: hidden\n")
	if err != nil {
		t.Fatal(err)
	}
	if model != "Keenetic Ultra" || version != "5.1.0" {
		t.Fatalf("model/version = %q / %q", model, version)
	}
}

func TestJSONShape(t *testing.T) {
	shape, items := JSONShape([]any{1, 2, 3})
	if shape != "array" || items != 3 {
		t.Fatalf("array shape = %s / %d", shape, items)
	}
	shape, items = JSONShape(map[string]any{"a": 1, "b": 2})
	if shape != "object" || items != 2 {
		t.Fatalf("object shape = %s / %d", shape, items)
	}
}

func TestConfigAssignment(t *testing.T) {
	config := "# ISP_INTERFACE=old\nISP_INTERFACE=\"ppp0\"\n"
	if got := configAssignment(config, "ISP_INTERFACE"); got != "ppp0" {
		t.Fatalf("ISP_INTERFACE = %q", got)
	}
}
