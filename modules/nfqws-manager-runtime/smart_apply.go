package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	smartApplyMaxBody    = 24 << 20
	smartApplyMaxDecoded = 12 << 20
	smartApplyMaxDeps    = 64
)

var smartApplyHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

type smartApplyDependency struct {
	Name           string `json:"name"`
	ContentBase64  string `json:"content_base64"`
	ExpectedSHA256 string `json:"expected_sha256"`
}

type smartApplyRequest struct {
	Config               string                 `json:"config"`
	ExpectedActiveSHA256 string                 `json:"expected_active_sha256"`
	ExpectedConfigSHA256 string                 `json:"expected_config_sha256"`
	Lists                []smartApplyDependency `json:"lists,omitempty"`
	Blobs                []smartApplyDependency `json:"blobs,omitempty"`
	ExpectedLists        map[string]string      `json:"expected_lists,omitempty"`
	ExpectedBlobs        map[string]string      `json:"expected_blobs,omitempty"`
	Confirm              string                 `json:"confirm"`
}

type smartApplySnapshot struct {
	path    string
	data    []byte
	mode    os.FileMode
	existed bool
}

type smartApplyDecoded struct {
	name string
	data []byte
	hash string
	path string
}

func registerSmartApplyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/smart-apply", mutationOnly(handleSmartApply))
}

func decodeSmartApplyJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, smartApplyMaxBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func smartApplySHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func smartApplyReferencedNames(config string) (map[string]bool, map[string]bool) {
	lists := map[string]bool{}
	blobs := map[string]bool{}
	listPattern := regexp.MustCompile(`--(?:hostlist|hostlist-exclude|hostlist-auto|ipset|ipset-exclude)=([^\s"'\\]+)`)
	blobPattern := regexp.MustCompile(`--blob=[A-Za-z0-9._-]+:@?([^\s"'\\]+)`)
	for _, m := range listPattern.FindAllStringSubmatch(config, -1) {
		name := filepath.Base(strings.TrimSpace(m[1]))
		if safeListSourceName(name) {
			lists[name] = true
		}
	}
	for _, m := range blobPattern.FindAllStringSubmatch(config, -1) {
		name := filepath.Base(strings.TrimSpace(m[1]))
		if safeBlobName(name) {
			blobs[name] = true
		}
	}
	return lists, blobs
}

func smartApplyValidateExpected(kind string, expected map[string]string, referenced map[string]bool) error {
	if len(expected) > smartApplyMaxDeps {
		return fmt.Errorf("too many expected %s dependencies", kind)
	}
	for name, hash := range expected {
		if !referenced[name] {
			return fmt.Errorf("unreferenced expected %s dependency: %s", kind, name)
		}
		if !smartApplyHashPattern.MatchString(strings.TrimSpace(hash)) {
			return fmt.Errorf("invalid expected SHA256 for %s", name)
		}
	}
	return nil
}

func smartApplyDecodeDependencies(kind string, deps []smartApplyDependency, referenced map[string]bool) ([]smartApplyDecoded, int, error) {
	if len(deps) > smartApplyMaxDeps {
		return nil, 0, fmt.Errorf("too many %s dependencies", kind)
	}
	seen := map[string]bool{}
	out := make([]smartApplyDecoded, 0, len(deps))
	total := 0
	for _, dep := range deps {
		name := strings.TrimSpace(dep.Name)
		if seen[name] {
			return nil, 0, fmt.Errorf("duplicate %s dependency: %s", kind, name)
		}
		seen[name] = true
		if !referenced[name] {
			return nil, 0, fmt.Errorf("unreferenced %s dependency: %s", kind, name)
		}
		expected := strings.TrimSpace(dep.ExpectedSHA256)
		if !smartApplyHashPattern.MatchString(expected) {
			return nil, 0, fmt.Errorf("invalid expected_sha256 for %s", name)
		}
		data, err := base64.StdEncoding.DecodeString(dep.ContentBase64)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid content_base64 for %s", name)
		}
		var path string
		if kind == "list" {
			if err := validateListSourcePayload(name, data, expected); err != nil {
				return nil, 0, err
			}
			path, err = listSourcePath(name)
		} else {
			if err := validateBlobPayload(name, data, expected); err != nil {
				return nil, 0, err
			}
			path, err = blobPath(name)
		}
		if err != nil {
			return nil, 0, err
		}
		total += len(data)
		out = append(out, smartApplyDecoded{name: name, data: data, hash: strings.ToLower(expected), path: path})
	}
	return out, total, nil
}

func smartApplyExistingHash(path string, max int64) (string, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", false, errors.New("dependency path is not a regular file")
	}
	if info.Size() > max {
		return "", false, errors.New("dependency exceeds manager safety limit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	return smartApplySHA256(data), true, nil
}

func smartApplyMissing(config string, providedLists, providedBlobs map[string]bool) ([]string, []string, error) {
	listRefs, blobRefs := smartApplyReferencedNames(config)
	missingLists := []string{}
	missingBlobs := []string{}
	for name := range listRefs {
		if providedLists[name] {
			continue
		}
		path, _ := listSourcePath(name)
		if _, ok, err := smartApplyExistingHash(path, listFileMaxBytes); err != nil {
			return nil, nil, fmt.Errorf("list %s: %w", name, err)
		} else if !ok {
			missingLists = append(missingLists, name)
		}
	}
	for name := range blobRefs {
		if providedBlobs[name] {
			continue
		}
		path, _ := blobPath(name)
		if _, ok, err := smartApplyExistingHash(path, blobMaxBytes); err != nil {
			return nil, nil, fmt.Errorf("blob %s: %w", name, err)
		} else if !ok {
			missingBlobs = append(missingBlobs, name)
		}
	}
	sort.Strings(missingLists)
	sort.Strings(missingBlobs)
	return missingLists, missingBlobs, nil
}

func smartApplySnapshotPath(path string, fallbackMode os.FileMode) (smartApplySnapshot, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return smartApplySnapshot{path: path, mode: fallbackMode, existed: false}, nil
	}
	if err != nil {
		return smartApplySnapshot{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return smartApplySnapshot{}, errors.New("target is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return smartApplySnapshot{}, err
	}
	return smartApplySnapshot{path: path, data: data, mode: info.Mode().Perm(), existed: true}, nil
}

func smartApplyRollback(snapshots []smartApplySnapshot, configSnap smartApplySnapshot, wasRunning bool) (bool, string) {
	errs := []string{}
	for i := len(snapshots) - 1; i >= 0; i-- {
		s := snapshots[i]
		if s.existed {
			if err := safety.WriteFileAtomic(s.path, s.data, s.mode); err != nil {
				errs = append(errs, s.path+": "+err.Error())
			}
		} else if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, s.path+": "+err.Error())
		}
	}
	if configSnap.existed {
		if err := safety.WriteFileAtomic(configSnap.path, configSnap.data, configSnap.mode); err != nil {
			errs = append(errs, "config: "+err.Error())
		}
	}
	if wasRunning {
		if out, err := runInit("restart"); err != nil {
			errs = append(errs, "restart: "+err.Error()+" output="+out)
		}
	}
	return len(errs) == 0, strings.Join(errs, "; ")
}

func handleSmartApply(w http.ResponseWriter, r *http.Request) {
	var request smartApplyRequest
	if err := decodeSmartApplyJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid smart apply request"})
		return
	}
	if request.Confirm != "NFQWS_SMART_APPLY" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal NFQWS_SMART_APPLY"})
		return
	}
	if err := validateConfig(request.Config); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(request.ExpectedActiveSHA256)) || !smartApplyHashPattern.MatchString(strings.TrimSpace(request.ExpectedConfigSHA256)) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "expected config hashes must be SHA256"})
		return
	}
	candidateHash := smartApplySHA256([]byte(request.Config))
	if !strings.EqualFold(candidateHash, request.ExpectedConfigSHA256) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "candidate config SHA256 mismatch"})
		return
	}

	status := readStatus()
	if !status.MutationReady || !status.ConfigExists || status.ConfigCut {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "nfqws2 runtime is not ready for Smart Apply"})
		return
	}
	if !strings.EqualFold(status.ConfigSHA256, request.ExpectedActiveSHA256) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "active config changed since Smart Apply plan", "current_sha256": status.ConfigSHA256})
		return
	}

	listRefs, blobRefs := smartApplyReferencedNames(request.Config)
	if err := smartApplyValidateExpected("list", request.ExpectedLists, listRefs); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if err := smartApplyValidateExpected("blob", request.ExpectedBlobs, blobRefs); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	decodedLists, listBytes, err := smartApplyDecodeDependencies("list", request.Lists, listRefs)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	decodedBlobs, blobBytes, err := smartApplyDecodeDependencies("blob", request.Blobs, blobRefs)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if listBytes+blobBytes+len(request.Config) > smartApplyMaxDecoded {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "Smart Apply decoded payload exceeds safety limit"})
		return
	}
	providedLists := map[string]bool{}
	providedBlobs := map[string]bool{}
	for _, dep := range decodedLists {
		providedLists[dep.name] = true
	}
	for _, dep := range decodedBlobs {
		providedBlobs[dep.name] = true
	}
	missingLists, missingBlobs, err := smartApplyMissing(request.Config, providedLists, providedBlobs)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	if len(missingLists) > 0 || len(missingBlobs) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "candidate dependencies are incomplete", "missing_lists": missingLists, "missing_blobs": missingBlobs})
		return
	}
	for name, expected := range request.ExpectedLists {
		if providedLists[name] {
			continue
		}
		path, _ := listSourcePath(name)
		h, ok, err := smartApplyExistingHash(path, listFileMaxBytes)
		if err != nil || !ok || !strings.EqualFold(h, expected) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "list changed since Smart Apply plan: " + name})
			return
		}
	}
	for name, expected := range request.ExpectedBlobs {
		if providedBlobs[name] {
			continue
		}
		path, _ := blobPath(name)
		h, ok, err := smartApplyExistingHash(path, blobMaxBytes)
		if err != nil || !ok || !strings.EqualFold(h, expected) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "blob changed since Smart Apply plan: " + name})
			return
		}
	}

	if err := ensureListSourceRoot(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "prepare lists root: " + err.Error()})
		return
	}
	if err := ensureBlobRoot(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "prepare blob root: " + err.Error()})
		return
	}

	configSnap, err := smartApplySnapshotPath(configPath, 0644)
	if err != nil || !configSnap.existed {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "snapshot active config failed"})
		return
	}
	backups := []string{}
	if b, err := createNamedBackup("smart-apply-config", configSnap.data, configSnap.mode); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create Smart Apply config backup: " + err.Error()})
		return
	} else {
		backups = append(backups, b)
	}

	touched := []smartApplySnapshot{}
	appliedLists := []string{}
	appliedBlobs := []string{}
	rollbackAndReply := func(code int, message string, output string) {
		rolledBack, rollbackErr := smartApplyRollback(touched, configSnap, status.Running)
		writeJSON(w, code, map[string]any{
			"error": message, "output": output, "rolled_back": rolledBack,
			"rollback_error": rollbackErr, "backups": backups,
		})
	}

	writeDep := func(dep smartApplyDecoded, mode os.FileMode, label string) error {
		existingHash, exists, err := smartApplyExistingHash(dep.path, func() int64 {
			if label == "list" {
				return listFileMaxBytes
			}
			return blobMaxBytes
		}())
		if err != nil {
			return err
		}
		if exists && strings.EqualFold(existingHash, dep.hash) {
			return nil
		}
		snap, err := smartApplySnapshotPath(dep.path, mode)
		if err != nil {
			return err
		}
		if snap.existed {
			b, err := createNamedBackup("smart-apply-"+label+"-"+dep.name, snap.data, snap.mode)
			if err != nil {
				return err
			}
			backups = append(backups, b)
			mode = snap.mode
		}
		if err := safety.WriteFileAtomic(dep.path, dep.data, mode); err != nil {
			return err
		}
		touched = append(touched, snap)
		return nil
	}

	for _, dep := range decodedLists {
		if err := writeDep(dep, 0644, "list"); err != nil {
			rollbackAndReply(http.StatusInternalServerError, "write list dependency: "+err.Error(), "")
			return
		}
		appliedLists = append(appliedLists, dep.name)
	}
	for _, dep := range decodedBlobs {
		if err := writeDep(dep, 0644, "blob"); err != nil {
			rollbackAndReply(http.StatusInternalServerError, "write blob dependency: "+err.Error(), "")
			return
		}
		appliedBlobs = append(appliedBlobs, dep.name)
	}

	if err := safety.WriteFileAtomic(configPath, []byte(request.Config), configSnap.mode); err != nil {
		rollbackAndReply(http.StatusInternalServerError, "publish Smart Apply config: "+err.Error(), "")
		return
	}

	output := ""
	if status.Running {
		output, err = runInit("restart")
		if err != nil {
			rollbackAndReply(http.StatusBadGateway, err.Error(), output)
			return
		}
	}

	after := readStatus()
	for attempt := 0; attempt < 10 && (after.ConfigSHA256 != candidateHash || status.Running && !after.Running); attempt++ {
		time.Sleep(200 * time.Millisecond)
		after = readStatus()
	}
	if after.ConfigSHA256 != candidateHash || status.Running && !after.Running {
		rollbackAndReply(http.StatusBadGateway, "Smart Apply runtime verification failed", output)
		return
	}
	for _, dep := range decodedLists {
		h, ok, err := smartApplyExistingHash(dep.path, listFileMaxBytes)
		if err != nil || !ok || !strings.EqualFold(h, dep.hash) {
			rollbackAndReply(http.StatusBadGateway, "Smart Apply list verification failed: "+dep.name, output)
			return
		}
	}
	for _, dep := range decodedBlobs {
		h, ok, err := smartApplyExistingHash(dep.path, blobMaxBytes)
		if err != nil || !ok || !strings.EqualFold(h, dep.hash) {
			rollbackAndReply(http.StatusBadGateway, "Smart Apply blob verification failed: "+dep.name, output)
			return
		}
	}
	for name, expected := range request.ExpectedLists {
		path, _ := listSourcePath(name)
		h, ok, err := smartApplyExistingHash(path, listFileMaxBytes)
		if err != nil || !ok || !strings.EqualFold(h, expected) {
			rollbackAndReply(http.StatusBadGateway, "Smart Apply expected list verification failed: "+name, output)
			return
		}
	}
	for name, expected := range request.ExpectedBlobs {
		path, _ := blobPath(name)
		h, ok, err := smartApplyExistingHash(path, blobMaxBytes)
		if err != nil || !ok || !strings.EqualFold(h, expected) {
			rollbackAndReply(http.StatusBadGateway, "Smart Apply expected blob verification failed: "+name, output)
			return
		}
	}

	_ = pruneBackups()
	sort.Strings(appliedLists)
	sort.Strings(appliedBlobs)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "config_sha256": candidateHash, "applied_lists": appliedLists,
		"applied_blobs": appliedBlobs, "backups": backups, "service_restarted": status.Running,
		"restart_output": output, "rolled_back": false, "status": after,
	})
}
