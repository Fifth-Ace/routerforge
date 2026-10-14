package main

import (
	"os"
	"strings"
	"testing"
)

func TestNF5BFrontendDeleteRequiresPreimage(t *testing.T) {
	data, err := os.ReadFile("../nfqws-manager/frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Count(s, "api('/list/delete'") != 1 {
		t.Fatal("all delete callsites need CAS coverage")
	}
	if !strings.Contains(s, "name:selectedList,expected_sha256:listMeta.sha256,confirm:'NFQWS_LIST_DELETE'") {
		t.Fatal("list deletion omitted exact preimage SHA")
	}
}
