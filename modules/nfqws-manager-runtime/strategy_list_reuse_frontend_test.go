package main

import (
	"strings"
	"testing"
)

func TestNF4BFrontendShowsListPlacement(t *testing.T) {
	selector := nfqwsNativeOwner(t, "NfqwsSelector.svelte")
	for _, marker := range []string{"p.placement_kind==='existing_list'", "p.placement_list_sha_before", "p.placement_list_sha_after", "p.placement_diff", "Конфигурация nfqws2 остаётся прежней."} {
		if !strings.Contains(selector, marker) {
			t.Fatalf("strategy placement source missing %s", marker)
		}
	}
	if !strings.Contains(selector, "preview_only") || !strings.Contains(selector, "globalThis.confirm(`Применить найденную стратегию") {
		t.Fatal("placement can bypass preview/consent")
	}
}
