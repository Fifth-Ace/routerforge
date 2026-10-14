package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// NF5E publishes a new upstream list with O_EXCL. Unlike atomic replacement,
// an existing file (including a symlink) can never be overwritten when it
// appears between the install handler's initial check and the final create.
// This is intentionally only the new-install path, not the CAS replace path.
func nf5eInstallNewSource(root, name string, data []byte) error {
	if !safeListSourceName(name) {
		return errors.New("unsafe list source filename")
	}
	if len(data) == 0 || len(data) > listFileMaxBytes {
		return errors.New("new list source payload is empty or too large")
	}
	if err := validateListContent(string(data)); err != nil {
		return err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return errors.New("list source root is not a real directory")
	}
	path := filepath.Join(root, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("exclusive new list source create: %w", err)
	}
	createdInfo, statErr := file.Stat()
	completed := false
	defer func() {
		if completed || statErr != nil {
			return
		}
		// If a write or fsync fails, remove only the inode we created. Never
		// delete an intervening replacement placed there by another process.
		if current, err := os.Lstat(path); err == nil && os.SameFile(createdInfo, current) {
			_ = os.Remove(path)
		}
	}()
	written, writeErr := file.Write(data)
	if writeErr == nil && written != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if statErr != nil {
		return fmt.Errorf("stat new list source: %w", statErr)
	}
	if writeErr != nil {
		return fmt.Errorf("write new list source: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close new list source: %w", closeErr)
	}
	completed = true
	return nil
}
