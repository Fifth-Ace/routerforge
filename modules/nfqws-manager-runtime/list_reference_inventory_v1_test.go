package main

import (
	"os"
	"strings"
	"testing"
)

func TestNF5FExactHostlistReference(t *testing.T) {
	data := []byte(`NFQWS_ARGS_CUSTOM="--filter-tcp=443 --hostlist=/opt/etc/nfqws2/lists/video.list"`)
	if !nf5fExactListReference(data, "video.list") {
		t.Fatal("real hostlist reference was missed")
	}
}

func TestNF5FNoSubstringFalseReference(t *testing.T) {
	data := []byte(`NFQWS_ARGS_CUSTOM="--hostlist=/opt/etc/nfqws2/lists/my-video.list"`)
	if nf5fExactListReference(data, "video.list") {
		t.Fatal("substring falsely marked a different list as referenced")
	}
}

func TestNF5FExclusionAndIPSetReferences(t *testing.T) {
	for _, selector := range []string{"--hostlist-exclude=", "--ipset=", "--ipset-exclude=", "--hostlist-auto="} {
		data := []byte(`NFQWS_ARGS_CUSTOM="` + selector + `/opt/etc/nfqws2/lists/exact.list"`)
		if !nf5fExactListReference(data, "exact.list") {
			t.Fatalf("valid selector not counted: %s", selector)
		}
	}
}

func TestNF5FUnrelatedTextDoesNotCreateReference(t *testing.T) {
	data := []byte("# lists/video.list\nPROFILE_NAME=video.list\n--hostlist=/opt/etc/nfqws2/lists/other.list\n")
	if nf5fExactListReference(data, "video.list") {
		t.Fatal("ordinary text or another list created a false reference")
	}
}

func TestNF5FRejectsUnsafeName(t *testing.T) {
	if nf5fExactListReference([]byte("--hostlist=../video.list"), "../video.list") {
		t.Fatal("unsafe list name accepted")
	}
}

func TestNF5FCaseSensitiveFilename(t *testing.T) {
	data := []byte("--hostlist=/opt/etc/nfqws2/lists/Video.list")
	if nf5fExactListReference(data, "video.list") {
		t.Fatal("Linux basename identity is case sensitive")
	}
}

func TestNF5FInventoryUsesExactReferenceCheck(t *testing.T) {
	data, err := os.ReadFile("list_sources.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "if nf5fExactListReference(data, name) {") {
		t.Fatal("inventory does not use exact reference helper")
	}
	if strings.Contains(s, "if strings.Contains(string(data), name) {") {
		t.Fatal("substring reference scan still present")
	}
}
