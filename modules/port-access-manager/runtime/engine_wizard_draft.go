package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"sync"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const engineWizardDraftDir = "/opt/etc/routerforge/port-access-manager/wizard-drafts"

var engineWizardDraftMu sync.Mutex

func wizardDraftPath(dir, engine string) (string, error) {
	if engine != "knockd" && engine != "fwknopd" {
		return "", errors.New("unsupported engine")
	}
	return filepath.Join(dir, engine+".json"), nil
}
func wizardDraftHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func readWizardDraft(dir, engine string) (EngineWizardRequest, string, error) {
	path, err := wizardDraftPath(dir, engine)
	if err != nil {
		return EngineWizardRequest{}, "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return EngineWizardRequest{}, "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 8192 {
		return EngineWizardRequest{}, "", errors.New("invalid draft file")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return EngineWizardRequest{}, "", err
	}
	var req EngineWizardRequest
	if err = json.Unmarshal(b, &req); err != nil {
		return EngineWizardRequest{}, "", err
	}
	if req.Engine != engine {
		return EngineWizardRequest{}, "", errors.New("draft engine mismatch")
	}
	if err = validateWizard(req); err != nil {
		return EngineWizardRequest{}, "", err
	}
	return req, wizardDraftHash(b), nil
}
func storeWizardDraft(dir string, req EngineWizardRequest, expected string) (string, error) {
	if err := validateWizard(req); err != nil {
		return "", err
	}
	path, err := wizardDraftPath(dir, req.Engine)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	_, before, err := readWizardDraft(dir, req.Engine)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if errors.Is(err, os.ErrNotExist) {
		if expected != "" {
			return "", errors.New("draft changed; refresh required")
		}
	}
	if err == nil && expected != before {
		return "", errors.New("draft changed; refresh required")
	}
	b, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	if len(b) > 8192 {
		return "", errors.New("draft exceeds size limit")
	}
	if err = safety.WriteFileAtomic(path, b, 0600); err != nil {
		return "", err
	}
	return wizardDraftHash(b), nil
}
func engineWizardDraftHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		jsonReply(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if r.Header.Get("X-RouterForge-Module-Authorized") != "core-authorized-v1" {
		jsonReply(w, 403, map[string]string{"error": "Core authorization required"})
		return
	}
	engineWizardDraftMu.Lock()
	defer engineWizardDraftMu.Unlock()
	if r.Method == http.MethodGet {
		engine := r.URL.Query().Get("engine")
		if _, err := wizardDraftPath(engineWizardDraftDir, engine); err != nil {
			jsonReply(w, 400, map[string]string{"error": err.Error()})
			return
		}
		req, sha, err := readWizardDraft(engineWizardDraftDir, engine)
		if errors.Is(err, os.ErrNotExist) {
			jsonReply(w, 200, map[string]any{"exists": false, "engine": engine, "applied": false})
			return
		}
		if err != nil {
			jsonReply(w, 409, map[string]string{"error": err.Error()})
			return
		}
		jsonReply(w, 200, map[string]any{"exists": true, "engine": engine, "draft": req, "sha256": sha, "applied": false})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var body struct {
		Draft      EngineWizardRequest `json:"draft"`
		BaseSHA256 string              `json:"base_sha256"`
		Confirm    string              `json:"confirm"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		jsonReply(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		jsonReply(w, 400, map[string]string{"error": "trailing request"})
		return
	}
	if body.Confirm != "SAVE_DRAFT" {
		jsonReply(w, 400, map[string]string{"error": "confirmation required"})
		return
	}
	sha, err := storeWizardDraft(engineWizardDraftDir, body.Draft, body.BaseSHA256)
	if err != nil {
		jsonReply(w, 409, map[string]string{"error": err.Error()})
		return
	}
	jsonReply(w, 200, map[string]any{"ok": true, "sha256": sha, "applied": false, "service_started": false})
}
