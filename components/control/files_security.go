package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	adminFileAllowedRoots = []string{"/opt", "/tmp"}
	adminFileEvalSymlinks = filepath.EvalSymlinks
)

type adminFileResolvedPath struct {
	Requested string
	Lexical   string
	Root      string
	Canonical string
}

func hasAdminFileParentTraversal(raw string) bool {
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if part == ".." {
			return true
		}
	}
	return false
}

func adminFilePathWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func adminFileLexicalPath(raw string) (string, string, error) {
	if raw == "" {
		return "", "", errors.New("file path is empty")
	}
	if strings.IndexByte(raw, 0) >= 0 {
		return "", "", errors.New("file path contains NUL")
	}
	if !filepath.IsAbs(raw) {
		return "", "", errors.New("file path must be absolute")
	}
	if hasAdminFileParentTraversal(raw) {
		return "", "", errors.New("parent traversal is not allowed")
	}

	clean := filepath.Clean(raw)
	for _, configuredRoot := range adminFileAllowedRoots {
		root := filepath.Clean(configuredRoot)
		if !filepath.IsAbs(root) {
			continue
		}
		if adminFilePathWithinRoot(root, clean) {
			return clean, root, nil
		}
	}
	return "", "", fmt.Errorf("file path is outside allowed roots")
}

func canonicalAdminFileRoot(root string) (string, error) {
	canonical, err := adminFileEvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve allowed root: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return "", fmt.Errorf("absolute allowed root: %w", err)
	}
	return filepath.Clean(canonical), nil
}

func resolveExistingAdminFilePath(raw string) (adminFileResolvedPath, error) {
	lexical, root, err := adminFileLexicalPath(raw)
	if err != nil {
		return adminFileResolvedPath{}, err
	}

	canonicalRoot, err := canonicalAdminFileRoot(root)
	if err != nil {
		return adminFileResolvedPath{}, err
	}
	canonical, err := adminFileEvalSymlinks(lexical)
	if err != nil {
		return adminFileResolvedPath{}, fmt.Errorf("resolve file path: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return adminFileResolvedPath{}, fmt.Errorf("absolute file path: %w", err)
	}
	canonical = filepath.Clean(canonical)
	if !adminFilePathWithinRoot(canonicalRoot, canonical) {
		return adminFileResolvedPath{}, errors.New("resolved file path escapes allowed root")
	}

	return adminFileResolvedPath{
		Requested: raw,
		Lexical:   lexical,
		Root:      root,
		Canonical: canonical,
	}, nil
}

func resolveCreatableAdminFilePath(raw string) (adminFileResolvedPath, error) {
	lexical, root, err := adminFileLexicalPath(raw)
	if err != nil {
		return adminFileResolvedPath{}, err
	}

	if _, err := os.Lstat(lexical); err == nil {
		return resolveExistingAdminFilePath(raw)
	} else if !errors.Is(err, os.ErrNotExist) {
		return adminFileResolvedPath{}, fmt.Errorf("inspect file path: %w", err)
	}

	canonicalRoot, err := canonicalAdminFileRoot(root)
	if err != nil {
		return adminFileResolvedPath{}, err
	}
	parent := filepath.Dir(lexical)
	canonicalParent, err := adminFileEvalSymlinks(parent)
	if err != nil {
		return adminFileResolvedPath{}, fmt.Errorf("resolve parent path: %w", err)
	}
	canonicalParent, err = filepath.Abs(canonicalParent)
	if err != nil {
		return adminFileResolvedPath{}, fmt.Errorf("absolute parent path: %w", err)
	}
	canonicalParent = filepath.Clean(canonicalParent)
	if !adminFilePathWithinRoot(canonicalRoot, canonicalParent) {
		return adminFileResolvedPath{}, errors.New("resolved parent path escapes allowed root")
	}

	canonical := filepath.Join(canonicalParent, filepath.Base(lexical))
	if !adminFilePathWithinRoot(canonicalRoot, canonical) {
		return adminFileResolvedPath{}, errors.New("resolved file path escapes allowed root")
	}
	return adminFileResolvedPath{
		Requested: raw,
		Lexical:   lexical,
		Root:      root,
		Canonical: canonical,
	}, nil
}
