package main

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	adminFileReadLimit      int64 = 256 << 10
	adminFileListEntryLimit       = 2048
)

type adminFileEntry struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Kind         string    `json:"kind"`
	Size         int64     `json:"size"`
	Mode         string    `json:"mode"`
	ModifiedAt   time.Time `json:"modified_at"`
	ModifiedAtNS int64     `json:"mtime_ns,string"`
}

type adminFileReadResponse struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	Mode         string    `json:"mode"`
	ModifiedAt   time.Time `json:"modified_at"`
	ModifiedAtNS int64     `json:"mtime_ns,string"`
	Encoding     string    `json:"encoding"`
	Content      string    `json:"content"`
}

func registerAdminFileReadRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/files/list", adminFileAccessOnly(false, handleAdminFileList))
	mux.HandleFunc("/v1/files/read", adminFileAccessOnly(false, handleAdminFileRead))
	mux.HandleFunc("/v1/files/download", adminFileAccessOnly(true, handleAdminFileDownload))
}

func adminFileAccessOnly(allowHead bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		methodAllowed := r.Method == http.MethodGet || (allowHead && r.Method == http.MethodHead)
		if !methodAllowed {
			allow := http.MethodGet
			if allowHead {
				allow = "GET, HEAD"
			}
			w.Header().Set("Allow", allow)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method is not allowed for Admin file access"})
			return
		}
		if r.Header.Get(adminMutationAuthorizationHeader) != adminMutationAuthorizationValue {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "authorized RouterForge Core root session required for Admin file access"})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next(w, r)
	}
}

func adminFilePathErrorStatus(err error) int {
	if errors.Is(err, os.ErrNotExist) {
		return http.StatusNotFound
	}
	message := err.Error()
	if strings.Contains(message, "outside allowed roots") || strings.Contains(message, "escapes allowed root") {
		return http.StatusForbidden
	}
	return http.StatusBadRequest
}

func writeAdminFilePathError(w http.ResponseWriter, err error) {
	status := adminFilePathErrorStatus(err)
	message := "invalid file path"
	switch status {
	case http.StatusNotFound:
		message = "file path not found"
	case http.StatusForbidden:
		message = "file path is outside the allowed boundary"
	}
	writeJSON(w, status, map[string]any{"error": message})
}

func adminFileKind(mode os.FileMode) string {
	switch {
	case mode&os.ModeSymlink != 0:
		return "symlink"
	case mode.IsDir():
		return "directory"
	case mode.IsRegular():
		return "file"
	default:
		return "other"
	}
}

func handleAdminFileList(w http.ResponseWriter, r *http.Request) {
	resolved, err := resolveExistingAdminFilePath(r.URL.Query().Get("path"))
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}

	dir, err := os.Open(resolved.Canonical)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot open directory"})
		return
	}
	defer dir.Close()

	stat, err := dir.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot inspect directory"})
		return
	}
	if !stat.IsDir() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "file path is not a directory"})
		return
	}

	entries, readErr := dir.ReadDir(adminFileListEntryLimit + 1)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot read directory"})
		return
	}
	if len(entries) > adminFileListEntryLimit {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error":       "directory entry limit exceeded",
			"max_entries": adminFileListEntryLimit,
		})
		return
	}

	result := make([]adminFileEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot inspect directory entry"})
			return
		}
		result = append(result, adminFileEntry{
			Name:         entry.Name(),
			Path:         filepath.Join(resolved.Lexical, entry.Name()),
			Kind:         adminFileKind(info.Mode()),
			Size:         info.Size(),
			Mode:         info.Mode().String(),
			ModifiedAt:   info.ModTime(),
			ModifiedAtNS: info.ModTime().UnixNano(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		leftDir := result[i].Kind == "directory"
		rightDir := result[j].Kind == "directory"
		if leftDir != rightDir {
			return leftDir
		}
		left := strings.ToLower(result[i].Name)
		right := strings.ToLower(result[j].Name)
		if left == right {
			return result[i].Name < result[j].Name
		}
		return left < right
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"path":        resolved.Lexical,
		"entries":     result,
		"entry_count": len(result),
		"max_entries": adminFileListEntryLimit,
	})
}

func handleAdminFileRead(w http.ResponseWriter, r *http.Request) {
	resolved, err := resolveExistingAdminFilePath(r.URL.Query().Get("path"))
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}

	file, err := os.Open(resolved.Canonical)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot open file"})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot inspect file"})
		return
	}
	if !stat.Mode().IsRegular() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "file path is not a regular file"})
		return
	}
	if stat.Size() > adminFileReadLimit {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error":          "text file exceeds read limit",
			"max_read_bytes": adminFileReadLimit,
		})
		return
	}

	content, err := io.ReadAll(io.LimitReader(file, adminFileReadLimit+1))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot read file"})
		return
	}
	if int64(len(content)) > adminFileReadLimit {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error":          "text file exceeds read limit",
			"max_read_bytes": adminFileReadLimit,
		})
		return
	}
	if bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content) {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "file is not UTF-8 text"})
		return
	}

	writeJSON(w, http.StatusOK, adminFileReadResponse{
		Path:         resolved.Lexical,
		Size:         int64(len(content)),
		Mode:         stat.Mode().String(),
		ModifiedAt:   stat.ModTime(),
		ModifiedAtNS: stat.ModTime().UnixNano(),
		Encoding:     "utf-8",
		Content:      string(content),
	})
}

func handleAdminFileDownload(w http.ResponseWriter, r *http.Request) {
	resolved, err := resolveExistingAdminFilePath(r.URL.Query().Get("path"))
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}

	file, err := os.Open(resolved.Canonical)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot open file"})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot inspect file"})
		return
	}
	if !stat.Mode().IsRegular() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "file path is not a regular file"})
		return
	}

	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(resolved.Lexical)})
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, filepath.Base(resolved.Lexical), stat.ModTime(), file)
}
