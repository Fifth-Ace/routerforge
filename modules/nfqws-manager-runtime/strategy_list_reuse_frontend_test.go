package main

import (
	"strings"
	"testing"
)

func TestNF4BFrontendShowsListPlacement(t *testing.T) {
	source := nfqwsFrontendSource(t)
	s := source
	for _, marker := range []string{"placement_kind==='existing_list'", "placement_list_sha_before", "placement_list_sha_after", "placement_diff"} {
		if !strings.Contains(s, marker) {
			t.Fatalf("NF4B preview marker missing: %s", marker)
		}
	}
}
