package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

type neighborConfigFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified string `json:"modified_at,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

type neighborConfigImportRequest struct {
	Name        string `json:"name"`
	Destination string `json:"destination"`
	Confirm     string `json:"confirm"`
}

func neighborConfigRoot() string {
	return filepath.Dir(configPath)
}

func safeNeighborConfigName(name string) bool {
	if name == "" || filepath.Base(name) != name || strings.HasPrefix(name, ".") {
		return false
	}
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".conf") && !strings.HasSuffix(lower, ".cfg") {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func neighborConfigPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !safeNeighborConfigName(name) {
		return "", errors.New("neighbor config filename must end with .conf or .cfg and contain only letters, digits, dot, _ or -")
	}
	path := filepath.Join(neighborConfigRoot(), name)
	if filepath.Clean(path) == filepath.Clean(configPath) {
		return "", errors.New("canonical nfqws2.conf cannot be imported as a neighbor")
	}
	return path, nil
}

func neighborConfigRegularFile(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("neighbor config is not a regular file")
	}
	if info.Size() > configMaxBytes {
		return nil, errors.New("neighbor config exceeds editor limit")
	}
	return info, nil
}

func readNeighborConfigFiles() []neighborConfigFile {
	entries, err := os.ReadDir(neighborConfigRoot())
	if err != nil {
		return []neighborConfigFile{}
	}
	out := make([]neighborConfigFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !safeNeighborConfigName(entry.Name()) {
			continue
		}
		path := filepath.Join(neighborConfigRoot(), entry.Name())
		if filepath.Clean(path) == filepath.Clean(configPath) {
			continue
		}
		info, err := neighborConfigRegularFile(path)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out = append(out, neighborConfigFile{
			Name: entry.Name(), Path: path, Size: info.Size(),
			Modified: info.ModTime().UTC().Format(time.RFC3339), SHA256: configHash(data),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func handleNeighborConfigList(w http.ResponseWriter, _ *http.Request) {
	files := readNeighborConfigFiles()
	writeJSON(w, http.StatusOK, map[string]any{
		"root": neighborConfigRoot(), "files": files, "count": len(files),
	})
}

func handleNeighborConfigImport(w http.ResponseWriter, r *http.Request) {
	var request neighborConfigImportRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid neighbor config import request"})
		return
	}
	if request.Confirm != "NFQWS_CONFIG_LIBRARY_IMPORT" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_CONFIG_LIBRARY_IMPORT"})
		return
	}
	source, err := neighborConfigPath(request.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	destination, err := configLibraryPath(request.Destination)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if _, err := neighborConfigRegularFile(source); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "neighbor config not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	data, cut, err := readBoundedFile(source, configMaxBytes)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read neighbor config: " + err.Error()})
		return
	}
	if cut {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "neighbor config exceeds editor limit"})
		return
	}
	if err := validateLibraryConfig(string(data)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "neighbor config rejected: " + err.Error()})
		return
	}
	if err := ensureConfigLibraryRoot(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create config library: " + err.Error()})
		return
	}
	if _, err := os.Lstat(destination); err == nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "destination config already exists"})
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat destination config: " + err.Error()})
		return
	}
	if err := safety.WriteFileAtomic(destination, data, 0600); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "import neighbor config: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "name": request.Destination, "source": request.Name,
		"library": configLibraryResponse(), "neighbors": readNeighborConfigFiles(),
	})
}
