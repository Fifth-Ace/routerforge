package main

import (
	"strings"
	"testing"
)

func TestNF4DFrontendReportsVerifiedRegressionGuards(t *testing.T) {
	selector := nfqwsNativeOwner(t, "NfqwsSelector.svelte")
	for _, marker := range []string{"response?.rolled_back||!response?.post_apply_verified", "response.regression_guards_verified||0", "production проверен; других сайтов:", "onApplied(status)"} {
		if !strings.Contains(selector, marker) {
			t.Fatalf("verified regression reporting missing %s", marker)
		}
	}
}
