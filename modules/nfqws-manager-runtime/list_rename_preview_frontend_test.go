package main

import (
	"strings"
	"testing"
)

func TestNF5G1FrontendReadOnlyRenamePreview(t *testing.T) {
	ui := nfqwsNativeOwner(t, "NfqwsListEditor.svelte")
	policy := nfqwsNativeOwner(t, "NfqwsListEditorModel.js")
	for _, marker := range []string{`id="previewRenameList"`, `id="renamePreviewModal"`, "async function renamePreview()", "expected_sha256:ref.sha256", "request('/list/rename-preview?'+query.toString())", "checkedRenamePreview(ref,next,response)", "baseline?.sha256!==ref.sha256", "renamePlan=verified"} {
		if !strings.Contains(ui, marker) {
			t.Fatalf("native read-only rename preview lacks %s", marker)
		}
	}
	for _, marker := range []string{"!p?.ok||!p.preview_only||p.mutation_performed", "p.old_list_sha256!==reference.sha256", "NFQWS_LIST_RENAME_PREVIEW_STALE_OR_INVALID"} {
		if !strings.Contains(policy, marker) {
			t.Fatalf("read-only rename policy lacks %s", marker)
		}
	}
}
