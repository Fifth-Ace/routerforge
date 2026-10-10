package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// Work with the real configuration files of existing Entware services.
// This intentionally does not generate commands or change live firewall rules.
const engineConfigMaximum = 128 << 10

var engineConfigMu sync.Mutex

var engineConfigFiles = map[string]string{
	"knockd":  "/opt/etc/knockd.conf",
	"fwknopd": "/opt/etc/fwknop/access.conf",
}

type engineConfigRequest struct {
	Action     string `json:"action"`
	Confirm    string `json:"confirm"`
	BaseSHA256 string `json:"base_sha256"`
	Content    string `json:"content"`
}

func configSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validEngineConfig(engine string, data []byte) error {
	if engineConfigFiles[engine] == "" {
		return errors.New("unknown engine")
	}
	if len(data) == 0 || len(data) > engineConfigMaximum || strings.IndexByte(string(data), 0) >= 0 {
		return errors.New("empty, too large, or binary configuration")
	}
	if !strings.Contains(string(data), "\n") {
		return errors.New("configuration must contain lines")
	}
	return nil
}

func readRegularConfig(path string) ([]byte, os.FileMode, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, 0, err
	}
	if !st.Mode().IsRegular() || st.Size() > engineConfigMaximum {
		return nil, 0, errors.New("configuration must be a regular file within size limit")
	}
	b, err := os.ReadFile(path)
	return b, st.Mode().Perm(), err
}

func saveExistingEngineConfig(path, engine, beforeSHA string, content []byte) (string, error) {
	if err := validEngineConfig(engine, content); err != nil {
		return "", err
	}
	previous, mode, err := readRegularConfig(path)
	if err != nil {
		return "", err
	}
	if configSHA(previous) != beforeSHA {
		return "", errors.New("configuration was modified since reading; reload before saving")
	}
	// Backup the last working bytes before replacing the file; no process restart here.
	backup := path + ".routerforge.bak"
	if _, err := os.Lstat(backup); err == nil {
		st, err := os.Lstat(backup)
		if err != nil || !st.Mode().IsRegular() {
			return "", errors.New("invalid backup path")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := safety.WriteFileAtomic(backup, previous, 0600); err != nil {
		return "", err
	}
	if err := safety.WriteFileAtomic(path, content, mode); err != nil {
		return "", err
	}
	return configSHA(content), nil
}

func restoreExistingEngineConfig(path, engine, beforeSHA string) (string, error) {
	current, mode, err := readRegularConfig(path)
	if err != nil {
		return "", err
	}
	if configSHA(current) != beforeSHA {
		return "", errors.New("configuration changed; reload before restore")
	}
	backup, _, err := readRegularConfig(path + ".routerforge.bak")
	if err != nil {
		return "", err
	}
	if err := validEngineConfig(engine, backup); err != nil {
		return "", err
	}
	if err := safety.WriteFileAtomic(path, backup, mode); err != nil {
		return "", err
	}
	return configSHA(backup), nil
}

func engineConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		jsonReply(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	engine := r.URL.Query().Get("engine")
	path, ok := engineConfigFiles[engine]
	if !ok {
		jsonReply(w, 400, map[string]string{"error": "unknown engine"})
		return
	}
	engineConfigMu.Lock()
	defer engineConfigMu.Unlock()
	content, _, err := readRegularConfig(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			jsonReply(w, 404, map[string]string{"error": "install and configure the upstream service first; file missing"})
			return
		}
		jsonReply(w, 409, map[string]string{"error": "configuration unavailable"})
		return
	}
	digest := configSHA(content)
	if r.Method == http.MethodGet {
		_, _, backErr := readRegularConfig(path + ".routerforge.bak")
		jsonReply(w, 200, map[string]any{"engine": engine, "present": true, "size": len(content), "sha256": digest, "backup_available": backErr == nil, "secrets_hidden": true})
		return
	}
	if r.Header.Get("X-RouterForge-Module-Authorized") != "core-authorized-v1" {
		jsonReply(w, 403, map[string]string{"error": "Core authorization required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var req engineConfigRequest
	if err := decoder.Decode(&req); err != nil {
		jsonReply(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		jsonReply(w, 400, map[string]string{"error": "trailing request data"})
		return
	}
	if req.BaseSHA256 != digest {
		jsonReply(w, 409, map[string]string{"error": "configuration changed; refresh before editing"})
		return
	}
	switch req.Action {
	case "read":
		if req.Confirm != "SHOW_CONFIG" {
			jsonReply(w, 400, map[string]string{"error": "explicit read confirmation required"})
			return
		}
		// Secrets in fwknopd are returned only following an explicit authenticated POST.
		jsonReply(w, 200, map[string]any{"engine": engine, "content": string(content), "sha256": digest})
	case "save":
		if req.Confirm != "SAVE_CONFIG" {
			jsonReply(w, 400, map[string]string{"error": "save confirmation required"})
			return
		}
		after, err := saveExistingEngineConfig(path, engine, digest, []byte(req.Content))
		if err != nil {
			jsonReply(w, 409, map[string]string{"error": err.Error()})
			return
		}
		jsonReply(w, 200, map[string]any{"ok": true, "sha256": after, "backup_available": true, "restart_required": true})
	case "restore":
		if req.Confirm != "RESTORE_CONFIG" {
			jsonReply(w, 400, map[string]string{"error": "restore confirmation required"})
			return
		}
		after, err := restoreExistingEngineConfig(path, engine, digest)
		if err != nil {
			jsonReply(w, 409, map[string]string{"error": err.Error()})
			return
		}
		jsonReply(w, 200, map[string]any{"ok": true, "sha256": after, "restart_required": true})
	default:
		jsonReply(w, 400, map[string]string{"error": "unsupported action"})
	}
}
