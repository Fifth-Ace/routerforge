package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const coreLoggingConfigPath = "/opt/etc/routerforge/logging.json"
const coreRAMLogPath = "/tmp/routerforge/logs/routerforge.log"

var coreLoggingMu sync.Mutex

type coreLoggingSettings struct {
	Mode       string `json:"mode"`
	CustomPath string `json:"custom_path,omitempty"`
	MaxMB      int    `json:"max_mb"`
	Backups    int    `json:"backups"`
}

func defaultCoreLoggingSettings() coreLoggingSettings {
	return coreLoggingSettings{Mode: "entware", MaxMB: 1, Backups: 2}
}

// Explicit, constrained log locations only. In particular, never write to
// configuration, executables, procfs or the router's filesystem root.
func coreCustomPathValid(p string) bool {
	if p == "" || len(p) > 255 || !filepath.IsAbs(p) || filepath.Clean(p) != p ||
		!strings.HasSuffix(p, ".log") || strings.ContainsAny(p, "\r\n\x00\\") {
		return false
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	for _, root := range []string{
		"/opt/var/log/", "/opt/mnt/", "/tmp/", "/mnt/", "/media/", "/storage/", "/var/log/",
	} {
		if strings.HasPrefix(p, root) && len(p) > len(root) {
			return true
		}
	}
	return false
}

func (s coreLoggingSettings) validate() error {
	if s.Mode != "entware" && s.Mode != "ram" && s.Mode != "custom" {
		return errors.New("mode must be entware, ram or custom")
	}
	if s.MaxMB < 1 || s.MaxMB > 16 || s.Backups < 1 || s.Backups > 5 {
		return errors.New("log rotation limits out of range")
	}
	if s.Mode == "custom" && !coreCustomPathValid(s.CustomPath) {
		return errors.New("invalid custom log path; use an existing log directory and a .log file")
	}
	// Retain a previously entered custom path when switching modes, but never
	// persist a dangerous path even when currently inactive.
	if s.CustomPath != "" && !coreCustomPathValid(s.CustomPath) {
		return errors.New("invalid saved custom log path")
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
	switch s.Mode {
	case "ram":
		if !ramAvailable {
			return "", errors.New("/tmp is not tmpfs; RAM logging is unavailable")
		}
		return coreRAMLogPath, nil
	case "custom":
		return s.CustomPath, nil
	default:
		return original, nil
	}
}

// No custom directories are created automatically. This prevents misspelled
// mount points from silently putting logs onto internal flash instead of USB.
func checkCoreCustomLogLocation(path string, probeWritable bool) error {
	if !coreCustomPathValid(path) {
		return errors.New("invalid custom log path")
	}
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("custom log directory unavailable: %w", err)
	}
	if !info.IsDir() {
		return errors.New("custom log parent is not a directory")
	}
	// Resolve parent symlinks before touching a file; symlinked mountpoints are
	// accepted only if their final destination remains inside allowed roots.
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil || !coreCustomPathValid(filepath.Join(resolved, filepath.Base(path))) {
		return errors.New("custom log directory resolves outside allowed storage roots")
	}
	// Do not follow an existing log file symlink or write into special files.
	fi, err := os.Lstat(path)
	if err == nil {
		if !fi.Mode().IsRegular() {
			return errors.New("custom log target must be a regular non-symlink file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("custom log target not accessible: %w", err)
	}
	if probeWritable {
		f, err := os.CreateTemp(parent, ".routerforge-log-probe-*")
		if err != nil {
			return fmt.Errorf("custom log directory not writable: %w", err)
		}
		name := f.Name()
		closeErr := f.Close()
		removeErr := os.Remove(name)
		if closeErr != nil {
			return closeErr
		}
		if removeErr != nil {
			return removeErr
		}
	}
	return nil
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
		path, pathErr := resolveCoreLogPath(s, "/opt/var/log/routerforge.log", coreRAMLoggingAvailable())
		// Report the effective location without ever rewriting it on GET.
		if pathErr != nil {
			path = ""
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"settings": s, "ram_available": coreRAMLoggingAvailable(), "active_path": path})
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
			http.Error(w, `{"error":"invalid logging limits, mode or path"}`, http.StatusBadRequest)
			return
		}
		if s.Mode == "ram" && !coreRAMLoggingAvailable() {
			http.Error(w, `{"error":"/tmp is not a RAM filesystem"}`, http.StatusConflict)
			return
		}
		if s.Mode == "custom" {
			if err := checkCoreCustomLogLocation(s.CustomPath, true); err != nil {
				http.Error(w, `{"error":"custom log directory unavailable or unwritable"}`, http.StatusConflict)
				return
			}
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
		path, _ := resolveCoreLogPath(s, "/opt/var/log/routerforge.log", coreRAMLoggingAvailable())
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "settings": s, "active_path": path})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, `{"error":"GET or POST required"}`, http.StatusMethodNotAllowed)
	}
}
