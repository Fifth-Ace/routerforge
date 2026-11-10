package main

import (
	"strings"
	"testing"
)

func TestNF5G1FrontendReadOnlyRenamePreview(t *testing.T) {
	source := nfqwsFrontendSource(t)
	s := source
	for _, marker := range []string{
		`id="previewRenameList"`,
		`id="renamePreviewModal"`,
		`async function previewListRename()`,
		`'/list/rename-preview?'`,
		`expected_sha256:listMeta.sha256`,
		`p.preview_only||p.mutation_performed`,
	} {
		if !strings.Contains(s, marker) {
			t.Fatalf("NF5G1 frontend marker missing: %s", marker)
		}
	}
}
