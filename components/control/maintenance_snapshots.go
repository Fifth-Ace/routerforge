package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	adminSnapshotRoot     = "/tmp/routerforge-snapshots"
	adminSnapshotMaxFiles = 32
	adminSnapshotMaxBytes = 8 << 20
)

type adminSnapshotRequest struct {
	Confirm string `json:"confirm"`
}

type adminSnapshotDeleteRequest struct {
	Path        string `json:"path"`
	ConfirmPath string `json:"confirm_path"`
	Confirm     string `json:"confirm"`
}

type adminSnapshotDocument struct {
	SchemaVersion int                    `json:"schema_version"`
	CreatedAt     time.Time              `json:"created_at"`
	Summary       adminSummary           `json:"summary"`
	Processes     []processInfo          `json:"processes"`
	Services      []serviceInfo          `json:"services"`
	Ports         []portInfo             `json:"ports"`
	Storage       []storageInfo          `json:"storage"`
	Thermal       []thermalInfo          `json:"thermal"`
	Integrations  []adminIntegrationInfo `json:"integrations"`
}

type adminSnapshotInfo struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

func registerAdminSnapshotRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/maintenance/snapshots", getOnly(handleAdminSnapshots))
	mux.HandleFunc("/v1/maintenance/snapshot", mutationOnly(handleAdminSnapshotCreate))
	mux.HandleFunc("/v1/maintenance/snapshot/delete", mutationOnly(handleAdminSnapshotDelete))
}

func handleAdminSnapshots(w http.ResponseWriter, _ *http.Request) {
	snapshots, err := listAdminSnapshots()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"snapshots": snapshots,
		"root":      adminSnapshotRoot,
		"max_files": adminSnapshotMaxFiles,
	})
}

func handleAdminSnapshotCreate(w http.ResponseWriter, r *http.Request) {
	var request adminSnapshotRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid snapshot request"})
		return
	}
	if request.Confirm != "SNAPSHOT" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal SNAPSHOT"})
		return
	}

	path, size, err := createAdminSnapshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"path": path,
		"size": size,
	})
}

func handleAdminSnapshotDelete(w http.ResponseWriter, r *http.Request) {
	var request adminSnapshotDeleteRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid snapshot delete request"})
		return
	}
	path, err := resolveAdminSnapshotPath(request.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if request.ConfirmPath != path {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm_path does not match snapshot path"})
		return
	}
	if request.Confirm != "DELETE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal DELETE"})
		return
	}
	if err := os.Remove(path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": path})
}

func createAdminSnapshot() (string, int64, error) {
	if err := os.MkdirAll(adminSnapshotRoot, 0700); err != nil {
		return "", 0, err
	}

	processes := readProcesses()
	services := readServices()
	ports := readPorts()
	integrations := make([]adminIntegrationInfo, 0, len(adminIntegrationDefinitions))
	for _, definition := range adminIntegrationDefinitions {
		integrations = append(integrations, detectAdminIntegration(definition, processes, services, ports))
	}
	document := adminSnapshotDocument{
		SchemaVersion: 1,
		CreatedAt:     time.Now().UTC(),
		Summary:       readSummary(),
		Processes:     processes,
		Services:      services,
		Ports:         ports,
		Storage:       readStorage(),
		Thermal:       readThermals(),
		Integrations:  integrations,
	}

	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return "", 0, err
	}
	content = append(content, '\n')
	if len(content) > adminSnapshotMaxBytes {
		return "", 0, fmt.Errorf("snapshot exceeds %d bytes", adminSnapshotMaxBytes)
	}

	name := fmt.Sprintf("routerforge-snapshot-%d.json", time.Now().UnixNano())
	path := filepath.Join(adminSnapshotRoot, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", 0, err
	}
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(content); err != nil {
		return "", 0, err
	}
	if err := file.Sync(); err != nil {
		return "", 0, err
	}
	if err := file.Close(); err != nil {
		return "", 0, err
	}
	success = true

	if err := pruneAdminSnapshots(); err != nil {
		return path, int64(len(content)), err
	}
	return path, int64(len(content)), nil
}

func listAdminSnapshots() ([]adminSnapshotInfo, error) {
	entries, err := os.ReadDir(adminSnapshotRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []adminSnapshotInfo{}, nil
		}
		return nil, err
	}
	result := make([]adminSnapshotInfo, 0)
	for _, entry := range entries {
		path := filepath.Join(adminSnapshotRoot, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !validAdminSnapshotName(entry.Name()) {
			continue
		}
		result = append(result, adminSnapshotInfo{
			Path:       path,
			Name:       entry.Name(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ModifiedAt.After(result[j].ModifiedAt)
	})
	return result, nil
}

func pruneAdminSnapshots() error {
	snapshots, err := listAdminSnapshots()
	if err != nil {
		return err
	}
	for len(snapshots) > adminSnapshotMaxFiles {
		oldest := snapshots[len(snapshots)-1]
		if err := os.Remove(oldest.Path); err != nil {
			return err
		}
		snapshots = snapshots[:len(snapshots)-1]
	}
	return nil
}

func resolveAdminSnapshotPath(path string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(clean) || filepath.Dir(clean) != filepath.Clean(adminSnapshotRoot) {
		return "", errors.New("snapshot must be directly inside RouterForge snapshot directory")
	}
	if !validAdminSnapshotName(filepath.Base(clean)) {
		return "", errors.New("invalid RouterForge snapshot name")
	}
	info, err := os.Lstat(clean)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("snapshot must be a regular non-symlink file")
	}
	return clean, nil
}

func validAdminSnapshotName(name string) bool {
	return strings.HasPrefix(name, "routerforge-snapshot-") &&
		strings.HasSuffix(name, ".json") &&
		!strings.ContainsAny(name, `/\`)
}
