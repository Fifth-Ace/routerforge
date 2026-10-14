package main

import (
	"os"
	"strings"
	"testing"
)

func TestNF5G1FrontendReadOnlyRenamePreview(t *testing.T) {
	data, err := os.ReadFile("../nfqws-manager/frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
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
