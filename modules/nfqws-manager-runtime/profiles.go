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
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const configLibraryRoot = "/opt/etc/nfqws2/routerforge-configs"

type configLibraryFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified string `json:"modified_at,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
	Active   bool   `json:"active"`
	Empty    bool   `json:"empty"`
}

type configLibraryRequest struct {
	Name        string `json:"name"`
	NewName     string `json:"new_name,omitempty"`
	Content     string `json:"content,omitempty"`
	FromCurrent bool   `json:"from_current,omitempty"`
	Confirm     string `json:"confirm"`
}

func registerConfigLibraryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/configs", getOnly(handleConfigLibraryList))
	mux.HandleFunc("/v1/configs/read", getOnly(handleConfigLibraryRead))
	mux.HandleFunc("/v1/configs/create", mutationOnly(handleConfigLibraryCreate))
	mux.HandleFunc("/v1/configs/save", mutationOnly(handleConfigLibrarySave))
	mux.HandleFunc("/v1/configs/rename", mutationOnly(handleConfigLibraryRename))
	mux.HandleFunc("/v1/configs/duplicate", mutationOnly(handleConfigLibraryDuplicate))
	mux.HandleFunc("/v1/configs/delete", mutationOnly(handleConfigLibraryDelete))
	mux.HandleFunc("/v1/configs/activate", mutationOnly(handleConfigLibraryActivate))
}

func safeConfigLibraryName(name string) bool {
	if name == "" || filepath.Base(name) != name || strings.HasPrefix(name, ".") {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(name), ".conf") {
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

func validateLibraryConfig(content string) error {
	if len(content) > configMaxBytes {
		return fmt.Errorf("nfqws2 config exceeds %d bytes", configMaxBytes)
	}
	if strings.IndexByte(content, 0) >= 0 {
		return errors.New("nfqws2 config contains NUL byte")
	}
	for i, line := range strings.Split(content, "\n") {
		if len(line) > 8192 {
			return fmt.Errorf("nfqws2 config line %d exceeds 8192 bytes", i+1)
		}
	}
	return nil
}

func configLibraryPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !safeConfigLibraryName(name) {
		return "", errors.New("config filename must end with .conf and contain only letters, digits, dot, _ or -")
	}
	return filepath.Join(configLibraryRoot, name), nil
}

func configHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readConfigLibraryFiles() []configLibraryFile {
	active := ""
	if data, err := os.ReadFile(configPath); err == nil {
		active = configHash(data)
	}
	entries, err := os.ReadDir(configLibraryRoot)
	if err != nil {
		return []configLibraryFile{}
	}
	out := make([]configLibraryFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !safeConfigLibraryName(entry.Name()) {
			continue
		}
		path := filepath.Join(configLibraryRoot, entry.Name())
		info, err := entry.Info()
		if err != nil || info.Size() > configMaxBytes {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		hash := configHash(data)
		out = append(out, configLibraryFile{
			Name: entry.Name(), Path: path, Size: info.Size(),
			Modified: info.ModTime().UTC().Format(time.RFC3339),
			SHA256:   hash, Active: active != "" && hash == active, Empty: len(data) == 0,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func configLibraryResponse() map[string]any {
	files := readConfigLibraryFiles()
	return map[string]any{
		"root":   configLibraryRoot,
		"files":  files,
		"count":  len(files),
		"status": readStatus(),
	}
}

func handleConfigLibraryList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, configLibraryResponse())
}

func handleConfigLibraryRead(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	path, err := configLibraryPath(name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if _, err := configLibraryRegularFile(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "stored config not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	data, cut, err := readBoundedFile(path, configMaxBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "stored config not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read stored config: " + err.Error()})
		return
	}
	if cut {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "stored config exceeds editor limit"})
		return
	}
	info, _ := os.Stat(path)
	activeData, _ := os.ReadFile(configPath)
	writeJSON(w, http.StatusOK, map[string]any{
		"name": name, "path": path, "content": string(data), "size": len(data),
		"sha256": configHash(data), "active": len(activeData) > 0 && configHash(activeData) == configHash(data),
		"empty": len(data) == 0,
		"modified_at": func() string {
			if info == nil {
				return ""
			}
			return info.ModTime().UTC().Format(time.RFC3339)
		}(),
	})
}

func ensureConfigLibraryRoot() error {
	if err := os.MkdirAll(configLibraryRoot, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(configLibraryRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("config library root is not a real directory")
	}
	return nil
}

func configLibraryRegularFile(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("stored config is not a regular file")
	}
	return info, nil
}

func handleConfigLibraryCreate(w http.ResponseWriter, r *http.Request) {
	var request configLibraryRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid config library create request"})
		return
	}
	if request.Confirm != "NFQWS_CONFIG_LIBRARY_CREATE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_CONFIG_LIBRARY_CREATE"})
		return
	}
	path, err := configLibraryPath(request.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	content := request.Content
	if request.FromCurrent {
		data, err := os.ReadFile(configPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read current config: " + err.Error()})
			return
		}
		content = string(data)
	}
	if err := validateLibraryConfig(content); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if err := ensureConfigLibraryRoot(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create config library: " + err.Error()})
		return
	}
	if _, err := os.Lstat(path); err == nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "stored config already exists"})
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat stored config: " + err.Error()})
		return
	}
	if err := safety.WriteFileAtomic(path, []byte(content), 0600); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create stored config: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": request.Name, "library": configLibraryResponse()})
}

func handleConfigLibrarySave(w http.ResponseWriter, r *http.Request) {
	var request configLibraryRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid config library save request"})
		return
	}
	if request.Confirm != "NFQWS_CONFIG_LIBRARY_SAVE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_CONFIG_LIBRARY_SAVE"})
		return
	}
	path, err := configLibraryPath(request.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if err := validateLibraryConfig(request.Content); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	info, err := configLibraryRegularFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "stored config unavailable: " + err.Error()})
		return
	}
	before, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "read stored config: " + err.Error()})
		return
	}
	backup, err := createNamedBackup("library-"+request.Name, before, info.Mode().Perm())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create stored config backup: " + err.Error()})
		return
	}
	if err := safety.WriteFileAtomic(path, []byte(request.Content), info.Mode().Perm()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "save stored config: " + err.Error(), "backup": backup})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": request.Name, "backup": backup, "library": configLibraryResponse()})
}

func handleConfigLibraryRename(w http.ResponseWriter, r *http.Request) {
	var request configLibraryRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid config library rename request"})
		return
	}
	if request.Confirm != "NFQWS_CONFIG_LIBRARY_RENAME" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_CONFIG_LIBRARY_RENAME"})
		return
	}
	oldPath, err := configLibraryPath(request.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	newPath, err := configLibraryPath(request.NewName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if _, err := configLibraryRegularFile(oldPath); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "stored config unavailable: " + err.Error()})
		return
	}
	if _, err := os.Lstat(newPath); err == nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "destination config already exists"})
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat destination config: " + err.Error()})
		return
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "rename stored config: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": request.NewName, "library": configLibraryResponse()})
}

func handleConfigLibraryDuplicate(w http.ResponseWriter, r *http.Request) {
	var request configLibraryRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid config library duplicate request"})
		return
	}
	if request.Confirm != "NFQWS_CONFIG_LIBRARY_DUPLICATE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_CONFIG_LIBRARY_DUPLICATE"})
		return
	}
	source, err := configLibraryPath(request.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	destination, err := configLibraryPath(request.NewName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if _, err := configLibraryRegularFile(source); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "source config unavailable: " + err.Error()})
		return
	}
	data, err := os.ReadFile(source)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "source config not found"})
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
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "duplicate stored config: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": request.NewName, "library": configLibraryResponse()})
}

func handleConfigLibraryDelete(w http.ResponseWriter, r *http.Request) {
	var request configLibraryRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid config library delete request"})
		return
	}
	if request.Confirm != "NFQWS_CONFIG_LIBRARY_DELETE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_CONFIG_LIBRARY_DELETE"})
		return
	}
	path, err := configLibraryPath(request.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	info, err := configLibraryRegularFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "stored config unavailable: " + err.Error()})
		return
	}
	before, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "stored config not found"})
		return
	}
	backup, err := createNamedBackup("library-delete-"+request.Name, before, info.Mode().Perm())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create delete backup: " + err.Error()})
		return
	}
	if err := os.Remove(path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete stored config: " + err.Error(), "backup": backup})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "backup": backup, "library": configLibraryResponse()})
}

func rollbackCanonicalConfig(before []byte, mode os.FileMode, wasRunning bool) (bool, string, string) {
	if err := safety.WriteFileAtomic(configPath, before, mode); err != nil {
		return false, err.Error(), ""
	}
	if !wasRunning {
		return true, "", ""
	}
	output, err := runInit("restart")
	if err != nil {
		return false, err.Error(), output
	}
	return true, "", output
}

func handleConfigLibraryActivate(w http.ResponseWriter, r *http.Request) {
	var request configLibraryRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid config library activate request"})
		return
	}
	if request.Confirm != "NFQWS_CONFIG_LIBRARY_ACTIVATE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_CONFIG_LIBRARY_ACTIVATE"})
		return
	}
	path, err := configLibraryPath(request.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if _, err := configLibraryRegularFile(path); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "stored config unavailable: " + err.Error()})
		return
	}
	selected, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "stored config not found"})
		return
	}
	if err := validateConfig(string(selected)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "selected config is not activatable: " + err.Error()})
		return
	}
	status := readStatus()
	if !status.MutationReady {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "nfqws2 runtime is not ready for config switch"})
		return
	}
	before, err := os.ReadFile(configPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read current config: " + err.Error()})
		return
	}
	if configHash(before) == configHash(selected) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "already_active": true, "name": request.Name, "status": status, "library": configLibraryResponse()})
		return
	}
	info, err := os.Stat(configPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat current config: " + err.Error()})
		return
	}
	backup, err := createNamedBackup("config-switch-"+request.Name, before, info.Mode().Perm())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create switch backup: " + err.Error()})
		return
	}
	if err := safety.WriteFileAtomic(configPath, selected, info.Mode().Perm()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "activate config: " + err.Error(), "backup": backup})
		return
	}

	output := ""
	if status.Running {
		output, err = runInit("restart")
		if err != nil {
			rolledBack, rollbackErr, rollbackOutput := rollbackCanonicalConfig(before, info.Mode().Perm(), true)
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"error": err.Error(), "output": output, "backup": backup,
				"rolled_back": rolledBack, "rollback_error": rollbackErr, "rollback_output": rollbackOutput,
			})
			return
		}
	}

	selectedHash := configHash(selected)
	after := readStatus()
	for attempt := 0; attempt < 10 && (after.ConfigSHA256 != selectedHash || status.Running && !after.Running); attempt++ {
		time.Sleep(200 * time.Millisecond)
		after = readStatus()
	}
	if after.ConfigSHA256 != selectedHash || status.Running && !after.Running {
		rolledBack, rollbackErr, rollbackOutput := rollbackCanonicalConfig(before, info.Mode().Perm(), status.Running)
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": "activated config verification failed", "backup": backup,
			"rolled_back": rolledBack, "rollback_error": rollbackErr, "rollback_output": rollbackOutput,
		})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "name": request.Name, "backup": backup, "restart_output": output,
		"service_restarted": status.Running, "status": after, "library": configLibraryResponse(),
	})
}
