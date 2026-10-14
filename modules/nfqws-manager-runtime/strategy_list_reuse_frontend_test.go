package main

import (
	"os"
	"strings"
	"testing"
)

func TestNF4BFrontendShowsListPlacement(t *testing.T) {
	b, err := os.ReadFile("../nfqws-manager/frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, marker := range []string{"placement_kind==='existing_list'", "placement_list_sha_before", "placement_list_sha_after", "placement_diff"} {
		if !strings.Contains(s, marker) {
			t.Fatalf("NF4B preview marker missing: %s", marker)
		}
	}
}
