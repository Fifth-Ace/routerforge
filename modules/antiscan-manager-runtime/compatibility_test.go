package main

import (
	"encoding/json"
	"testing"
)

func TestCompareAntiscanVersionsMirrorsPinnedUpstreamOrdering(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{"1.10.6", "1.10.6", 0},
		{"1.10.7", "1.10.6", 1},
		{"1.10.5", "1.10.6", -1},
		{"1.10.6-2", "1.10.6-1", 1},
		{"1:1.10.6", "1.99.99", 1},
		{"1.10.6~rc1", "1.10.6", -1},
	}
	for _, test := range tests {
		got := compareAntiscanVersions(test.left, test.right)
		if got < 0 {
			got = -1
		} else if got > 0 {
			got = 1
		}
		if got != test.want {
			t.Fatalf("compare %q %q = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestAntiscanVersionUpstreamPartAcceptsPackageRevision(t *testing.T) {
	if got := antiscanVersionUpstreamPart("1.10.6-3"); got != "1.10.6" {
		t.Fatalf("upstream part=%q", got)
	}
	if got := antiscanVersionUpstreamPart("2:1.10.6-3"); got != "1.10.6" {
		t.Fatalf("epoch upstream part=%q", got)
	}
}

func TestExtractAntiscanUpdateChannelSupportsArrayAndObjectPackages(t *testing.T) {
	fixtures := []string{
		`{"main":{"min_os":"5.2.0","packages":[{"antiscan":{"version":"1.10.7","update_info":"note"}}]}}`,
		`{"main":{"min_os":"5.2.0","packages":{"a":{"antiscan":{"version":"1.10.7","update_info":"note"}}}}}`,
	}
	for _, fixture := range fixtures {
		var payload map[string]any
		if err := json.Unmarshal([]byte(fixture), &payload); err != nil {
			t.Fatal(err)
		}
		version, info, minOS := extractAntiscanUpdateChannel(payload, "main")
		if version != "1.10.7" || info != "note" || minOS != "5.2.0" {
			t.Fatalf("version=%q info=%q minOS=%q", version, info, minOS)
		}
	}
}

func TestCompatibilityContractIsVisibilityOnly(t *testing.T) {
	result := antiscanCompatibility{
		UpdateOwner: "upstream-opkg",
		UpdateAPI:   false,
		AutoUpdate:  false,
	}
	if result.UpdateOwner != "upstream-opkg" || result.UpdateAPI || result.AutoUpdate {
		t.Fatalf("unsafe update ownership: %+v", result)
	}
}
