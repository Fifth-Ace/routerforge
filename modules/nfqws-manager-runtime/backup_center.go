package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	persistentBackupRoot = "/opt/var/lib/routerforge/nfqws-manager/backups"
	persistentBackupMax  = 64
)

type persistentBackupManifest struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Kind    string `json:"kind"`
	Target  string `json:"target,omitempty"`
	Created string `json:"created_at"`
	Size    int64  `json:"size"`
	Mode    uint32 `json:"mode"`
	SHA256  string `json:"sha256"`
}

type backupCenterEntry struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Kind          string `json:"kind"`
	Target        string `json:"target,omitempty"`
	Created       string `json:"created_at"`
	Size          int64  `json:"size"`
	Mode          uint32 `json:"mode"`
	SHA256        string `json:"sha256"`
	Restorable    bool   `json:"restorable"`
	CurrentExists bool   `json:"current_exists"`
	CurrentSHA256 string `json:"current_sha256,omitempty"`
	State         string `json:"state"`
}

type backupRestoreRequest struct {
	ID      string `json:"id"`
	Confirm string `json:"confirm"`
}

type backupDeleteRequest struct {
	ID      string `json:"id"`
	Confirm string `json:"confirm"`
}

type backupPruneRequest struct {
	Confirm string `json:"confirm"`
}

func registerBackupCenterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/backups", getOnly(handleBackupCenterList))
	mux.HandleFunc("/v1/backups/read", getOnly(handleBackupCenterRead))
	mux.HandleFunc("/v1/backups/restore", mutationOnly(handleBackupCenterRestore))
	mux.HandleFunc("/v1/backups/delete", mutationOnly(handleBackupCenterDelete))
	mux.HandleFunc("/v1/backups/prune", mutationOnly(handleBackupCenterPrune))
}

func safeBackupID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || filepath.Base(id) != id || strings.HasPrefix(id, ".") {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func backupKindTarget(label string) (string, string) {
	label = strings.TrimSpace(label)
	switch {
	case label == "config", label == "smart-apply-config", strings.HasPrefix(label, "config-switch-"):
		return "config", configPath
	case strings.HasPrefix(label, "smart-apply-list-"):
		name := strings.TrimPrefix(label, "smart-apply-list-")
		if safeListSourceName(name) {
			return "list", filepath.Join(listsRoot, name)
		}
	case strings.HasPrefix(label, "list-source-"):
		name := strings.TrimPrefix(label, "list-source-")
		if safeListSourceName(name) {
			return "list", filepath.Join(listsRoot, name)
		}
	case strings.HasPrefix(label, "deleted-"):
		name := strings.TrimPrefix(label, "deleted-")
		if editableListName(name) {
			return "list", filepath.Join(listsRoot, name)
		}
	case strings.HasPrefix(label, "list-"):
		name := strings.TrimPrefix(label, "list-")
		if editableListName(name) {
			return "list", filepath.Join(listsRoot, name)
		}
	case strings.HasPrefix(label, "smart-apply-blob-"):
		name := strings.TrimPrefix(label, "smart-apply-blob-")
		if safeBlobName(name) {
			return "blob", filepath.Join(blobRoot, name)
		}
	case strings.HasPrefix(label, "blob-delete-"):
		name := strings.TrimPrefix(label, "blob-delete-")
		if safeBlobName(name) {
			return "blob", filepath.Join(blobRoot, name)
		}
	case strings.HasPrefix(label, "blob-"):
		name := strings.TrimPrefix(label, "blob-")
		if safeBlobName(name) {
			return "blob", filepath.Join(blobRoot, name)
		}
	case strings.HasPrefix(label, "library-delete-"):
		name := strings.TrimPrefix(label, "library-delete-")
		if safeConfigLibraryName(name) {
			return "library", filepath.Join(configLibraryRoot, name)
		}
	case strings.HasPrefix(label, "library-"):
		name := strings.TrimPrefix(label, "library-")
		if safeConfigLibraryName(name) {
			return "library", filepath.Join(configLibraryRoot, name)
		}
	}
	return "unknown", ""
}

func ensurePersistentBackupRoot() error {
	if err := os.MkdirAll(persistentBackupRoot, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(persistentBackupRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("persistent backup root is not a real directory")
	}
	return nil
}

func persistentBackupID(label string, data []byte) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, label)
	if len(safe) > 48 {
		safe = safe[:48]
	}
	return strconv.FormatInt(time.Now().UTC().UnixNano(), 10) + "-" + safe + "-" + blobSHA256(data)[:10]
}

func recordPersistentBackup(label string, data []byte, mode os.FileMode) (string, error) {
	kind, target := backupKindTarget(label)
	return recordPersistentBackupTarget(label, kind, target, data, mode)
}

func recordPersistentBackupTarget(label, kind, target string, data []byte, mode os.FileMode) (string, error) {
	if err := ensurePersistentBackupRoot(); err != nil {
		return "", err
	}
	if len(data) > listFileMaxBytes {
		return "", fmt.Errorf("persistent backup payload exceeds %d bytes", listFileMaxBytes)
	}
	id := persistentBackupID(label, data)
	if !safeBackupID(id) {
		return "", errors.New("generated persistent backup id is unsafe")
	}
	dir := filepath.Join(persistentBackupRoot, id)
	if err := os.Mkdir(dir, 0700); err != nil {
		return "", err
	}
	cleanup := func() {
		_ = os.Remove(filepath.Join(dir, "payload.bin"))
		_ = os.Remove(filepath.Join(dir, "manifest.json"))
		_ = os.Remove(dir)
	}
	if err := safety.WriteFileAtomic(filepath.Join(dir, "payload.bin"), data, 0600); err != nil {
		cleanup()
		return "", err
	}
	manifest := persistentBackupManifest{
		ID: id, Label: label, Kind: kind, Target: target,
		Created: time.Now().UTC().Format(time.RFC3339Nano),
		Size:    int64(len(data)), Mode: uint32(mode.Perm()), SHA256: blobSHA256(data),
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		cleanup()
		return "", err
	}
	if err := safety.WriteFileAtomic(filepath.Join(dir, "manifest.json"), raw, 0600); err != nil {
		cleanup()
		return "", err
	}
	_ = prunePersistentBackups()
	return id, nil
}

func persistentBackupDir(id string) (string, error) {
	id = strings.TrimSpace(id)
	if !safeBackupID(id) {
		return "", errors.New("invalid backup id")
	}
	dir := filepath.Join(persistentBackupRoot, id)
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("backup entry is not a real directory")
	}
	return dir, nil
}

func readPersistentBackup(id string) (persistentBackupManifest, []byte, error) {
	var manifest persistentBackupManifest
	dir, err := persistentBackupDir(id)
	if err != nil {
		return manifest, nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return manifest, nil, err
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, nil, err
	}
	if manifest.ID != id || !safeBackupID(manifest.ID) {
		return manifest, nil, errors.New("backup manifest id mismatch")
	}
	payload, err := os.ReadFile(filepath.Join(dir, "payload.bin"))
	if err != nil {
		return manifest, nil, err
	}
	if int64(len(payload)) != manifest.Size || !strings.EqualFold(blobSHA256(payload), manifest.SHA256) {
		return manifest, nil, errors.New("backup payload integrity check failed")
	}
	return manifest, payload, nil
}

func validRestoreTarget(kind, target string) bool {
	clean := filepath.Clean(target)
	switch kind {
	case "config":
		return clean == filepath.Clean(configPath)
	case "list":
		return filepath.Dir(clean) == filepath.Clean(listsRoot) && editableListName(filepath.Base(clean))
	case "blob":
		return filepath.Dir(clean) == filepath.Clean(blobRoot) && safeBlobName(filepath.Base(clean))
	case "library":
		return filepath.Dir(clean) == filepath.Clean(configLibraryRoot) && safeConfigLibraryName(filepath.Base(clean))
	default:
		return false
	}
}

func backupCurrentHash(target string, limit int64) (bool, string) {
	info, err := os.Lstat(target)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > limit {
		return false, ""
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return false, ""
	}
	return true, blobSHA256(data)
}

func backupPayloadLimit(kind string) int64 {
	switch kind {
	case "blob":
		return blobMaxBytes
	case "config", "library":
		return configMaxBytes
	default:
		return listFileMaxBytes
	}
}

func backupEntry(manifest persistentBackupManifest) backupCenterEntry {
	entry := backupCenterEntry{
		ID: manifest.ID, Label: manifest.Label, Kind: manifest.Kind, Target: manifest.Target,
		Created: manifest.Created, Size: manifest.Size, Mode: manifest.Mode, SHA256: manifest.SHA256,
		Restorable: validRestoreTarget(manifest.Kind, manifest.Target), State: "UNBOUND",
	}
	if !entry.Restorable {
		return entry
	}
	exists, current := backupCurrentHash(manifest.Target, backupPayloadLimit(manifest.Kind))
	entry.CurrentExists = exists
	entry.CurrentSHA256 = current
	if !exists {
		entry.State = "MISSING"
	} else if strings.EqualFold(current, manifest.SHA256) {
		entry.State = "CURRENT"
	} else {
		entry.State = "CHANGED"
	}
	return entry
}

func readBackupCenterInventory() []backupCenterEntry {
	entries, err := os.ReadDir(persistentBackupRoot)
	if err != nil {
		return []backupCenterEntry{}
	}
	out := []backupCenterEntry{}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !safeBackupID(entry.Name()) {
			continue
		}
		manifest, _, err := readPersistentBackup(entry.Name())
		if err != nil {
			continue
		}
		out = append(out, backupEntry(manifest))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Created > out[j].Created
	})
	return out
}

func prunePersistentBackups() error {
	entries, err := os.ReadDir(persistentBackupRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	type item struct {
		id  string
		mod time.Time
	}
	items := []item{}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !safeBackupID(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err == nil {
			items = append(items, item{id: entry.Name(), mod: info.ModTime()})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod.After(items[j].mod) })
	for i := persistentBackupMax; i < len(items); i++ {
		dir := filepath.Join(persistentBackupRoot, items[i].id)
		_ = os.Remove(filepath.Join(dir, "payload.bin"))
		_ = os.Remove(filepath.Join(dir, "manifest.json"))
		if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func backupCenterResponse() map[string]any {
	files := readBackupCenterInventory()
	return map[string]any{
		"root":         persistentBackupRoot,
		"files":        files,
		"count":        len(files),
		"max":          persistentBackupMax,
		"safety_count": countBackups(),
	}
}

func handleBackupCenterList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, backupCenterResponse())
}

func textBackupKind(kind string) bool {
	return kind == "config" || kind == "list" || kind == "library"
}

func handleBackupCenterRead(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	manifest, payload, err := readPersistentBackup(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "backup not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "read backup: " + err.Error()})
		return
	}
	entry := backupEntry(manifest)
	response := map[string]any{"entry": entry}
	if textBackupKind(manifest.Kind) {
		response["content"] = string(payload)
		if entry.CurrentExists {
			if current, cut, err := readBoundedFile(manifest.Target, backupPayloadLimit(manifest.Kind)); err == nil && !cut {
				response["current_content"] = string(current)
			}
		}
	} else {
		response["content_base64"] = base64.StdEncoding.EncodeToString(payload)
	}
	writeJSON(w, http.StatusOK, response)
}

func rollbackRestore(target string, before []byte, beforeMode os.FileMode, existed bool, wasRunning bool) (bool, string) {
	if existed {
		if err := safety.WriteFileAtomic(target, before, beforeMode); err != nil {
			return false, err.Error()
		}
	} else {
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err.Error()
		}
	}
	if target == configPath && wasRunning {
		if output, err := runInit("restart"); err != nil {
			return false, output + " " + err.Error()
		}
	}
	return true, ""
}

func handleBackupCenterRestore(w http.ResponseWriter, r *http.Request) {
	var request backupRestoreRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid backup restore request"})
		return
	}
	if request.Confirm != "NFQWS_BACKUP_RESTORE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_BACKUP_RESTORE"})
		return
	}
	manifest, payload, err := readPersistentBackup(strings.TrimSpace(request.ID))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "read backup: " + err.Error()})
		return
	}
	if !validRestoreTarget(manifest.Kind, manifest.Target) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "backup target is not restorable"})
		return
	}
	if int64(len(payload)) > backupPayloadLimit(manifest.Kind) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "backup exceeds current safety limit"})
		return
	}
	if manifest.Kind == "config" {
		if err := validateConfig(string(payload)); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "backup config is invalid: " + err.Error()})
			return
		}
		if !readStatus().MutationReady {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "nfqws2 runtime is not ready for config restore"})
			return
		}
	}
	if manifest.Kind == "list" {
		if err := validateListContent(string(payload)); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "backup list is invalid: " + err.Error()})
			return
		}
	}

	target := manifest.Target
	info, statErr := os.Lstat(target)
	existed := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat restore target: " + statErr.Error()})
		return
	}
	if existed && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "restore target is not a regular file"})
		return
	}
	before := []byte{}
	beforeMode := os.FileMode(manifest.Mode)
	if beforeMode == 0 {
		beforeMode = 0644
	}
	if existed {
		before, err = os.ReadFile(target)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read restore target: " + err.Error()})
			return
		}
		beforeMode = info.Mode().Perm()
		if _, err := recordPersistentBackupTarget("restore-before-"+manifest.Kind+"-"+filepath.Base(target), manifest.Kind, target, before, beforeMode); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create pre-restore persistent backup: " + err.Error()})
			return
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "prepare restore target directory: " + err.Error()})
		return
	}

	status := readStatus()
	if err := safety.WriteFileAtomic(target, payload, beforeMode); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "restore backup payload: " + err.Error()})
		return
	}

	restartOutput := ""
	if manifest.Kind == "config" && status.Running {
		restartOutput, err = runInit("restart")
		if err != nil {
			rolledBack, rollbackErr := rollbackRestore(target, before, beforeMode, existed, true)
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"error": err.Error(), "output": restartOutput,
				"rolled_back": rolledBack, "rollback_error": rollbackErr,
			})
			return
		}
	}

	exists, currentHash := backupCurrentHash(target, backupPayloadLimit(manifest.Kind))
	if !exists || !strings.EqualFold(currentHash, manifest.SHA256) {
		rolledBack, rollbackErr := rollbackRestore(target, before, beforeMode, existed, manifest.Kind == "config" && status.Running)
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":       "restored backup verification failed",
			"rolled_back": rolledBack, "rollback_error": rollbackErr,
		})
		return
	}
	if manifest.Kind == "config" && status.Running && !readStatus().Running {
		rolledBack, rollbackErr := rollbackRestore(target, before, beforeMode, existed, true)
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":       "nfqws2 is not running after config restore",
			"rolled_back": rolledBack, "rollback_error": rollbackErr,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "id": manifest.ID, "kind": manifest.Kind, "target": target,
		"sha256": manifest.SHA256, "service_restarted": manifest.Kind == "config" && status.Running,
		"restart_output": restartOutput, "status": readStatus(), "backups": backupCenterResponse(),
	})
}

func handleBackupCenterPrune(w http.ResponseWriter, r *http.Request) {
	var request backupPruneRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid backup prune request"})
		return
	}
	if request.Confirm != "NFQWS_BACKUP_PRUNE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_BACKUP_PRUNE"})
		return
	}
	beforeSafety := countBackups()
	beforePersistent := len(readBackupCenterInventory())
	if err := pruneBackups(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "prune safety backups: " + err.Error()})
		return
	}
	if err := prunePersistentBackups(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "prune persistent backups: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"safety_before": beforeSafety, "safety_after": countBackups(),
		"persistent_before": beforePersistent, "persistent_after": len(readBackupCenterInventory()),
		"production_mutation": false, "runtime_restarted": false,
	})
}

func handleBackupCenterDelete(w http.ResponseWriter, r *http.Request) {
	var request backupDeleteRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid backup delete request"})
		return
	}
	if request.Confirm != "NFQWS_BACKUP_DELETE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_BACKUP_DELETE"})
		return
	}
	dir, err := persistentBackupDir(strings.TrimSpace(request.ID))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "backup not found"})
		return
	}
	if err := os.Remove(filepath.Join(dir, "payload.bin")); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete backup payload: " + err.Error()})
		return
	}
	if err := os.Remove(filepath.Join(dir, "manifest.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete backup manifest: " + err.Error()})
		return
	}
	if err := os.Remove(dir); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete backup directory: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "backups": backupCenterResponse()})
}
