package main

import (
	"strings"
	"testing"
)

func TestNF5G2FrontendUsesVerifiedPreview(t *testing.T) {
	source := nfqwsFrontendSource(t)
	s := source
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
