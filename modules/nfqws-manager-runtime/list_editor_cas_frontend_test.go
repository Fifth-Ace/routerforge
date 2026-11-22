package main

import (
	"strings"
	"testing"
)

func TestNF5AFrontendSendsPreimageForEveryListSave(t *testing.T) {
	ui := nfqwsNativeOwner(t, "NfqwsListEditor.svelte")
	policy := nfqwsNativeOwner(t, "NfqwsListEditorModel.js")
	runtime := nfqwsNativeOwner(t, "NfqwsRuntime.js")
	for _, marker := range []string{"async function save()", "!canWrite||!dirty", "window.confirm(`Сохранить", "const ref={...baseline}", "listMutation('save',ref,sent)", "request(cmd.path,cmd.body)", "!result?.status"} {
		if !strings.Contains(ui, marker) {
			t.Fatalf("native save owner lacks %s", marker)
		}
	}
	for _, marker := range []string{"path:'/list/save'", "expected_sha256:reference.sha256", "confirm:'NFQWS_LIST_SAVE'", "NFQWS_LIST_INVALID_MUTATION_REFERENCE"} {
		if !strings.Contains(policy, marker) {
			t.Fatalf("save policy lacks %s", marker)
		}
	}
	// The remaining explicit legacy list-tools call is covered separately, no unguarded save.
	if strings.Count(runtime, "api('/list/save'") != 1 || !strings.Contains(runtime, "expected_sha256:expected,confirm:'NFQWS_LIST_SAVE'") {
		t.Fatal("remaining runtime list-tool save lost CAS")
	}
}
