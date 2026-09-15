package safety

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Resolver applies one shared path-safety policy to filesystem operations.
type Resolver struct {
	Roots        []string
	EvalSymlinks func(string) (string, error)
}

// ResolvedPath keeps the requested path together with its checked forms.
type ResolvedPath struct {
	Requested string
	Lexical   string
	Root      string
	Canonical string
}

// HasParentTraversal reports whether a path contains an explicit ".." segment.
func HasParentTraversal(raw string) bool {
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if part == ".." {
			return true
		}
	}
	return false
}

// PathWithinRoot reports whether target is root itself or a descendant of root.
func PathWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func (r Resolver) evalSymlinks(path string) (string, error) {
	if r.EvalSymlinks != nil {
		return r.EvalSymlinks(path)
	}
	return filepath.EvalSymlinks(path)
}

// Lexical validates a raw absolute path before any symlink resolution.
func (r Resolver) Lexical(raw string, forbidden func(string) bool) (string, string, error) {
	if raw == "" {
		return "", "", errors.New("file path is empty")
	}
	if strings.IndexByte(raw, 0) >= 0 {
		return "", "", errors.New("file path contains NUL")
	}
	if !filepath.IsAbs(raw) {
		return "", "", errors.New("file path must be absolute")
	}
	if HasParentTraversal(raw) {
		return "", "", errors.New("parent traversal is not allowed")
	}

	clean := filepath.Clean(raw)
	if forbidden != nil && forbidden(clean) {
		return "", "", errors.New("file path is inside a protected system pseudo-filesystem")
	}

	for _, configuredRoot := range r.Roots {
		root := filepath.Clean(configuredRoot)
		if !filepath.IsAbs(root) {
			continue
		}
		if PathWithinRoot(root, clean) {
			return clean, root, nil
		}
	}

	return "", "", fmt.Errorf("file path is outside allowed roots")
}

// CanonicalRoot resolves and normalizes an allowed root.
func (r Resolver) CanonicalRoot(root string) (string, error) {
	canonical, err := r.evalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve allowed root: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return "", fmt.Errorf("absolute allowed root: %w", err)
	}
	return filepath.Clean(canonical), nil
}

// ResolveExisting validates and resolves a path that must already exist.
func (r Resolver) ResolveExisting(raw string, forbidden func(string) bool) (ResolvedPath, error) {
	lexical, root, err := r.Lexical(raw, forbidden)
	if err != nil {
		return ResolvedPath{}, err
	}

	canonicalRoot, err := r.CanonicalRoot(root)
	if err != nil {
		return ResolvedPath{}, err
	}

	canonical, err := r.evalSymlinks(lexical)
	if err != nil {
		return ResolvedPath{}, fmt.Errorf("resolve file path: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return ResolvedPath{}, fmt.Errorf("absolute file path: %w", err)
	}
	canonical = filepath.Clean(canonical)

	if !PathWithinRoot(canonicalRoot, canonical) {
		return ResolvedPath{}, errors.New("resolved file path escapes allowed root")
	}

	return ResolvedPath{
		Requested: raw,
		Lexical:   lexical,
		Root:      root,
		Canonical: canonical,
	}, nil
}

// ResolveCreatable validates a path whose final leaf may not exist yet.
func (r Resolver) ResolveCreatable(
	raw string,
	forbidden func(string) bool,
	readOnlyRoot func(string) bool,
) (ResolvedPath, error) {
	lexical, root, err := r.Lexical(raw, forbidden)
	if err != nil {
		return ResolvedPath{}, err
	}
	if readOnlyRoot != nil && readOnlyRoot(root) {
		return ResolvedPath{}, errors.New("allowed root is read-only")
	}

	if _, err := os.Lstat(lexical); err == nil {
		return r.ResolveExisting(raw, forbidden)
	} else if !errors.Is(err, os.ErrNotExist) {
		return ResolvedPath{}, fmt.Errorf("inspect file path: %w", err)
	}

	canonicalRoot, err := r.CanonicalRoot(root)
	if err != nil {
		return ResolvedPath{}, err
	}

	parent := filepath.Dir(lexical)
	canonicalParent, err := r.evalSymlinks(parent)
	if err != nil {
		return ResolvedPath{}, fmt.Errorf("resolve parent path: %w", err)
	}
	canonicalParent, err = filepath.Abs(canonicalParent)
	if err != nil {
		return ResolvedPath{}, fmt.Errorf("absolute parent path: %w", err)
	}
	canonicalParent = filepath.Clean(canonicalParent)

	if !PathWithinRoot(canonicalRoot, canonicalParent) {
		return ResolvedPath{}, errors.New("resolved parent path escapes allowed root")
	}

	canonical := filepath.Join(canonicalParent, filepath.Base(lexical))
	if !PathWithinRoot(canonicalRoot, canonical) {
		return ResolvedPath{}, errors.New("resolved file path escapes allowed root")
	}

	return ResolvedPath{
		Requested: raw,
		Lexical:   lexical,
		Root:      root,
		Canonical: canonical,
	}, nil
}
