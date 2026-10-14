package main

import (
	"os"
	"strings"
	"testing"
)

func TestNF5AFrontendSendsPreimageForEveryListSave(t *testing.T) {
	data, err := os.ReadFile("../nfqws-manager/frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Count(s, "api('/list/save'") != 2 {
		t.Fatal("unexpected list save callers: contract must cover each")
	}
	for _, marker := range []string{
		"expected_sha256:listMeta.sha256",
		"expected_sha256:analysisHashes[target]",
		"analysisHashes=loaded.hashes",
		"hashes[item.name]=d.sha256",
	} {
		if !strings.Contains(s, marker) {
			t.Fatalf("missing NF5A preimage safety marker: %s", marker)
		}
	}
}
