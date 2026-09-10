package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
)

type adminFileCopyRequest struct {
	Source               string `json:"source"`
	Destination          string `json:"destination"`
	ConfirmSource        string `json:"confirm_source"`
	ConfirmDestination   string `json:"confirm_destination"`
	ExpectedSize         int64  `json:"expected_size"`
	ExpectedMtimeNS      int64  `json:"expected_mtime_ns,string"`
}

func handleAdminFileCopy(w http.ResponseWriter, r *http.Request) {
	var request adminFileCopyRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid file copy request"})
		return
	}
	if request.Source == "" || request.Source != request.ConfirmSource ||
		request.Destination == "" || request.Destination != request.ConfirmDestination {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "copy confirmation paths do not match"})
		return
	}

	source, err := resolveExistingAdminFilePath(request.Source)
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}
	destination, err := resolveCreatableAdminFilePath(request.Destination)
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}
	if source.Canonical == destination.Canonical {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "copy source and destination are identical"})
		return
	}

	sourceInfo, err := os.Lstat(source.Canonical)
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 || !sourceInfo.Mode().IsRegular() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "copy currently supports regular files only"})
		return
	}
	if sourceInfo.Size() != request.ExpectedSize || sourceInfo.ModTime().UnixNano() != request.ExpectedMtimeNS {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "copy source changed since it was listed"})
		return
	}
	if _, err := os.Lstat(destination.Canonical); err == nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "copy destination already exists"})
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot inspect copy destination"})
		return
	}

	input, err := os.Open(source.Canonical)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot open copy source"})
		return
	}
	defer input.Close()

	parent := filepath.Dir(destination.Canonical)
	temp, err := os.CreateTemp(parent, ".routerforge-copy-*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot create copy temporary file"})
		return
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		_ = os.Remove(tempPath)
	}()

	if err := temp.Chmod(sourceInfo.Mode().Perm()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot preserve copy mode"})
		return
	}
	if stat, ok := sourceInfo.Sys().(*syscall.Stat_t); ok {
		if err := temp.Chown(int(stat.Uid), int(stat.Gid)); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot preserve copy ownership"})
			return
		}
	}

	written, err := io.Copy(temp, input)
	if err != nil || written != sourceInfo.Size() {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot copy file content"})
		return
	}
	if err := temp.Sync(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot sync copied file"})
		return
	}
	if err := temp.Close(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot close copied file"})
		return
	}
	closed = true

	rechecked, err := resolveCreatableAdminFilePath(request.Destination)
	if err != nil || rechecked.Canonical != destination.Canonical {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "copy destination changed during validation"})
		return
	}
	if _, err := os.Lstat(destination.Canonical); !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "copy destination appeared during operation"})
		return
	}
	if err := os.Rename(tempPath, destination.Canonical); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot publish copied file"})
		return
	}
	after, err := os.Stat(destination.Canonical)
	if err != nil || !after.Mode().IsRegular() || after.Size() != sourceInfo.Size() {
		_ = os.Remove(destination.Canonical)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "copy verification failed"})
		return
	}

	log.Printf("routerforge-admin file-mutation action=copy source=%q destination=%q bytes=%d", source.Lexical, destination.Lexical, after.Size())
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":          true,
		"action":      "copy",
		"source":      source.Lexical,
		"destination": destination.Lexical,
		"size":        after.Size(),
		"mode":        after.Mode().String(),
		"modified_at": after.ModTime(),
	})
}
