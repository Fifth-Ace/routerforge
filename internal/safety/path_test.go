package safety

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLexicalRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	resolver := Resolver{Roots: []string{root}}

	bad := []string{
		"relative/file",
		filepath.Join(root, "safe") + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "escape",
		root + string(os.PathSeparator) + ".." + string(os.PathSeparator) + filepath.Base(root) + "-other",
		root + string(os.PathSeparator) + "bad\x00name",
	}

	for _, candidate := range bad {
		if _, _, err := resolver.Lexical(candidate, nil); err == nil {
			t.Fatalf("unsafe path accepted: %q", candidate)
		}
	}
}

func TestLexicalRejectsPrefixSibling(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "allowed")
	sibling := filepath.Join(base, "allowed-other")

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}

	resolver := Resolver{Roots: []string{root}}
	if _, _, err := resolver.Lexical(filepath.Join(sibling, "file"), nil); err == nil {
		t.Fatal("prefix sibling must not be treated as inside allowed root")
	}
}

func TestResolveExistingRejectsSymlinkEscape(t *testing.T) {
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

	resolver := Resolver{
		Roots: []string{root},
		EvalSymlinks: func(path string) (string, error) {
			switch filepath.Clean(path) {
			case filepath.Clean(root):
				return root, nil
			case filepath.Clean(target):
				return filepath.Join(outside, "secret"), nil
			default:
				return filepath.EvalSymlinks(path)
			}
		},
	}

	if _, err := resolver.ResolveExisting(target, nil); err == nil {
		t.Fatal("symlink escape must be rejected")
	}
}

func TestResolveCreatableRejectsSymlinkParentEscape(t *testing.T) {
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

	resolver := Resolver{
		Roots: []string{root},
		EvalSymlinks: func(path string) (string, error) {
			switch filepath.Clean(path) {
			case filepath.Clean(root):
				return root, nil
			case filepath.Clean(parent):
				return outside, nil
			default:
				return filepath.EvalSymlinks(path)
			}
		},
	}

	if _, err := resolver.ResolveCreatable(target, nil, nil); err == nil {
		t.Fatal("creatable target through escaping symlink parent must be rejected")
	}
}

func TestResolveCreatableAllowsMissingLeaf(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "etc")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(parent, "new.conf")
	resolver := Resolver{Roots: []string{root}}

	got, err := resolver.ResolveCreatable(target, nil, nil)
	if err != nil {
		t.Fatalf("resolve creatable: %v", err)
	}
	if got.Canonical != target {
		t.Fatalf("canonical=%q want=%q", got.Canonical, target)
	}
}

func TestResolveCreatableRejectsReadOnlyRoot(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "etc")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(parent, "new.conf")
	resolver := Resolver{Roots: []string{root}}

	_, err := resolver.ResolveCreatable(target, nil, func(candidate string) bool {
		return filepath.Clean(candidate) == filepath.Clean(root)
	})
	if err == nil {
		t.Fatal("read-only root must reject creatable target")
	}
}

func TestForbiddenCallbackIsPreserved(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "blocked")
	resolver := Resolver{Roots: []string{root}}

	_, _, err := resolver.Lexical(target, func(candidate string) bool {
		return filepath.Clean(candidate) == filepath.Clean(target)
	})
	if err == nil {
		t.Fatal("forbidden callback must reject path")
	}
}
