package main

import (
	"strings"
	"testing"
)

func TestNF5BFrontendDeleteRequiresPreimage(t *testing.T) {
	ui := nfqwsNativeOwner(t, "NfqwsListEditor.svelte")
	policy := nfqwsNativeOwner(t, "NfqwsListEditorModel.js")
	for _, marker := range []string{"async function remove()", "!canManage||dirty", "window.confirm(`Удалить", "listMutation('delete',reference)", "request(cmd.path,cmd.body)", "!result?.status"} {
		if !strings.Contains(ui, marker) {
			t.Fatalf("native delete owner lacks %s", marker)
		}
	}
	for _, marker := range []string{"if(reference.protected)", "expected_sha256:reference.sha256", "confirm:'NFQWS_LIST_DELETE'", "path:'/list/delete'", "NFQWS_LIST_INVALID_MUTATION_REFERENCE"} {
		if !strings.Contains(policy, marker) {
			t.Fatalf("delete policy lacks %s", marker)
		}
	}
	if strings.Count(ui, "listMutation('delete'") != 1 {
		t.Fatal("native delete must have one guarded command")
	}
}
