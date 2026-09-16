package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Fifth-Ace/routerforge/internal/platform/configvault"
)

const (
	adminConfigVaultRoot              = "/opt/var/lib/routerforge/config-vault"
	adminConfigVaultManagedRoot       = "/opt/etc/routerforge"
	adminConfigVaultMaxFiles          = 128
	adminConfigVaultReasonMaxLen      = 256
	adminConfigVaultTransactionMaxLen = 128
)

type adminConfigVaultCaptureRequest struct {
	Reason        string `json:"reason"`
	TransactionID string `json:"transaction_id"`
	Confirm       string `json:"confirm"`
}

type adminConfigVaultLastWorkingRequest struct {
	SnapshotID string `json:"snapshot_id"`
	Confirm    string `json:"confirm"`
}

type adminConfigVaultManagedArtifact struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Relative string `json:"relative"`
	Size     int64  `json:"size"`
}

type adminConfigVaultDiffItem struct {
	ID         string `json:"id"`
	SourcePath string `json:"source_path"`
	Status     string `json:"status"`
	Snapshot   string `json:"snapshot_sha256,omitempty"`
	Current    string `json:"current_sha256,omitempty"`
}

type adminConfigVaultDiff struct {
	SnapshotID string                     `json:"snapshot_id"`
	Component  string                     `json:"component"`
	Changed    bool                       `json:"changed"`
	Items      []adminConfigVaultDiffItem `json:"items"`
}

func registerAdminConfigVaultRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/maintenance/config-vault", getOnly(handleAdminConfigVaultOverview))
	mux.HandleFunc("/v1/maintenance/config-vault/", getOnly(handleAdminConfigVaultGet))
	mux.HandleFunc("/v1/maintenance/config-vault-diff", getOnly(handleAdminConfigVaultDiff))
	mux.HandleFunc("/v1/maintenance/config-vault-restore-preview", getOnly(handleAdminConfigVaultRestorePreview))
	mux.HandleFunc("/v1/maintenance/config-vault-capture", mutationOnly(handleAdminConfigVaultCapture))
	mux.HandleFunc("/v1/maintenance/config-vault-last-working", mutationOnly(handleAdminConfigVaultLastWorking))
	mux.HandleFunc("/v1/maintenance/config-vault-restore", mutationOnly(handleAdminConfigVaultRestore))
}

func newAdminConfigVault() (*configvault.Store, error) {
	return configvault.New(configvault.Options{
		Root:         adminConfigVaultRoot,
		AllowedRoots: []string{adminConfigVaultManagedRoot},
	})
}

func discoverAdminConfigVaultArtifacts() ([]adminConfigVaultManagedArtifact, []configvault.ArtifactSpec, error) {
	info, err := os.Lstat(adminConfigVaultManagedRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []adminConfigVaultManagedArtifact{}, []configvault.ArtifactSpec{}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errors.New("RouterForge config root must be a regular directory")
	}

	managed := make([]adminConfigVaultManagedArtifact, 0)
	specs := make([]configvault.ArtifactSpec, 0)

	err = filepath.Walk(adminConfigVaultManagedRoot, func(path string, entry os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == adminConfigVaultManagedRoot {
			return nil
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Mode().IsRegular() {
			return nil
		}
		if len(managed) >= adminConfigVaultMaxFiles {
			return fmt.Errorf("managed config file count exceeds %d", adminConfigVaultMaxFiles)
		}

		relative, err := filepath.Rel(adminConfigVaultManagedRoot, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		id := adminConfigVaultArtifactID(relative)

		managed = append(managed, adminConfigVaultManagedArtifact{
			ID:       id,
			Path:     path,
			Relative: relative,
			Size:     entry.Size(),
		})
		specs = append(specs, configvault.ArtifactSpec{ID: id, Path: path})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	sort.Slice(managed, func(i, j int) bool { return managed[i].Relative < managed[j].Relative })
	sort.Slice(specs, func(i, j int) bool { return specs[i].Path < specs[j].Path })
	return managed, specs, nil
}

func adminConfigVaultArtifactID(relative string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(relative)))
	return "file-" + hex.EncodeToString(sum[:8])
}

func handleAdminConfigVaultOverview(w http.ResponseWriter, _ *http.Request) {
	store, err := newAdminConfigVault()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	snapshots, err := store.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	state, err := store.State()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	managed, _, err := discoverAdminConfigVaultArtifacts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"root":              store.Root(),
		"managed_root":      adminConfigVaultManagedRoot,
		"managed_artifacts": managed,
		"snapshots":         snapshots,
		"state":             state,
		"retention":         store.Retention(),
		"restore_enabled":   true,
		"restore_preview":   true,
	})
}

func handleAdminConfigVaultGet(w http.ResponseWriter, r *http.Request) {
	const prefix = "/v1/maintenance/config-vault/"
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, prefix))
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	store, err := newAdminConfigVault()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	manifest, err := store.Get(id)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "snapshot not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, manifest)
}

func handleAdminConfigVaultCapture(w http.ResponseWriter, r *http.Request) {
	var request adminConfigVaultCaptureRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid config vault capture request"})
		return
	}
	if request.Confirm != "SNAPSHOT" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal SNAPSHOT"})
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	request.TransactionID = strings.TrimSpace(request.TransactionID)
	if len(request.Reason) > adminConfigVaultReasonMaxLen {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "reason is too long"})
		return
	}
	if len(request.TransactionID) > adminConfigVaultTransactionMaxLen {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "transaction_id is too long"})
		return
	}

	managed, specs, err := discoverAdminConfigVaultArtifacts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if len(specs) == 0 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "no managed RouterForge config files found"})
		return
	}

	store, err := newAdminConfigVault()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	manifest, err := store.Capture(configvault.CaptureRequest{
		Component:     "admin",
		Reason:        request.Reason,
		TransactionID: request.TransactionID,
		Artifacts:     specs,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"snapshot":          manifest,
		"managed_artifacts": len(managed),
	})
}

func handleAdminConfigVaultLastWorking(w http.ResponseWriter, r *http.Request) {
	var request adminConfigVaultLastWorkingRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid LAST WORKING request"})
		return
	}
	if request.Confirm != "LAST_WORKING" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal LAST_WORKING"})
		return
	}
	store, err := newAdminConfigVault()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if err := store.MarkLastWorking(strings.TrimSpace(request.SnapshotID)); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	state, _ := store.State()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "state": state})
}

func handleAdminConfigVaultDiff(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	diff, err := buildAdminConfigVaultDiff(id)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, diff)
}

func handleAdminConfigVaultRestorePreview(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	diff, err := buildAdminConfigVaultDiff(id)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"snapshot_id":      diff.SnapshotID,
		"component":        diff.Component,
		"would_change":     diff.Changed,
		"items":            diff.Items,
		"apply_enabled":    true,
		"restore_endpoint": "/v1/maintenance/config-vault-restore",
	})
}

func buildAdminConfigVaultDiff(id string) (adminConfigVaultDiff, error) {
	store, err := newAdminConfigVault()
	if err != nil {
		return adminConfigVaultDiff{}, err
	}
	manifest, err := store.Get(id)
	if err != nil {
		return adminConfigVaultDiff{}, err
	}

	current, _, err := discoverAdminConfigVaultArtifacts()
	if err != nil {
		return adminConfigVaultDiff{}, err
	}
	currentByPath := make(map[string]adminConfigVaultManagedArtifact, len(current))
	for _, item := range current {
		currentByPath[item.Path] = item
	}

	items := make([]adminConfigVaultDiffItem, 0, len(manifest.Artifacts)+len(current))
	seen := make(map[string]struct{}, len(manifest.Artifacts))
	changed := false

	for _, snapshotArtifact := range manifest.Artifacts {
		seen[snapshotArtifact.SourcePath] = struct{}{}
		currentArtifact, exists := currentByPath[snapshotArtifact.SourcePath]
		if !exists {
			changed = true
			items = append(items, adminConfigVaultDiffItem{
				ID:         snapshotArtifact.ID,
				SourcePath: snapshotArtifact.SourcePath,
				Status:     "removed",
				Snapshot:   snapshotArtifact.SHA256,
			})
			continue
		}
		checksum, err := sha256RegularFile(currentArtifact.Path)
		if err != nil {
			return adminConfigVaultDiff{}, err
		}
		status := "unchanged"
		if checksum != snapshotArtifact.SHA256 {
			status = "changed"
			changed = true
		}
		items = append(items, adminConfigVaultDiffItem{
			ID:         snapshotArtifact.ID,
			SourcePath: snapshotArtifact.SourcePath,
			Status:     status,
			Snapshot:   snapshotArtifact.SHA256,
			Current:    checksum,
		})
	}

	for _, currentArtifact := range current {
		if _, exists := seen[currentArtifact.Path]; exists {
			continue
		}
		checksum, err := sha256RegularFile(currentArtifact.Path)
		if err != nil {
			return adminConfigVaultDiff{}, err
		}
		changed = true
		items = append(items, adminConfigVaultDiffItem{
			ID:         currentArtifact.ID,
			SourcePath: currentArtifact.Path,
			Status:     "added",
			Current:    checksum,
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].SourcePath < items[j].SourcePath })
	return adminConfigVaultDiff{
		SnapshotID: manifest.ID,
		Component:  manifest.Component,
		Changed:    changed,
		Items:      items,
	}, nil
}

func sha256RegularFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("managed config path is not a regular non-symlink file")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}
