package main

import (
	"os"
	"strings"
	"testing"
)

func TestNF4DFrontendReportsVerifiedRegressionGuards(t *testing.T) {
	data, err := os.ReadFile("../nfqws-manager/frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, marker := range []string{"result.regression_guards_verified", "Проверено других сайтов:"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("NF4D regression guard status missing: %s", marker)
		}
	}
}
