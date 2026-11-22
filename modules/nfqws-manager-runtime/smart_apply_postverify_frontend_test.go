package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestNF4CFrontendOnlyShowsSuccessAfterLiveTargetVerification(t *testing.T) {
	selector := nfqwsNativeOwner(t, "NfqwsSelector.svelte")
	auto := nfqwsNativeOwner(t, "NfqwsAutoSetup.svelte")
	for _, marker := range []string{"globalThis.confirm(`Применить найденную стратегию", "selectorApplyBody(result,p,latest)", "request('/bench-autotune/apply','POST'", "response?.rolled_back||!response?.post_apply_verified", "NFQWS_SELECTOR_APPLY_NOT_VERIFIED", "status.config_sha256!==response.config_sha256", "onApplied(status)"} {
		if !strings.Contains(selector, marker) {
			t.Fatalf("selector postverify missing %s", marker)
		}
	}
	if !strings.Contains(auto, "result.rolled_back||!result.post_apply_verified") || !strings.Contains(auto, "APPLY_ROLLBACK") {
		t.Fatal("auto-setup reported success without live postverify")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found; native guards validated")
	}
	cmd := exec.Command(node, "--check", "../nfqws-manager/frontend/NfqwsListEditorModel.js")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("list model invalid: %s %v", out, err)
	}
}
