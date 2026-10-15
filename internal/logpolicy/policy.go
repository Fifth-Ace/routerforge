// Package logpolicy keeps RouterForge-owned service logs on the same storage
// selected in Core settings. It does not modify third-party service logs.
package logpolicy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const SettingsPath = "/opt/etc/routerforge/logging.json"
const RAMDir = "/tmp/routerforge/logs"
const EntwareDir = "/opt/var/log/routerforge"

var safeComponent = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
var writeMu sync.Mutex

type Settings struct {
	Mode       string `json:"mode"`
	CustomPath string `json:"custom_path,omitempty"`
	MaxMB      int    `json:"max_mb"`
	Backups    int    `json:"backups"`
}

func (s Settings) valid() error {
	if s.Mode != "entware" && s.Mode != "ram" && s.Mode != "custom" {
		return errors.New("invalid logging mode")
	}
	if s.MaxMB < 1 || s.MaxMB > 16 || s.Backups < 1 || s.Backups > 5 {
		return errors.New("invalid log retention")
	}
	if s.CustomPath != "" && !safeCustomPath(s.CustomPath) {
		return errors.New("invalid custom log path")
	}
	if s.Mode == "custom" && s.CustomPath == "" {
		return errors.New("missing custom log path")
	}
	return nil
}

func safeCustomPath(path string) bool {
	if path == "" || len(path) > 255 || !filepath.IsAbs(path) || filepath.Clean(path) != path ||
		!strings.HasSuffix(path, ".log") || strings.ContainsAny(path, "\r\n\x00\\") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	for _, prefix := range []string{"/opt/var/log/", "/opt/mnt/", "/tmp/", "/mnt/", "/media/", "/storage/", "/var/log/"} {
		if strings.HasPrefix(path, prefix) && len(path) > len(prefix) {
			return true
		}
	}
	return false
}

func readSettings(configFile string) (Settings, error) {
	defaults := Settings{Mode: "entware", MaxMB: 1, Backups: 2}
	raw, err := os.ReadFile(configFile)
	if errors.Is(err, os.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return Settings{}, err
	}
	if len(raw) > 4096 {
		return Settings{}, errors.New("oversize logging settings")
	}
	var s Settings
	if err = json.Unmarshal(raw, &s); err != nil {
		return Settings{}, err
	}
	return s, s.valid()
}

// Resolve returns the log file used for new records. The existing service's
// default path is retained in Entware mode (no migration on package upgrade).
func Resolve(module, defaultPath string) (string, error) {
	s, err := readSettings(SettingsPath)
	if err != nil {
		return "", err
	}
	return resolve(s, module, defaultPath, ramFSAvailable())
}

func resolve(s Settings, module, defaultPath string, ramAvailable bool) (string, error) {
	if err := s.valid(); err != nil {
		return "", err
	}
	if !safeComponent.MatchString(module) {
		return "", errors.New("invalid module id")
	}
	if defaultPath == "" || !filepath.IsAbs(defaultPath) {
		return "", errors.New("invalid default log path")
	}
	if s.Mode == "entware" {
		return defaultPath, nil
	}
	if s.Mode == "ram" && !ramAvailable {
		return "", errors.New("RAM logging requested without tmpfs")
	}
	directory := RAMDir
	if s.Mode == "custom" {
		directory = filepath.Dir(s.CustomPath)
	}
	extension := filepath.Ext(defaultPath)
	if extension != ".jsonl" {
		extension = ".log"
	}
	return filepath.Join(directory, "routerforge-"+module+extension), nil
}

// validateDestination prevents RAM writes through symlinked paths, and avoids
// silently creating custom directories on unmounted persistent storage.
func validateDestination(s Settings, path string) error {
	dir := filepath.Dir(path)
	if s.Mode == "custom" {
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errors.New("custom logging parent is not a directory")
		}
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil || !safeCustomPath(filepath.Join(resolved, filepath.Base(s.CustomPath))) {
			return errors.New("custom logging directory escapes allowed roots")
		}
	} else {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if s.Mode == "ram" {
			resolved, err := filepath.EvalSymlinks(dir)
			if err != nil || !strings.HasPrefix(resolved, "/tmp/") || !ramDirectoryAvailable(dir) {
				return errors.New("RAM log directory is not tmpfs")
			}
		}
	}
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return errors.New("log target is not a regular file")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// GuardedPath resolves a managed audit path and checks its storage before any
// audit append. It keeps the old path untouched when mode=entware.
func GuardedPath(module, defaultPath string) (string, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	s, err := readSettings(SettingsPath)
	if err != nil {
		return "", err
	}
	path, err := resolve(s, module, defaultPath, ramFSAvailable())
	if err != nil {
		return "", err
	}
	if err := validateDestination(s, path); err != nil {
		return "", err
	}
	return path, nil
}

// OpenAppendNoFollow prevents redirecting a log to a symlink at file-open time.
func OpenAppendNoFollow(path string) (*os.File, error) { return openAppendNoFollow(path) }

// Append writes exactly one bounded record, rotating at the configured limit.
// If configuration is damaged/unavailable, it never silently falls back to flash.
func Append(module, defaultPath string, record []byte) error {
	if len(record) == 0 {
		return nil
	}
	if len(record) > 64*1024 {
		return errors.New("log record too large")
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	s, err := readSettings(SettingsPath)
	if err != nil {
		return err
	}
	return appendWithSettings(s, module, defaultPath, record, ramFSAvailable())
}

func appendWithSettings(s Settings, module, defaultPath string, record []byte, ramAvailable bool) error {
	path, err := resolve(s, module, defaultPath, ramAvailable)
	if err != nil {
		return err
	}
	if err = validateDestination(s, path); err != nil {
		return err
	}
	maxSize := int64(s.MaxMB) * 1024 * 1024
	info, err := os.Lstat(path)
	if err == nil && info.Size()+int64(len(record)) > maxSize {
		for i := s.Backups; i >= 1; i-- {
			older := fmt.Sprintf("%s.%d", path, i)
			if i == s.Backups {
				if err := removeIfRegular(older); err != nil {
					return err
				}
			} else {
				newer := fmt.Sprintf("%s.%d", path, i+1)
				if err := renameIfRegular(older, newer); err != nil {
					return err
				}
			}
		}
		if err := renameIfRegular(path, path+".1"); err != nil {
			return err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := openAppendNoFollow(path)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(record)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func removeIfRegular(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("refusing to remove non-regular rotated log")
	}
	return os.Remove(path)
}
func renameIfRegular(from, to string) error {
	info, err := os.Lstat(from)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("refusing to rotate non-regular log")
	}
	return os.Rename(from, to)
}

// Writer is a dynamic sink: changing SettingsPath affects the next log call,
// without restarting a running management process.
type Writer struct {
	Module      string
	DefaultPath string
}

func (w Writer) Write(b []byte) (int, error) {
	if err := Append(w.Module, w.DefaultPath, b); err != nil {
		// Keep operational errors visible via the existing daemon stderr.
		_, _ = fmt.Fprintf(os.Stderr, "routerforge-logpolicy %s: %v\n", w.Module, err)
		return len(b), nil
	}
	return len(b), nil
}
func Install(module string) {
	log.SetOutput(Writer{Module: module, DefaultPath: filepath.Join(EntwareDir, "routerforge-"+module+".log")})
	log.Printf("routerforge %s: diagnostic sink initialized", module)
}

// Explicitly expose a correctly structured writer, not a file descriptor held
// indefinitely at the old path. io.Writer assertion prevents interface drift.
var _ io.Writer = Writer{}
