package main

import (
	"os"
	"path/filepath"
	"testing"
)

func withAdminFileRoots(t *testing.T, roots ...string) {
	t.Helper()
	oldRoots := adminFileAllowedRoots
	oldEval := adminFileEvalSymlinks
	adminFileAllowedRoots = append([]string(nil), roots...)
	adminFileEvalSymlinks = filepath.EvalSymlinks
	t.Cleanup(func() {
		adminFileAllowedRoots = oldRoots
		adminFileEvalSymlinks = oldEval
	})
}

func TestResolveExistingAdminFilePathAllowsNestedPath(t *testing.T) {
	root := t.TempDir()
	withAdminFileRoots(t, root)
	nested := filepath.Join(root, "etc", "routerforge")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(nested, "config.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := resolveExistingAdminFilePath(file)
	if err != nil {
		t.Fatalf("resolve existing: %v", err)
	}
	if got.Canonical != file {
		t.Fatalf("canonical=%q want=%q", got.Canonical, file)
	}
}

func TestAdminFilePathRejectsRelativeTraversalAndNUL(t *testing.T) {
	root := t.TempDir()
	withAdminFileRoots(t, root)

	bad := []string{
		"relative/file",
		filepath.Join(root, "safe") + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "escape",
		root + string(os.PathSeparator) + ".." + string(os.PathSeparator) + filepath.Base(root) + "-other",
		root + string(os.PathSeparator) + "bad\x00name",
	}
	for _, candidate := range bad {
		if _, _, err := adminFileLexicalPath(candidate); err == nil {
			t.Fatalf("unsafe path accepted: %q", candidate)
		}
	}
}

func TestAdminFilePathRejectsPrefixSibling(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "allowed")
	sibling := filepath.Join(base, "allowed-other")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	withAdminFileRoots(t, root)

	if _, _, err := adminFileLexicalPath(filepath.Join(sibling, "file")); err == nil {
		t.Fatal("prefix sibling must not be treated as inside allowed root")
	}
}

func TestResolveExistingAdminFilePathRejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "allowed")
	outside := filepath.Join(base, "outside")
	target := filepath.Join(root, "escape", "secret")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	withAdminFileRoots(t, root)

	adminFileEvalSymlinks = func(path string) (string, error) {
		switch filepath.Clean(path) {
		case filepath.Clean(root):
			return root, nil
		case filepath.Clean(target):
			return filepath.Join(outside, "secret"), nil
		default:
			return filepath.EvalSymlinks(path)
		}
	}

	if _, err := resolveExistingAdminFilePath(target); err == nil {
		t.Fatal("symlink escape must be rejected")
	}
}

func TestResolveCreatableAdminFilePathRejectsSymlinkParentEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "allowed")
	outside := filepath.Join(base, "outside")
	parent := filepath.Join(root, "escape")
	target := filepath.Join(parent, "new.conf")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	withAdminFileRoots(t, root)

	adminFileEvalSymlinks = func(path string) (string, error) {
		switch filepath.Clean(path) {
		case filepath.Clean(root):
			return root, nil
		case filepath.Clean(parent):
			return outside, nil
		default:
			return filepath.EvalSymlinks(path)
		}
	}

	if _, err := resolveCreatableAdminFilePath(target); err == nil {
		t.Fatal("creatable target through escaping symlink parent must be rejected")
	}
}

func TestResolveCreatableAdminFilePathAllowsMissingLeaf(t *testing.T) {
	root := t.TempDir()
	withAdminFileRoots(t, root)
	parent := filepath.Join(root, "etc")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "new.conf")

	got, err := resolveCreatableAdminFilePath(target)
	if err != nil {
		t.Fatalf("resolve creatable: %v", err)
	}
	if got.Canonical != target {
		t.Fatalf("canonical=%q want=%q", got.Canonical, target)
	}
}

func TestResolveExistingAdminFilePathAllowsCanonicalPathInsideRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "allowed")
	realDir := filepath.Join(root, "real")
	requested := filepath.Join(root, "alias", "config")
	realFile := filepath.Join(realDir, "config")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(realFile, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	withAdminFileRoots(t, root)

	adminFileEvalSymlinks = func(path string) (string, error) {
		switch filepath.Clean(path) {
		case filepath.Clean(root):
			return root, nil
		case filepath.Clean(requested):
			return realFile, nil
		default:
			return filepath.EvalSymlinks(path)
		}
	}

	got, err := resolveExistingAdminFilePath(requested)
	if err != nil {
		t.Fatalf("internal canonical path rejected: %v", err)
	}
	if got.Canonical != realFile {
		t.Fatalf("canonical=%q want=%q", got.Canonical, realFile)
	}
}
