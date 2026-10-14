package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// NF5G5: before the source is unlinked, revalidate the post-rename bytes
// and make sure no active or saved config has gained a reference to the old
// basename. New configs may appear after Preview or during service restart.
// The guard deliberately treats uncertain or unreadable state as a conflict.
// It prevents the normal stale-reference window but cannot make multi-file
// rename crash-atomic or lock unrelated external processes.
func nf5g5AllowOldSourceRemoval(activePath, libRoot, oldName string, snaps []nf5g2ConfigSnap) error {
	if !safeListSourceName(oldName) {
		return errors.New("invalid old source list name")
	}
	for _, snap := range snaps {
		current, err := nf5bReadReferenceConfig(snap.path)
		if err != nil {
			return fmt.Errorf("verify renamed config %s: %w", snap.path, err)
		}
		if listSourceSHA256(current) != listSourceSHA256(snap.updated) {
			return fmt.Errorf("renamed config changed after write: %s", snap.path)
		}
	}
	active, err := nf5bReadReferenceConfig(activePath)
	if err != nil {
		return fmt.Errorf("read active config before unlink: %w", err)
	}
	if nf5bReferencesInContent(active, oldName) {
		return errors.New("active config still references original list")
	}

	info, err := os.Lstat(libRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect saved config directory before unlink: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("saved config directory is not a real directory")
	}
	entries, err := os.ReadDir(libRoot)
	if err != nil {
		return fmt.Errorf("read saved configs before unlink: %w", err)
	}
	if len(entries) > 512 {
		return errors.New("saved config scan exceeds safety budget")
	}
	for _, entry := range entries {
		if !safeConfigLibraryName(entry.Name()) {
			continue
		}
		data, err := nf5bReadReferenceConfig(filepath.Join(libRoot, entry.Name()))
		if err != nil {
			return fmt.Errorf("inspect saved config %s before unlink: %w", entry.Name(), err)
		}
		if nf5bReferencesInContent(data, oldName) {
			return fmt.Errorf("saved config %s still references original list", entry.Name())
		}
	}
	return nil
}
