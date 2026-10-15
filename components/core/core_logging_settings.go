package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const coreLoggingConfigPath = "/opt/etc/routerforge/logging.json"
const coreRAMLogPath = "/tmp/routerforge/logs/routerforge.log"

var coreLoggingMu sync.Mutex

type coreLoggingSettings struct {
	Mode    string `json:"mode"`
	MaxMB   int    `json:"max_mb"`
	Backups int    `json:"backups"`
}

func defaultCoreLoggingSettings() coreLoggingSettings {
	return coreLoggingSettings{Mode: "entware", MaxMB: 1, Backups: 2}
}

func (s coreLoggingSettings) validate() error {
	if s.Mode != "entware" && s.Mode != "ram" {
		return errors.New("mode must be entware or ram")
	}
	if s.MaxMB < 1 || s.MaxMB > 16 || s.Backups < 1 || s.Backups > 5 {
		return errors.New("log rotation limits out of range")
	}
	return nil
}

func readCoreLoggingSettings() (coreLoggingSettings, error) {
	data, err := os.ReadFile(coreLoggingConfigPath)
	if errors.Is(err, os.ErrNotExist) {
		return defaultCoreLoggingSettings(), nil
	}
	if err != nil {
		return coreLoggingSettings{}, err
	}
	var s coreLoggingSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return coreLoggingSettings{}, err
	}
	return s, s.validate()
}

func resolveCoreLogPath(s coreLoggingSettings, original string, ramAvailable bool) (string, error) {
	if err := s.validate(); err != nil {
		return "", err
	}
	if s.Mode == "ram" {
		if !ramAvailable {
			return "", errors.New("/tmp is not tmpfs; RAM logging is unavailable")
		}
		return coreRAMLogPath, nil
	}
	return original, nil
}

func coreLoggingHandler(w http.ResponseWriter, r *http.Request) {
	coreLoggingMu.Lock()
	defer coreLoggingMu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch r.Method {
	case http.MethodGet:
		s, err := readCoreLoggingSettings()
		if err != nil {
			http.Error(w, `{"error":"logging settings unreadable"}`, http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"settings": s, "ram_available": coreRAMLoggingAvailable()})
	case http.MethodPost:
		if !sameOriginRequest(r) {
			http.Error(w, `{"error":"cross-origin request rejected"}`, http.StatusForbidden)
			return
		}
		if r.ContentLength > 4096 {
			http.Error(w, `{"error":"request too large"}`, http.StatusRequestEntityTooLarge)
			return
		}
		var s coreLoggingSettings
		dec := json.NewDecoder(io.LimitReader(r.Body, 4097))
		dec.DisallowUnknownFields()
		if dec.Decode(&s) != nil {
			http.Error(w, `{"error":"invalid settings"}`, http.StatusBadRequest)
			return
		}
		var trailing any
		if dec.Decode(&trailing) != io.EOF {
			http.Error(w, `{"error":"trailing data"}`, http.StatusBadRequest)
			return
		}
		if err := s.validate(); err != nil {
			http.Error(w, `{"error":"invalid logging limits or mode"}`, http.StatusBadRequest)
			return
		}
		if s.Mode == "ram" && !coreRAMLoggingAvailable() {
			http.Error(w, `{"error":"/tmp is not a RAM filesystem"}`, http.StatusConflict)
			return
		}
		if err := os.MkdirAll(filepath.Dir(coreLoggingConfigPath), 0700); err != nil {
			http.Error(w, `{"error":"settings directory unavailable"}`, http.StatusInternalServerError)
			return
		}
		data, err := json.Marshal(s)
		if err != nil || safety.WriteFileAtomic(coreLoggingConfigPath, append(data, '\n'), 0600) != nil {
			http.Error(w, `{"error":"settings write failed"}`, http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "settings": s})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, `{"error":"GET or POST required"}`, http.StatusMethodNotAllowed)
	}
}
