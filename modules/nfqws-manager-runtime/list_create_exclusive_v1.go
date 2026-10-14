package main

import (
	"errors"
	"os"
	"path/filepath"
)

// NF5C: create user lists without a check-then-replace race. O_EXCL refuses
// existing files and final-component symlinks even if created concurrently.
// Do not use atomic replace/write helpers for an exclusively new filename.
func nf5cCreateListExclusiveInRoot(root, name string) error {
	if !safeListSourceName(name) {
		return errors.New("unsafe or invalid new list filename")
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return errors.New("list root must be a real directory")
	}
	file, err := os.OpenFile(filepath.Join(root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

func nf5cCreateListExclusive(name string) error {
	if err := ensureListSourceRoot(); err != nil {
		return err
	}
	return nf5cCreateListExclusiveInRoot(listsRoot, name)
}
