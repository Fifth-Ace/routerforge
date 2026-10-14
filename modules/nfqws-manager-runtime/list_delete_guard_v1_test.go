package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNF5BNoReferencesCanDelete(t *testing.T) {
	data := []byte("hello.example\n")
	if err := nf5bAllowDelete("custom.list", data, listSourceSHA256(data), nil); err != nil {
		t.Fatal(err)
	}
}

func TestNF5BRefusesActiveReference(t *testing.T) {
	data := []byte("hello.example\n")
	err := nf5bAllowDelete("custom.list", data, listSourceSHA256(data), []string{"active:nfqws2.conf"})
	if err == nil || !strings.Contains(err.Error(), "active:") {
		t.Fatalf("reference ignored: %v", err)
	}
}

func TestNF5BRefusesSavedReference(t *testing.T) {
	data := []byte("hello.example\n")
	err := nf5bAllowDelete("custom.list", data, listSourceSHA256(data), []string{"library:stored.conf"})
	if err == nil {
		t.Fatal("saved profile reference ignored")
	}
}

func TestNF5BRefusesStaleHash(t *testing.T) {
	data := []byte("new.example\n")
	if err := nf5bAllowDelete("custom.list", data, listSourceSHA256([]byte("old.example\n")), nil); err == nil {
		t.Fatal("stale list identity accepted")
	}
}

func TestNF5BRefusesAbsentHash(t *testing.T) {
	if err := nf5bAllowDelete("custom.list", []byte("test"), "", nil); err == nil {
		t.Fatal("unidentified delete accepted")
	}
}

func TestNF5BRefusesProtectedAndUnsafe(t *testing.T) {
	data := []byte("test")
	for _, name := range []string{"user.list", "exclude.list", "../escape.list", "file.list-opkg"} {
		if err := nf5bAllowDelete(name, data, listSourceSHA256(data), nil); err == nil {
			t.Errorf("allowed protected/unsafe %s", name)
		}
	}
}

func TestNF5BReferenceDetection(t *testing.T) {
	if !nf5bReferencesInContent([]byte(`NFQWS_ARGS_CUSTOM="--filter-tcp=443 --hostlist=/opt/etc/nfqws2/lists/custom.list"`), "custom.list") {
		t.Fatal("active hostlist dependency not detected")
	}
	if !nf5bReferencesInContent([]byte(`NFQWS_ARGS_CUSTOM="--ipset=/opt/etc/nfqws2/lists/custom.list"`), "custom.list") {
		t.Fatal("ipset dependency not detected")
	}
	if nf5bReferencesInContent([]byte(`NFQWS_ARGS_CUSTOM="--hostlist=/opt/etc/nfqws2/lists/other.list"`), "custom.list") {
		t.Fatal("unrelated config mistakenly referenced")
	}
}

func TestNF5BReferenceConfigRegularFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "saved.conf")
	if err := os.WriteFile(file, []byte("some config"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := nf5bReadReferenceConfig(file)
	if err != nil || string(data) != "some config" {
		t.Fatalf("read %s %v", data, err)
	}
}

func TestNF5BReferenceConfigSymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.WriteFile(target, []byte("some config"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := nf5bReadReferenceConfig(link); err == nil {
		t.Fatal("symlink followed")
	}
}
