package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"
)

func handleAdminFileHash(w http.ResponseWriter, r *http.Request) {
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

	info, err := file.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot inspect file"})
		return
	}
	if !info.Mode().IsRegular() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "file path is not a regular file"})
		return
	}

	algorithm := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("algorithm")))
	var digest string
	switch algorithm {
	case "md5":
		hasher := md5.New()
		if _, err := io.Copy(hasher, file); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot hash file"})
			return
		}
		digest = hex.EncodeToString(hasher.Sum(nil))
	case "sha256":
		hasher := sha256.New()
		if _, err := io.Copy(hasher, file); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot hash file"})
			return
		}
		digest = hex.EncodeToString(hasher.Sum(nil))
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "algorithm must be md5 or sha256"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":      resolved.Lexical,
		"algorithm": algorithm,
		"digest":    digest,
		"size":      info.Size(),
	})
}
