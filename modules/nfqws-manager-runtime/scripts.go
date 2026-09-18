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

func scriptTargetAllowed(path string) bool {
	clean := filepath.Clean(path)
	if !strings.HasSuffix(strings.ToLower(clean), ".lua") {
		return false
	}
	rel, err := filepath.Rel("/opt", clean)
	if err != nil || rel == ".." || filepath.IsAbs(rel) {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func resolveScriptPath(name string) (string, string, os.FileInfo, error) {
	if !safeScriptName(name) {
		return "", "", nil, errors.New("invalid lua script filename")
	}
	logical := filepath.Join(scriptsRoot, name)
	resolved, err := filepath.EvalSymlinks(logical)
	if err != nil {
		return logical, "", nil, err
	}
	resolved = filepath.Clean(resolved)
	if !scriptTargetAllowed(resolved) {
		return logical, resolved, nil, errors.New("lua script target must stay below /opt and end with .lua")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return logical, resolved, nil, err
	}
	if !info.Mode().IsRegular() {
		return logical, resolved, nil, errors.New("lua script target must be a regular file")
	}
	return logical, resolved, info, nil
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
		logical, _, info, err := resolveScriptPath(entry.Name())
		if err != nil {
			continue
		}
		out = append(out, filePreview{
			Name:      entry.Name(),
			Path:      logical,
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
	logical, resolved, info, err := resolveScriptPath(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "lua script not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "lua script is not readable: " + err.Error()})
		return
	}
	data, cut, err := readBoundedFile(resolved, scriptFileMaxBytes)
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
		"path":      logical,
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
