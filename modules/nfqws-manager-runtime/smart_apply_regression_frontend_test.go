package main

import (
	"strings"
	"testing"
)

func TestNF4DFrontendReportsVerifiedRegressionGuards(t *testing.T) {
	source := nfqwsFrontendSource(t)
	body := source
	for _, marker := range []string{"result.regression_guards_verified", "Проверено других сайтов:"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("NF4D regression guard status missing: %s", marker)
		}
	}
}
