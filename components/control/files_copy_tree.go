package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

const (
	adminDirectoryCopyMaxEntries = 4096
	adminDirectoryCopyMaxBytes   = int64(512 * 1024 * 1024)
)

type adminDirectoryCopySnapshot struct {
	Relative string
	Size     int64
	MtimeNS  int64
	Mode     os.FileMode
}

func handleAdminDirectoryCopy(
	w http.ResponseWriter,
	request adminFileCopyRequest,
	source adminFileResolvedPath,
	destination adminFileResolvedPath,
	sourceInfo os.FileInfo,
) {
	if !sourceInfo.IsDir() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "directory copy source is not a directory"})
		return
	}
	if adminFilePathWithinRoot(source.Canonical, destination.Canonical) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "directory cannot be copied inside itself"})
		return
	}

	parent := filepath.Dir(destination.Canonical)
	staging, err := os.MkdirTemp(parent, ".routerforge-copy-dir-*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot create directory copy staging area"})
		return
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(staging)
		}
	}()

	snapshots, totalBytes, err := copyAdminDirectoryTree(source.Canonical, staging)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if err := verifyAdminDirectorySource(source.Canonical, snapshots); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}

	unlockVault, vaultProtected := lockAdminConfigVaultMutation(destination.Canonical)
	defer unlockVault()

	rechecked, err := resolveCreatableAdminFilePath(request.Destination)
	if err != nil || rechecked.Canonical != destination.Canonical {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "copy destination changed during validation"})
		return
	}
	if _, err := os.Lstat(destination.Canonical); !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "copy destination appeared during operation"})
		return
	}
	if vaultProtected {
		if _, err := captureAdminConfigVaultPrechangeLocked("directory-copy", destination.Canonical); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":            "automatic config vault pre-change snapshot failed",
				"mutation_blocked": true,
			})
			return
		}
	}

	if err := os.Rename(staging, destination.Canonical); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot publish copied directory"})
		return
	}
	published = true

	entries, bytesCopied, err := measureAdminDirectoryTree(destination.Canonical)
	if err != nil || entries != len(snapshots) || bytesCopied != totalBytes {
		_ = os.RemoveAll(destination.Canonical)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "directory copy verification failed"})
		return
	}

	log.Printf(
		"routerforge-admin file-mutation action=copy-directory source=%q destination=%q entries=%d bytes=%d",
		source.Lexical,
		destination.Lexical,
		entries,
		bytesCopied,
	)
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":          true,
		"action":      "copy",
		"kind":        "directory",
		"source":      source.Lexical,
		"destination": destination.Lexical,
		"entries":     entries,
		"bytes":       bytesCopied,
	})
}

func copyAdminDirectoryTree(sourceRoot, stagingRoot string) ([]adminDirectoryCopySnapshot, int64, error) {
	rootInfo, err := os.Lstat(sourceRoot)
	if err != nil {
		return nil, 0, err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, 0, errors.New("directory copy source must be a real directory")
	}

	if err := os.Chmod(stagingRoot, rootInfo.Mode().Perm()); err != nil {
		return nil, 0, errors.New("cannot preserve directory root mode")
	}
	if err := copyAdminDirectoryOwnership(stagingRoot, rootInfo); err != nil {
		return nil, 0, errors.New("cannot preserve directory root ownership")
	}

	snapshots := make([]adminDirectoryCopySnapshot, 0, 64)
	directories := make([]adminDirectoryCopySnapshot, 0, 16)
	var totalBytes int64

	err = filepath.Walk(sourceRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("directory copy refuses symbolic links")
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("directory copy supports regular files and directories only")
		}

		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relative == "." {
			relative = ""
		}

		if len(snapshots) >= adminDirectoryCopyMaxEntries {
			return errors.New("directory copy exceeds entry limit")
		}
		snapshot := adminDirectoryCopySnapshot{
			Relative: relative,
			Size:     info.Size(),
			MtimeNS:  info.ModTime().UnixNano(),
			Mode:     info.Mode(),
		}
		snapshots = append(snapshots, snapshot)

		if relative == "" {
			directories = append(directories, snapshot)
			return nil
		}

		target := filepath.Join(stagingRoot, relative)
		if info.IsDir() {
			if err := os.Mkdir(target, info.Mode().Perm()); err != nil {
				return err
			}
			if err := os.Chmod(target, info.Mode().Perm()); err != nil {
				return err
			}
			if err := copyAdminDirectoryOwnership(target, info); err != nil {
				return err
			}
			directories = append(directories, snapshot)
			return nil
		}

		totalBytes += info.Size()
		if totalBytes > adminDirectoryCopyMaxBytes {
			return errors.New("directory copy exceeds byte limit")
		}
		return copyAdminDirectoryRegularFile(path, target, info)
	})
	if err != nil {
		return nil, 0, err
	}

	// Apply directory timestamps after children are created so the copied tree
	// retains the source directory mtimes.
	sort.Slice(directories, func(i, j int) bool {
		return len(directories[i].Relative) > len(directories[j].Relative)
	})
	for _, directory := range directories {
		target := stagingRoot
		if directory.Relative != "" {
			target = filepath.Join(stagingRoot, directory.Relative)
		}
		when := unixNanoTime(directory.MtimeNS)
		if err := os.Chtimes(target, when, when); err != nil {
			return nil, 0, err
		}
	}

	return snapshots, totalBytes, nil
}

func copyAdminDirectoryRegularFile(source, destination string, info os.FileInfo) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = output.Close()
		if !ok {
			_ = os.Remove(destination)
		}
	}()

	if err := output.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if err := copyAdminDirectoryOwnership(destination, info); err != nil {
		return err
	}
	written, err := io.Copy(output, input)
	if err != nil || written != info.Size() {
		return errors.New("cannot copy directory file content")
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	when := info.ModTime()
	if err := os.Chtimes(destination, when, when); err != nil {
		return err
	}
	ok = true
	return nil
}

func copyAdminDirectoryOwnership(path string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return os.Chown(path, int(stat.Uid), int(stat.Gid))
}

func verifyAdminDirectorySource(sourceRoot string, snapshots []adminDirectoryCopySnapshot) error {
	seen := make(map[string]bool, len(snapshots))
	for _, snapshot := range snapshots {
		seen[snapshot.Relative] = true
		path := sourceRoot
		if snapshot.Relative != "" {
			path = filepath.Join(sourceRoot, snapshot.Relative)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return errors.New("directory source changed during copy")
		}
		if info.Mode()&os.ModeSymlink != 0 ||
			info.Size() != snapshot.Size ||
			info.ModTime().UnixNano() != snapshot.MtimeNS ||
			info.Mode().Type() != snapshot.Mode.Type() {
			return errors.New("directory source changed during copy")
		}
	}

	count := 0
	err := filepath.Walk(sourceRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("directory source changed during copy")
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relative == "." {
			relative = ""
		}
		if !seen[relative] {
			return errors.New("directory source changed during copy")
		}
		count++
		return nil
	})
	if err != nil || count != len(snapshots) {
		return errors.New("directory source changed during copy")
	}
	return nil
}

func measureAdminDirectoryTree(root string) (int, int64, error) {
	entries := 0
	var totalBytes int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("copied directory contains a symbolic link")
		}
		entries++
		if info.Mode().IsRegular() {
			totalBytes += info.Size()
		}
		return nil
	})
	return entries, totalBytes, err
}

func unixNanoTime(value int64) (t time.Time) {
	return time.Unix(0, value)
}
