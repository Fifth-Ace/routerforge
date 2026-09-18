package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	scriptsRoot        = "/opt/etc/nfqws2/lua"
	scriptFileMaxBytes = 512 << 10
)

func safeScriptName(name string) bool {
	if !safeListName(name) {
		return false
	}
	return strings.HasSuffix(strings.ToLower(name), ".lua")
}

func readScriptInventory() []filePreview {
	entries, err := os.ReadDir(scriptsRoot)
	if err != nil {
		return []filePreview{}
	}
	out := make([]filePreview, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !safeScriptName(entry.Name()) {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, filePreview{
			Name:      entry.Name(),
			Path:      filepath.Join(scriptsRoot, entry.Name()),
			Size:      info.Size(),
			Modified:  info.ModTime().UTC().Format(time.RFC3339),
			Truncated: info.Size() > scriptFileMaxBytes,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func handleScripts(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"root":      scriptsRoot,
			"files":     readScriptInventory(),
			"read_only": true,
		})
		return
	}
	if !safeScriptName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid lua script filename"})
		return
	}
	path := filepath.Join(scriptsRoot, name)
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "lua script not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat lua script: " + err.Error()})
		return
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "lua script must be a regular file"})
		return
	}
	data, cut, err := readBoundedFile(path, scriptFileMaxBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "lua script not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read lua script: " + err.Error()})
		return
	}
	if cut {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "lua script exceeds viewer limit"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":      name,
		"path":      path,
		"content":   string(data),
		"size":      len(data),
		"read_only": true,
		"modified_at": func() string {
			if info == nil {
				return ""
			}
			return info.ModTime().UTC().Format(time.RFC3339)
		}(),
	})
}
