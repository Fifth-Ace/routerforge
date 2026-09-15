package main

import (
	"path/filepath"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

var (
	adminFileAllowedRoots = []string{"/opt", "/tmp"}
	adminFileEvalSymlinks = filepath.EvalSymlinks
)

type adminFileResolvedPath = safety.ResolvedPath

func adminFileSafetyResolver() safety.Resolver {
	return safety.Resolver{
		Roots:        append([]string(nil), adminFileAllowedRoots...),
		EvalSymlinks: adminFileEvalSymlinks,
	}
}

func hasAdminFileParentTraversal(raw string) bool {
	return safety.HasParentTraversal(raw)
}

func adminFilePathWithinRoot(root, target string) bool {
	return safety.PathWithinRoot(root, target)
}

func adminFileLexicalPath(raw string) (string, string, error) {
	return adminFileSafetyResolver().Lexical(raw, adminFileForbiddenSystemPath)
}

func canonicalAdminFileRoot(root string) (string, error) {
	return adminFileSafetyResolver().CanonicalRoot(root)
}

func resolveExistingAdminFilePath(raw string) (adminFileResolvedPath, error) {
	return adminFileSafetyResolver().ResolveExisting(raw, adminFileForbiddenSystemPath)
}

func resolveCreatableAdminFilePath(raw string) (adminFileResolvedPath, error) {
	return adminFileSafetyResolver().ResolveCreatable(
		raw,
		adminFileForbiddenSystemPath,
		adminFileRootReadOnly,
	)
}
