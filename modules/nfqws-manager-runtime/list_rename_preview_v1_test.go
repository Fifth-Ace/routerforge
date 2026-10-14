package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nf5gTestRoots(t *testing.T, config string) (string, string, string, string) {
	t.Helper()
	root := t.TempDir()
	lists := filepath.Join(root, "lists")
	library := filepath.Join(root, "library")
	active := filepath.Join(root, "active.conf")
	for _, p := range []string{lists, library} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(active, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	old := []byte("hello.example\n")
	if err := os.WriteFile(filepath.Join(lists, "one.list"), old, 0600); err != nil {
		t.Fatal(err)
	}
	return lists, active, library, listSourceSHA256(old)
}

func TestNF5G1PlanExactSelectorsAndPreservesOtherBytes(t *testing.T) {
	root := "/opt/etc/nfqws2/lists"
	in := []byte("# heading\r\n--hostlist=" + root + "/one.list --filter-tcp=443\r\n--ipset-exclude=" + root + "/one.list\r\n")
	out, n, err := nf5gPlanConfigRename(in, root, "one.list", "two.list")
	if err != nil || n != 2 || string(out) != strings.ReplaceAll(string(in), "one.list", "two.list") {
		t.Fatalf("count=%d err=%v out=%q", n, err, out)
	}
}

func TestNF5G1PreviewActiveAndSavedNoWrites(t *testing.T) {
	root := t.TempDir()
	lists, active, lib, sha := nf5gTestRoots(t, "NFQWS_ARGS_CUSTOM='--hostlist="+filepath.Join(root, "dummy")+"'\n")
	config := "NFQWS_ARGS_CUSTOM='--hostlist=" + filepath.Join(lists, "one.list") + "'\n"
	if err := os.WriteFile(active, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	saved := "--hostlist-exclude=" + filepath.Join(lists, "one.list") + "\n"
	if err := os.WriteFile(filepath.Join(lib, "saved.conf"), []byte(saved), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := nf5gPreviewRenameInRoots(lists, active, lib, "one.list", "two.list", sha)
	if err != nil || !p.OK || !p.PreviewOnly || p.MutationPerformed || p.ReferenceCount != 2 || len(p.ConfigChanges) != 2 {
		t.Fatalf("preview=%+v err=%v", p, err)
	}
	if _, err := os.Stat(filepath.Join(lists, "two.list")); !os.IsNotExist(err) {
		t.Fatal("preview created a destination")
	}
	got, _ := os.ReadFile(active)
	if string(got) != config {
		t.Fatal("preview mutated active config")
	}
	got, _ = os.ReadFile(filepath.Join(lib, "saved.conf"))
	if string(got) != saved {
		t.Fatal("preview mutated saved config")
	}
}

func TestNF5G1PreviewUnreferencedList(t *testing.T) {
	lists, active, lib, sha := nf5gTestRoots(t, "NFQWS_ARGS_CUSTOM=''\n")
	p, err := nf5gPreviewRenameInRoots(lists, active, lib, "one.list", "two.list", sha)
	if err != nil || !p.OK || p.ReferenceCount != 0 || len(p.ConfigChanges) != 0 {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestNF5G1RejectsStaleSHAAndTargetCollision(t *testing.T) {
	lists, active, lib, sha := nf5gTestRoots(t, "NFQWS_ARGS_CUSTOM=''\n")
	if _, err := nf5gPreviewRenameInRoots(lists, active, lib, "one.list", "two.list", strings.Repeat("0", 64)); err == nil {
		t.Fatal("stale preimage accepted")
	}
	if err := os.WriteFile(filepath.Join(lists, "two.list"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := nf5gPreviewRenameInRoots(lists, active, lib, "one.list", "two.list", sha); err == nil {
		t.Fatal("existing destination accepted")
	}
}

func TestNF5G1RejectsProtectedAndUnsafeNames(t *testing.T) {
	lists, active, lib, sha := nf5gTestRoots(t, "")
	for _, names := range [][2]string{{"one.list", "user.list"}, {"one.list", "../x.list"}, {"one.list", "one.list"}, {"user.list", "other.list"}, {"one.list", "bad.list-old"}} {
		if _, err := nf5gPreviewRenameInRoots(lists, active, lib, names[0], names[1], sha); err == nil {
			t.Fatalf("unsafe rename accepted: %v", names)
		}
	}
}

func TestNF5G1RejectsDynamicOrSubstringConfigMentions(t *testing.T) {
	root := "/opt/etc/nfqws2/lists"
	for _, conf := range []string{"--hostlist=/other/one.list\n", "--hostlist=" + root + "/my-one.list\n", "# one.list\n", "# --hostlist=" + root + "/one.list\n", "--hostlist=$ROOT/one.list\n"} {
		if _, _, err := nf5gPlanConfigRename([]byte(conf), root, "one.list", "two.list"); err == nil {
			t.Errorf("ambiguous config allowed: %s", conf)
		}
	}
}

func TestNF5G1RejectsDestinationAlreadyReferenced(t *testing.T) {
	root := "/opt/etc/nfqws2/lists"
	_, _, err := nf5gPlanConfigRename([]byte("--hostlist="+root+"/two.list"), root, "one.list", "two.list")
	if err == nil {
		t.Fatal("new name is already referenced")
	}
}

func TestNF5G1RejectsSymlinks(t *testing.T) {
	lists, active, lib, sha := nf5gTestRoots(t, "")
	if err := os.Symlink(filepath.Join(lists, "one.list"), filepath.Join(lists, "two.list")); err != nil {
		t.Skip(err)
	}
	if _, err := nf5gPreviewRenameInRoots(lists, active, lib, "one.list", "two.list", sha); err == nil {
		t.Fatal("target symlink accepted")
	}
}

func TestNF5G1RejectsUnreadableConfigLibrary(t *testing.T) {
	lists, active, lib, sha := nf5gTestRoots(t, "")
	if err := os.Mkdir(filepath.Join(lib, "unsafe.conf"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := nf5gPreviewRenameInRoots(lists, active, lib, "one.list", "two.list", sha); err == nil {
		t.Fatal("unsafe saved config accepted")
	}
}

func TestNF5G1NoMutationRoutes(t *testing.T) {
	b, err := os.ReadFile("list_rename_preview_v1.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `getOnly(handleNF5GRenamePreview)`) || !strings.Contains(s, `"preview_only"`) {
		t.Fatal("read-only handler contract missing")
	}
	if strings.Contains(s, "safety.WriteFileAtomic") || strings.Contains(s, "os.Rename(") || strings.Contains(s, "os.Remove(") {
		t.Fatal("preview must not mutate files")
	}
}
