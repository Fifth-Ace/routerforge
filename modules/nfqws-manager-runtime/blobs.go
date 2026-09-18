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

const (
	blobRoot     = "/opt/etc/nfqws2/blobs"
	blobMaxBytes = 1 << 20
)

var blobHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

type blobFile struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Size       int64    `json:"size"`
	Modified   string   `json:"modified_at,omitempty"`
	SHA256     string   `json:"sha256"`
	Referenced bool     `json:"referenced"`
	References []string `json:"references"`
}

type blobInstallRequest struct {
	Name           string `json:"name"`
	ContentBase64  string `json:"content_base64"`
	ExpectedSHA256 string `json:"expected_sha256"`
	Replace        bool   `json:"replace,omitempty"`
	Confirm        string `json:"confirm"`
}

type blobDeleteRequest struct {
	Name    string `json:"name"`
	Confirm string `json:"confirm"`
}

func registerBlobRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/blobs", getOnly(handleBlobList))
	mux.HandleFunc("/v1/blobs/install", mutationOnly(handleBlobInstall))
	mux.HandleFunc("/v1/blobs/delete", mutationOnly(handleBlobDelete))
}

func safeBlobName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || filepath.Base(name) != name || strings.HasPrefix(name, ".") {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(name), ".bin") {
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

func blobPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !safeBlobName(name) {
		return "", errors.New("blob filename must end with .bin and contain only letters, digits, dot, _ or -")
	}
	return filepath.Join(blobRoot, name), nil
}

func ensureBlobRoot() error {
	if err := os.MkdirAll(blobRoot, 0755); err != nil {
		return err
	}
	info, err := os.Lstat(blobRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("blob root is not a real directory")
	}
	return nil
}

func regularBlobFile(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("blob is not a regular file")
	}
	if info.Size() > blobMaxBytes {
		return nil, fmt.Errorf("blob exceeds %d bytes", blobMaxBytes)
	}
	return info, nil
}

func blobSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validateBlobPayload(name string, data []byte, expected string) error {
	if !safeBlobName(name) {
		return errors.New("invalid blob filename")
	}
	if len(data) == 0 {
		return errors.New("blob payload must not be empty")
	}
	if len(data) > blobMaxBytes {
		return fmt.Errorf("blob payload exceeds %d bytes", blobMaxBytes)
	}
	expected = strings.TrimSpace(expected)
	if !blobHashPattern.MatchString(expected) {
		return errors.New("expected_sha256 must be a 64-character hexadecimal SHA256")
	}
	if !strings.EqualFold(blobSHA256(data), expected) {
		return errors.New("blob payload SHA256 does not match expected_sha256")
	}
	return nil
}

func blobReferences(name string) []string {
	if !safeBlobName(name) {
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

func readBlobInventory() []blobFile {
	entries, err := os.ReadDir(blobRoot)
	if err != nil {
		return []blobFile{}
	}
	out := make([]blobFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !safeBlobName(entry.Name()) {
			continue
		}
		path := filepath.Join(blobRoot, entry.Name())
		info, err := regularBlobFile(path)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		refs := blobReferences(entry.Name())
		out = append(out, blobFile{
			Name: entry.Name(), Path: path, Size: info.Size(),
			Modified: info.ModTime().UTC().Format(time.RFC3339),
			SHA256:   blobSHA256(data), Referenced: len(refs) > 0, References: refs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func blobInventoryResponse() map[string]any {
	files := readBlobInventory()
	return map[string]any{
		"root":  blobRoot,
		"files": files,
		"count": len(files),
	}
}

func handleBlobList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, blobInventoryResponse())
}

func handleBlobInstall(w http.ResponseWriter, r *http.Request) {
	var request blobInstallRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid blob install request"})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	expectedConfirm := "NFQWS_BLOB_INSTALL"
	if request.Replace {
		expectedConfirm = "NFQWS_BLOB_REPLACE"
	}
	if request.Confirm != expectedConfirm {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "invalid blob install confirmation"})
		return
	}
	path, err := blobPath(request.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	data, err := base64.StdEncoding.DecodeString(request.ContentBase64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "content_base64 is invalid"})
		return
	}
	if err := validateBlobPayload(request.Name, data, request.ExpectedSHA256); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if err := ensureBlobRoot(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "prepare blob root: " + err.Error()})
		return
	}

	mode := os.FileMode(0644)
	backup := ""
	replaced := false
	references := []string{}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "existing blob path is not a regular file"})
			return
		}
		if !request.Replace {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "blob already exists; explicit replace is required"})
			return
		}
		if info.Size() > blobMaxBytes {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "existing blob exceeds manager safety limit"})
			return
		}
		before, err := os.ReadFile(path)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read existing blob: " + err.Error()})
			return
		}
		if blobSHA256(before) == blobSHA256(data) {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": true, "name": request.Name, "already_current": true,
				"sha256": blobSHA256(data), "inventory": blobInventoryResponse(),
			})
			return
		}
		mode = info.Mode().Perm()
		references = blobReferences(request.Name)
		backup, err = createNamedBackup("blob-"+request.Name, before, mode)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create blob backup: " + err.Error()})
			return
		}
		replaced = true
	} else if !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "stat blob: " + err.Error()})
		return
	}

	if err := safety.WriteFileAtomic(path, data, mode); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "install blob: " + err.Error(), "backup": backup})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "name": request.Name, "sha256": blobSHA256(data),
		"replaced": replaced, "backup": backup, "references": references,
		"runtime_restarted": false, "inventory": blobInventoryResponse(),
	})
}

func handleBlobDelete(w http.ResponseWriter, r *http.Request) {
	var request blobDeleteRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid blob delete request"})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Confirm != "NFQWS_BLOB_DELETE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_BLOB_DELETE"})
		return
	}
	path, err := blobPath(request.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	info, err := regularBlobFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "blob not found"})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	refs := blobReferences(request.Name)
	if len(refs) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "blob is referenced by active or stored config", "references": refs,
		})
		return
	}
	before, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read blob before delete: " + err.Error()})
		return
	}
	backup, err := createNamedBackup("blob-delete-"+request.Name, before, info.Mode().Perm())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create blob delete backup: " + err.Error()})
		return
	}
	if err := os.Remove(path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete blob: " + err.Error(), "backup": backup})
		return
	}
	_ = pruneBackups()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "name": request.Name, "backup": backup, "inventory": blobInventoryResponse(),
	})
}
