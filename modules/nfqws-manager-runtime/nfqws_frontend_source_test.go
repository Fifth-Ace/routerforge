package main

import (
	"os"
	"testing"
)

// Exact shipped Svelte template plus guarded business runtime. Never use the
// legacy inline HTML as proof of mutation or preimage safety.
func nfqwsFrontendSource(t *testing.T) string {
	t.Helper()
	markup, err := os.ReadFile("../nfqws-manager/frontend/NFQWSManagerApp.svelte")
	if err != nil {
		t.Fatal(err)
	}
	controller, err := os.ReadFile("../nfqws-manager/frontend/NfqwsRuntime.js")
	if err != nil {
		t.Fatal(err)
	}
	return string(markup) + "\n" + string(controller)
}
