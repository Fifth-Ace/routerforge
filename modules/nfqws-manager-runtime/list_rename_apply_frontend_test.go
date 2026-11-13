package main

import (
	"strings"
	"testing"
)

func TestNF5G2FrontendUsesVerifiedPreview(t *testing.T) {
	ui := nfqwsNativeOwner(t, "NfqwsListEditor.svelte")
	policy := nfqwsNativeOwner(t, "NfqwsListEditorModel.js")
	for _, marker := range []string{`id="applyRenameList"`, "if(!active||!canManage||dirty||!renamePlan)", "listRenameApply(plan,ref)", "window.confirm(`Переименовать", "renamePlan=null", "request(cmd.path,cmd.body)", "!result?.ok||result.rolled_back", "selectAfterMutation(plan.new_name)"} {
		if !strings.Contains(ui, marker) {
			t.Fatalf("native rename apply lacks %s", marker)
		}
	}
	for _, marker := range []string{"reference.sha256!==plan.old_list_sha256", "expected_config_changes:plan.config_changes", "confirm:'ROUTERFORGE_LIST_RENAME_APPLY'", "path:'/list/rename-apply'"} {
		if !strings.Contains(policy, marker) {
			t.Fatalf("rename apply policy lacks %s", marker)
		}
	}
}
