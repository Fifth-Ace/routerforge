package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type adminFileMoveRequest struct {
	Source          string `json:"source"`
	Destination     string `json:"destination"`
	ConfirmSource   string `json:"confirm_source"`
	ConfirmDest     string `json:"confirm_destination"`
	ExpectedSize    *int64 `json:"expected_size,omitempty"`
	ExpectedMtimeNS *int64 `json:"expected_mtime_ns,string,omitempty"`
}

type adminFileDeleteRequest struct {
	Path            string `json:"path"`
	ConfirmPath     string `json:"confirm_path"`
	ExpectedSize    *int64 `json:"expected_size,omitempty"`
	ExpectedMtimeNS *int64 `json:"expected_mtime_ns,string,omitempty"`
}

type adminFileChmodRequest struct {
	Path            string `json:"path"`
	ConfirmPath     string `json:"confirm_path"`
	Mode            string `json:"mode"`
	ExpectedSize    *int64 `json:"expected_size,omitempty"`
	ExpectedMtimeNS *int64 `json:"expected_mtime_ns,string,omitempty"`
}

func decodeAdminFileMutationJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
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

func adminFileResolvedIsRoot(resolved adminFileResolvedPath) (bool, error) {
	canonicalRoot, err := canonicalAdminFileRoot(resolved.Root)
	if err != nil {
		return false, err
	}
	return filepath.Clean(resolved.Canonical) == filepath.Clean(canonicalRoot), nil
}

func inspectAdminFileMutationTarget(raw string) (adminFileResolvedPath, os.FileInfo, error) {
	resolved, err := resolveExistingAdminFilePath(raw)
	if err != nil {
		return adminFileResolvedPath{}, nil, err
	}
	if adminFileRootReadOnly(resolved.Root) {
		return adminFileResolvedPath{}, nil, errors.New("allowed root is read-only")
	}
	info, err := os.Lstat(resolved.Lexical)
	if err != nil {
		return adminFileResolvedPath{}, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return adminFileResolvedPath{}, nil, errors.New("mutating a symlink leaf is not allowed")
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return adminFileResolvedPath{}, nil, errors.New("target path is not a regular file or directory")
	}
	isRoot, err := adminFileResolvedIsRoot(resolved)
	if err != nil {
		return adminFileResolvedPath{}, nil, err
	}
	if isRoot {
		return adminFileResolvedPath{}, nil, errors.New("allowed root itself cannot be mutated")
	}
	return resolved, info, nil
}

func checkAdminFileObjectPrecondition(info os.FileInfo, expectedSize, expectedMtimeNS *int64) error {
	if expectedSize == nil || expectedMtimeNS == nil {
		return errors.New("expected_size and expected_mtime_ns are required")
	}
	if info.Size() != *expectedSize || info.ModTime().UnixNano() != *expectedMtimeNS {
		return errors.New("file object changed since it was observed")
	}
	return nil
}

func writeAdminFileObjectError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "file path not found"})
	case errors.Is(err, os.ErrPermission):
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "filesystem permission denied"})
	case strings.Contains(err.Error(), "outside allowed roots"),
		strings.Contains(err.Error(), "escapes allowed root"),
		strings.Contains(err.Error(), "allowed root itself"):
		writeJSON(w, http.StatusForbidden, map[string]any{"error": err.Error()})
	case strings.Contains(err.Error(), "are required"):
		writeJSON(w, http.StatusPreconditionRequired, map[string]any{"error": err.Error()})
	case strings.Contains(err.Error(), "changed since"):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
}

func recheckAdminFileMutationTarget(raw, canonical string, expectedSize, expectedMtimeNS *int64) (adminFileResolvedPath, os.FileInfo, error) {
	resolved, info, err := inspectAdminFileMutationTarget(raw)
	if err != nil {
		return adminFileResolvedPath{}, nil, err
	}
	if resolved.Canonical != canonical {
		return adminFileResolvedPath{}, nil, errors.New("target path changed during validation")
	}
	if err := checkAdminFileObjectPrecondition(info, expectedSize, expectedMtimeNS); err != nil {
		return adminFileResolvedPath{}, nil, err
	}
	return resolved, info, nil
}

func destinationAbsent(raw string) (adminFileResolvedPath, error) {
	resolved, err := resolveCreatableAdminFilePath(raw)
	if err != nil {
		return adminFileResolvedPath{}, err
	}
	if _, err := os.Lstat(resolved.Lexical); err == nil {
		return adminFileResolvedPath{}, errors.New("destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return adminFileResolvedPath{}, err
	}
	return resolved, nil
}

func handleAdminFileMove(w http.ResponseWriter, r *http.Request) {
	var request adminFileMoveRequest
	if err := decodeAdminFileMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid file move request"})
		return
	}
	if !exactAdminFileConfirmation(request.Source, request.ConfirmSource) ||
		!exactAdminFileConfirmation(request.Destination, request.ConfirmDest) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "move confirmations do not match source/destination"})
		return
	}

	source, info, err := inspectAdminFileMutationTarget(request.Source)
	if err != nil {
		writeAdminFileObjectError(w, err)
		return
	}
	if err := checkAdminFileObjectPrecondition(info, request.ExpectedSize, request.ExpectedMtimeNS); err != nil {
		writeAdminFileObjectError(w, err)
		return
	}

	destination, err := destinationAbsent(request.Destination)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
			return
		}
		writeAdminFileObjectError(w, err)
		return
	}
	if source.Root != destination.Root {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "move across allowed roots is not enabled"})
		return
	}
	if source.Canonical == destination.Canonical {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "source and destination resolve to the same path"})
		return
	}
	if info.IsDir() && adminFilePathWithinRoot(source.Canonical, destination.Canonical) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "directory cannot be moved inside itself"})
		return
	}

	recheckedSource, _, err := recheckAdminFileMutationTarget(
		request.Source,
		source.Canonical,
		request.ExpectedSize,
		request.ExpectedMtimeNS,
	)
	if err != nil {
		writeAdminFileObjectError(w, err)
		return
	}
	recheckedDestination, err := destinationAbsent(request.Destination)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
			return
		}
		writeAdminFileObjectError(w, err)
		return
	}
	if recheckedDestination.Canonical != destination.Canonical {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "destination path changed during validation"})
		return
	}

	if err := os.Rename(recheckedSource.Canonical, recheckedDestination.Canonical); err != nil {
		if errors.Is(err, os.ErrPermission) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "filesystem permission denied"})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": "atomic move failed; cross-filesystem moves are not enabled"})
		return
	}

	if _, err := os.Lstat(recheckedSource.Canonical); !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "move verification failed at source"})
		return
	}
	after, err := os.Lstat(recheckedDestination.Canonical)
	if err != nil || after.Mode()&os.ModeSymlink != 0 {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "move verification failed at destination"})
		return
	}

	log.Printf("routerforge-admin file-mutation action=move source=%q destination=%q", request.Source, request.Destination)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"action":      "move",
		"source":      request.Source,
		"destination": request.Destination,
		"size":        after.Size(),
		"mode":        after.Mode().String(),
		"modified_at": after.ModTime(),
	})
}

func handleAdminFileDelete(w http.ResponseWriter, r *http.Request) {
	var request adminFileDeleteRequest
	if err := decodeAdminFileMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid file delete request"})
		return
	}
	if !exactAdminFileConfirmation(request.Path, request.ConfirmPath) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm_path does not match target path"})
		return
	}

	resolved, info, err := inspectAdminFileMutationTarget(request.Path)
	if err != nil {
		writeAdminFileObjectError(w, err)
		return
	}
	if err := checkAdminFileObjectPrecondition(info, request.ExpectedSize, request.ExpectedMtimeNS); err != nil {
		writeAdminFileObjectError(w, err)
		return
	}

	rechecked, _, err := recheckAdminFileMutationTarget(
		request.Path,
		resolved.Canonical,
		request.ExpectedSize,
		request.ExpectedMtimeNS,
	)
	if err != nil {
		writeAdminFileObjectError(w, err)
		return
	}

	if err := os.Remove(rechecked.Canonical); err != nil {
		if errors.Is(err, os.ErrPermission) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "filesystem permission denied"})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": "delete failed; directories must be empty"})
		return
	}
	if _, err := os.Lstat(rechecked.Canonical); !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete verification failed"})
		return
	}

	log.Printf("routerforge-admin file-mutation action=delete path=%q", request.Path)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"action": "delete",
		"path":   request.Path,
	})
}

func parseAdminFileMode(raw string) (os.FileMode, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 3 {
		raw = "0" + raw
	}
	if len(raw) != 4 || raw[0] != '0' {
		return 0, errors.New("mode must be an octal string from 0000 to 0777")
	}
	value, err := strconv.ParseUint(raw, 8, 12)
	if err != nil || value > 0o777 {
		return 0, errors.New("mode must be an octal string from 0000 to 0777")
	}
	return os.FileMode(value), nil
}

func handleAdminFileChmod(w http.ResponseWriter, r *http.Request) {
	var request adminFileChmodRequest
	if err := decodeAdminFileMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid file chmod request"})
		return
	}
	if !exactAdminFileConfirmation(request.Path, request.ConfirmPath) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm_path does not match target path"})
		return
	}
	mode, err := parseAdminFileMode(request.Mode)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	resolved, info, err := inspectAdminFileMutationTarget(request.Path)
	if err != nil {
		writeAdminFileObjectError(w, err)
		return
	}
	if err := checkAdminFileObjectPrecondition(info, request.ExpectedSize, request.ExpectedMtimeNS); err != nil {
		writeAdminFileObjectError(w, err)
		return
	}

	rechecked, _, err := recheckAdminFileMutationTarget(
		request.Path,
		resolved.Canonical,
		request.ExpectedSize,
		request.ExpectedMtimeNS,
	)
	if err != nil {
		writeAdminFileObjectError(w, err)
		return
	}

	if err := os.Chmod(rechecked.Canonical, mode); err != nil {
		if errors.Is(err, os.ErrPermission) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "filesystem permission denied"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "chmod failed"})
		return
	}
	after, err := os.Lstat(rechecked.Canonical)
	if err != nil || after.Mode().Perm() != mode.Perm() {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "chmod verification failed"})
		return
	}

	log.Printf("routerforge-admin file-mutation action=chmod path=%q mode=%04o", request.Path, mode.Perm())
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"action":      "chmod",
		"path":        request.Path,
		"mode":        after.Mode().String(),
		"mode_octal":  "0" + strconv.FormatUint(uint64(after.Mode().Perm()), 8),
		"modified_at": after.ModTime(),
	})
}
