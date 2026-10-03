package main

import "testing"

func TestP30BOfficialModuleOrder(t *testing.T) {
	want := []string{
		"routerforge-core",
		"monitoring",
		"dns",
		"admin",
		"network-tools",
		"nfqws-manager",
		"antiscan-manager",
		"profiling",
	}
	for i := 0; i < len(want)-1; i++ {
		if moduleOrder(want[i]) >= moduleOrder(want[i+1]) {
			t.Fatalf("module order broken: %s=%d must be before %s=%d",
				want[i], moduleOrder(want[i]), want[i+1], moduleOrder(want[i+1]))
		}
	}
	if moduleOrder("future-module") >= moduleOrder("profiling") {
		t.Fatalf("future modules must remain before profiling: future=%d profiling=%d",
			moduleOrder("future-module"), moduleOrder("profiling"))
	}
}
