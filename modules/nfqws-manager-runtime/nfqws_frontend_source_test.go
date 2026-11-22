package main

import (
	"os"
	"path/filepath"
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

// Read the actual native Svelte owner. A single legacy controller snapshot cannot
// establish CAS/consent contracts after the Svelte 5 ownership split.
func nfqwsNativeOwner(t *testing.T, file string) string {
	t.Helper()
	if filepath.Base(file) != file {
		t.Fatalf("invalid native owner path %q", file)
	}
	data, err := os.ReadFile(filepath.Join("..", "nfqws-manager", "frontend", file))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
