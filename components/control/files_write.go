package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"
)

const (
	adminFileWriteContentLimit     int64 = 128 << 10
	adminFileWriteRequestBodyLimit int64 = 272 << 10
)

type adminFileMkdirRequest struct {
	Path        string `json:"path"`
	ConfirmPath string `json:"confirm_path"`
}

type adminFileWriteRequest struct {
	Path            string `json:"path"`
	ConfirmPath     string `json:"confirm_path"`
	Content         string `json:"content"`
	Create          bool   `json:"create"`
	ExpectedSize    *int64 `json:"expected_size,omitempty"`
	ExpectedMtimeNS *int64 `json:"expected_mtime_ns,string,omitempty"`
}

func registerAdminFileMutationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/files/mkdir", mutationOnly(handleAdminFileMkdir))
	mux.HandleFunc("/v1/files/write", mutationOnly(handleAdminFileWrite))
	mux.HandleFunc("/v1/files/move", mutationOnly(handleAdminFileMove))
	mux.HandleFunc("/v1/files/copy", mutationOnly(handleAdminFileCopy))
	mux.HandleFunc("/v1/files/delete", mutationOnly(handleAdminFileDelete))
	mux.HandleFunc("/v1/files/chmod", mutationOnly(handleAdminFileChmod))
}

func exactAdminFileConfirmation(path, confirmPath string) bool {
	return path != "" && path == confirmPath
}

func decodeAdminFileWriteJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, adminFileWriteRequestBodyLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func handleAdminFileMkdir(w http.ResponseWriter, r *http.Request) {
	var request adminFileMkdirRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid directory creation request"})
		return
	}
	if !exactAdminFileConfirmation(request.Path, request.ConfirmPath) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm_path does not match target path"})
		return
	}

	resolved, err := resolveCreatableAdminFilePath(request.Path)
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}
	if _, err := os.Lstat(resolved.Lexical); err == nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "target path already exists"})
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot inspect target path"})
		return
	}

	rechecked, err := resolveCreatableAdminFilePath(request.Path)
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}
	if rechecked.Canonical != resolved.Canonical {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "target path changed during validation"})
		return
	}

	if err := os.Mkdir(resolved.Canonical, 0o755); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrExist) {
			status = http.StatusConflict
		} else if errors.Is(err, os.ErrPermission) {
			status = http.StatusForbidden
		}
		writeJSON(w, status, map[string]any{"error": "cannot create directory"})
		return
	}

	info, err := os.Stat(resolved.Canonical)
	if err != nil || !info.IsDir() {
		_ = os.Remove(resolved.Canonical)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "directory creation verification failed"})
		return
	}

	log.Printf("routerforge-admin file-mutation action=mkdir path=%q", resolved.Lexical)
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":          true,
		"action":      "mkdir",
		"path":        resolved.Lexical,
		"mode":        info.Mode().String(),
		"modified_at": info.ModTime(),
	})
}

func validateAdminFileWriteContent(content string) error {
	if int64(len(content)) > adminFileWriteContentLimit {
		return fmt.Errorf("file content exceeds %d bytes", adminFileWriteContentLimit)
	}
	if strings.IndexByte(content, 0) >= 0 || !utf8.ValidString(content) {
		return errors.New("file content must be UTF-8 text without NUL")
	}
	return nil
}

func preserveAdminFileMetadata(tempPath string, info os.FileInfo) error {
	if err := os.Chmod(tempPath, info.Mode().Perm()); err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("file ownership metadata is unavailable")
	}
	return os.Chown(tempPath, int(stat.Uid), int(stat.Gid))
}

func adminFileWriteState(path string) (os.FileInfo, bool, error) {
	info, err := os.Lstat(path)
	if err == nil {
		return info, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return nil, false, err
}

func checkAdminFileWritePrecondition(request adminFileWriteRequest, info os.FileInfo, exists bool) error {
	if request.Create {
		if exists {
			return errors.New("create target already exists")
		}
		if request.ExpectedSize != nil || request.ExpectedMtimeNS != nil {
			return errors.New("create request must not include existing-file preconditions")
		}
		return nil
	}

	if !exists {
		return os.ErrNotExist
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("writing through a symlink leaf is not allowed")
	}
	if !info.Mode().IsRegular() {
		return errors.New("target path is not a regular file")
	}
	if request.ExpectedSize == nil || request.ExpectedMtimeNS == nil {
		return errors.New("expected_size and expected_mtime_ns are required for edit")
	}
	if info.Size() != *request.ExpectedSize || info.ModTime().UnixNano() != *request.ExpectedMtimeNS {
		return errors.New("file changed since it was read")
	}
	return nil
}

func writeAdminFilePreconditionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "file path not found"})
	case strings.Contains(err.Error(), "required for edit"):
		writeJSON(w, http.StatusPreconditionRequired, map[string]any{"error": err.Error()})
	case strings.Contains(err.Error(), "changed since") || strings.Contains(err.Error(), "already exists"):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
	case strings.Contains(err.Error(), "symlink") || strings.Contains(err.Error(), "not a regular file"):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid file write precondition"})
	}
}

func handleAdminFileWrite(w http.ResponseWriter, r *http.Request) {
	var request adminFileWriteRequest
	if err := decodeAdminFileWriteJSON(w, r, &request); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"error":          "file write request body exceeds limit",
				"max_body_bytes": adminFileWriteRequestBodyLimit,
			})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid file write request"})
		return
	}
	if !exactAdminFileConfirmation(request.Path, request.ConfirmPath) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm_path does not match target path"})
		return
	}
	if err := validateAdminFileWriteContent(request.Content); err != nil {
		status := http.StatusUnsupportedMediaType
		if strings.Contains(err.Error(), "exceeds") {
			status = http.StatusRequestEntityTooLarge
		}
		writeJSON(w, status, map[string]any{
			"error":             err.Error(),
			"max_content_bytes": adminFileWriteContentLimit,
		})
		return
	}

	resolved, err := resolveCreatableAdminFilePath(request.Path)
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}

	before, exists, err := adminFileWriteState(resolved.Lexical)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot inspect target file"})
		return
	}
	if err := checkAdminFileWritePrecondition(request, before, exists); err != nil {
		writeAdminFilePreconditionError(w, err)
		return
	}

	parent := filepath.Dir(resolved.Canonical)
	temp, err := os.CreateTemp(parent, ".routerforge-write-*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot create atomic write temporary file"})
		return
	}
	tempPath := temp.Name()
	tempClosed := false
	defer func() {
		if !tempClosed {
			_ = temp.Close()
		}
		_ = os.Remove(tempPath)
	}()

	if exists {
		if err := preserveAdminFileMetadata(tempPath, before); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot preserve file metadata"})
			return
		}
	} else if err := temp.Chmod(0o600); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot set new file mode"})
		return
	}

	if _, err := io.WriteString(temp, request.Content); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot write temporary file"})
		return
	}
	if err := temp.Sync(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot sync temporary file"})
		return
	}
	if err := temp.Close(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot close temporary file"})
		return
	}
	tempClosed = true

	rechecked, err := resolveCreatableAdminFilePath(request.Path)
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}
	if rechecked.Canonical != resolved.Canonical {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "target path changed during validation"})
		return
	}

	current, currentExists, err := adminFileWriteState(rechecked.Lexical)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot recheck target file"})
		return
	}
	if err := checkAdminFileWritePrecondition(request, current, currentExists); err != nil {
		writeAdminFilePreconditionError(w, err)
		return
	}

	if err := os.Rename(tempPath, resolved.Canonical); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot atomically replace target file"})
		return
	}

	after, err := os.Stat(resolved.Canonical)
	if err != nil || !after.Mode().IsRegular() || after.Size() != int64(len(request.Content)) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "file write verification failed"})
		return
	}

	action := "edit"
	status := http.StatusOK
	if request.Create {
		action = "create"
		status = http.StatusCreated
	}
	log.Printf("routerforge-admin file-mutation action=%s path=%q bytes=%d", action, resolved.Lexical, len(request.Content))
	writeJSON(w, status, map[string]any{
		"ok":          true,
		"action":      action,
		"path":        resolved.Lexical,
		"size":        after.Size(),
		"mode":        after.Mode().String(),
		"modified_at": after.ModTime(),
	})
}
