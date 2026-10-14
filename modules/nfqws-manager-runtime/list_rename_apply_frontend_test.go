package main

import (
	"os"
	"strings"
	"testing"
)

func TestNF5G2FrontendUsesVerifiedPreview(t *testing.T) {
	data, err := os.ReadFile("../nfqws-manager/frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, marker := range []string{
		`id="applyRenameList"`,
		`expected_config_changes:p.config_changes`,
		`confirm:'ROUTERFORGE_LIST_RENAME_APPLY'`,
		`'/list/rename-apply'`,
		`nf5g2PreviewPlan=null`,
		`if(!result.ok||result.rolled_back)`,
	} {
		if !strings.Contains(s, marker) {
			t.Fatalf("missing rename apply marker: %s", marker)
		}
	}
}
