package main

import (
	"os"
	"strings"
	"testing"
)

func TestNF5DFrontendSendsOldAndNewListSHA(t *testing.T) {
	data, err := os.ReadFile("../nfqws-manager/frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Count(s, "api('/list-sources/install'") != 1 {
		t.Fatal("upstream list source installer callsites changed; review all callers")
	}
	for _, marker := range []string{
		"expected_sha256:item.sha256",
		"expected_existing_sha256:replace?local.sha256:''",
		"confirm:replace?'NFQWS_LIST_SOURCE_REPLACE':'NFQWS_LIST_SOURCE_INSTALL'",
	} {
		if !strings.Contains(s, marker) {
			t.Fatalf("missing source-list CAS marker %q", marker)
		}
	}
}
