package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

var listSourceHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

type listSourceFile struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Size       int64    `json:"size"`
	Modified   string   `json:"modified_at,omitempty"`
	SHA256     string   `json:"sha256"`
	Protected  bool     `json:"protected"`
	Referenced bool     `json:"referenced"`
	References []string `json:"references"`
}

type listSourceInstallRequest struct {
	Name           string `json:"name"`
	ContentBase64  string `json:"content_base64"`
	ExpectedSHA256 string `json:"expected_sha256"`
	Replace        bool   `json:"replace,omitempty"`
	Confirm        string `json:"confirm"`
}

func registerListSourceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/list-sources", getOnly(handleListSourceList))
	mux.HandleFunc("/v1/list-sources/install", mutationOnly(handleListSourceInstall))
}

func safeListSourceName(name string) bool {
	name = strings.TrimSpace(name)
	if !safeListName(name) {
		return false
	}
	return strings.HasSuffix(strings.ToLower(name), ".list")
}

func listSourcePath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !safeListSourceName(name) {
		return "", errors.New("list source filename must end with .list and contain only safe characters")
	}
	return filepath.Join(listsRoot, name), nil
}

func ensureListSourceRoot() error {
	if err := os.MkdirAll(listsRoot, 0755); err != nil {
		return err
	}
	info, err := os.Lstat(listsRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("lists root is not a real directory")
	}
	return nil
}

func regularListSourceFile(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("list source is not a regular file")
	}
	if info.Size() > listFileMaxBytes {
		return nil, fmt.Errorf("list source exceeds %d bytes", listFileMaxBytes)
	}
	return info, nil
}

func listSourceSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validateListSourcePayload(name string, data []byte, expected string) error {
	if !safeListSourceName(name) {
		return errors.New("invalid list source filename")
	}
	if len(data) == 0 {
		return errors.New("list source payload must not be empty")
	}
	if len(data) > listFileMaxBytes {
		return fmt.Errorf("list source payload exceeds %d bytes", listFileMaxBytes)
	}
	if err := validateListContent(string(data)); err != nil {
		return err
	}
	expected = strings.TrimSpace(expected)
	if !listSourceHashPattern.MatchString(expected) {
		return errors.New("expected_sha256 must be a 64-character hexadecimal SHA256")
	}
	if !strings.EqualFold(listSourceSHA256(data), expected) {
		return errors.New("list source payload SHA256 does not match expected_sha256")
	}
	return nil
}

func listSourceReferences(name string) []string {
	if !safeListSourceName(name) {
		return []string{}
	}
	refs := []string{}
	contains := func(path, label string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		if strings.Contains(string(data), name) {
			refs = append(refs, label)
		}
	}
	contains(configPath, "active:nfqws2.conf")
	entries, err := os.ReadDir(configLibraryRoot)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !safeConfigLibraryName(entry.Name()) {
				continue
			}
			contains(filepath.Join(configLibraryRoot, entry.Name()), "library:"+entry.Name())
		}
	}
	sort.Strings(refs)
	return refs
}

func readListSourceInventory() []listSourceFile {
	entries, err := os.ReadDir(listsRoot)
	if err != nil {
		return []listSourceFile{}
	}
	out := make([]listSourceFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !safeListSourceName(entry.Name()) {
			continue
		}
		path := filepath.Join(listsRoot, entry.Name())
		info, err := regularListSourceFile(path)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		refs := listSourceReferences(entry.Name())
		out = append(out, listSourceFile{
			Name: entry.Name(), Path: path, Size: info.Size(),
			Modified: info.ModTime().UTC().Format(time.RFC3339),
			SHA256:   listSourceSHA256(data), Protected: protectedListName(entry.Name()),
			Referenced: len(refs) > 0, References: refs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func listSourceInventoryResponse() map[string]any {
	files := readListSourceInventory()
	return map[string]any{
		"root": listsRoot, "files": files, "count": len(files),
	}
}

func handleListSourceList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, listSourceInventoryResponse())
}

func handleListSourceInstall(w http.ResponseWriter, r *http.Request) {
	var request listSourceInstallRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid list source install request"})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	expectedConfirm := "NFQWS_LIST_SOURCE_INSTALL"
	if request.Replace {
		expectedConfirm = "NFQWS_LIST_SOURCE_REPLACE"
	}
	if request.Confirm != expectedConfirm {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "invalid list source install confirmation"})
		return
	}
	path, err := listSourcePath(request.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	data, err := base64.StdEncoding.DecodeString(request.ContentBase64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "content_base64 is invalid"})
		return
	}
	if err := validateListSourcePayload(request.Name, data, request.ExpectedSHA256); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if err := ensureListSourceRoot(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "prepare lists root: " + err.Error()})
		return
	}

	mode := os.FileMode(0644)
	backup := ""
	replaced := false
	references := []string{}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "existing list source path is not a regular file"})
			return
		}
		if info.Size() > listFileMaxBytes {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "existing list source exceeds manager safety limit"})
			return
		}
		if !request.Replace {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "list source already exists; explicit replace is required"})
			return
		}
		before, err := os.ReadFile(path)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read existing list source: " + err.Error()})
			return
		}
		if listSourceSHA256(before) == listSourceSHA256(data) {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": true, "name": request.Name, "already_current": true,
				"sha256": listSourceSHA256(data), "runtime_restarted": false,
				"inventory": listSourceInventoryResponse(), "status": readStatus(),
			})
			return
		}
		mode = info.Mode().Perm()
		references = listSourceReferences(request.Name)
		backup, err = createNamedBackup("list-source-"+request.Name, before, mode)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create list source backup: " + err.Error()})
			return
		}
		replaced = true
	} else if !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat list source: " + err.Error()})
		return
	}

	if err := safety.WriteFileAtomic(path, data, mode); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "install list source: " + err.Error(), "backup": backup})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "name": request.Name, "sha256": listSourceSHA256(data),
		"replaced": replaced, "backup": backup, "references": references,
		"protected": protectedListName(request.Name), "runtime_restarted": false,
		"inventory": listSourceInventoryResponse(), "status": readStatus(),
	})
}
