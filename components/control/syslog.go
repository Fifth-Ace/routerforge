package main

// RouterForge Syslog Manager: an explicit, opt-in configuration manager for
// Entware's syslog-ng. Reading this API never installs packages or changes files.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	rfSyslogConfig      = "/opt/etc/syslog-ng.conf"
	rfSyslogMeta        = "/opt/etc/routerforge/syslog-manager.json"
	rfSyslogBackup      = "/opt/etc/routerforge/syslog-backups"
	rfSyslogDefaultRoot = "/opt/var/log/routerforge-syslog"
	rfSyslogMarker      = "# MANAGED BY ROUTERFORGE SYSLOG MANAGER v1"
)

var rfSyslogMu sync.Mutex
var rfSyslogDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
var rfSyslogFile = regexp.MustCompile(`^syslog-(entware|[0-9A-Fa-f:.]+)\.log$`)

type rfSyslogSettings struct {
	Bind     string   `json:"bind"`
	Port     int      `json:"port"`
	UDP      bool     `json:"udp"`
	TCP      bool     `json:"tcp"`
	Local    bool     `json:"local"`
	Allowed  []string `json:"allowed"`
	Root     string   `json:"root"`
	KeepDays int      `json:"keep_days"`
	MaxMB    int      `json:"max_mb"`
}

func rfSyslogDefaults() rfSyslogSettings {
	return rfSyslogSettings{
		Bind: "0.0.0.0", Port: 514, UDP: true, TCP: false, Local: false,
		Allowed: []string{"192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"},
		Root:    rfSyslogDefaultRoot, KeepDays: 30, MaxMB: 500,
	}
}

func rfSyslogValidate(s rfSyslogSettings) error {
	if s.Bind != "0.0.0.0" && s.Bind != "127.0.0.1" {
		return errors.New("bind must be 0.0.0.0 or 127.0.0.1")
	}
	if s.Port < 1024 && s.Port != 514 || s.Port > 65535 {
		return errors.New("invalid syslog port")
	}
	if !s.UDP && !s.TCP && !s.Local {
		return errors.New("at least one source required")
	}
	if s.KeepDays < 1 || s.KeepDays > 365 || s.MaxMB < 10 || s.MaxMB > 10240 {
		return errors.New("retention out of bounds")
	}
	if !rfSyslogRootAllowed(s.Root) {
		return errors.New("invalid log storage path")
	}
	if (s.UDP || s.TCP) && (len(s.Allowed) < 1 || len(s.Allowed) > 16) {
		return errors.New("1..16 allowed networks required")
	}
	for _, raw := range s.Allowed {
		if strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\n\r\"'") {
			return errors.New("invalid allowed network")
		}
		_, cidr, err := net.ParseCIDR(raw)
		if err != nil || cidr.String() != raw || cidr.IP.To4() == nil {
			return errors.New("allowed networks must be canonical IPv4 CIDRs")
		}
	}
	return nil
}
func rfSyslogRootAllowed(p string) bool {
	if p == rfSyslogDefaultRoot {
		return true
	}
	if filepath.Clean(p) != p || strings.ContainsAny(p, "\n\r\\\x00\"'") {
		return false
	}
	// A dedicated directory under an Entware USB mount, never arbitrary paths.
	parts := strings.Split(p, "/")
	return len(parts) == 5 && parts[0] == "" && parts[1] == "opt" && parts[2] == "mnt" && parts[3] != "" && parts[3] != "." && parts[3] != ".." && parts[4] == "routerforge-syslog" && regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`).MatchString(parts[3])
}
func rfSyslogHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func rfSyslogReadConfig() ([]byte, error) {
	st, err := os.Lstat(rfSyslogConfig)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > 512*1024 {
		return nil, errors.New("syslog config is not a safe regular file")
	}
	return os.ReadFile(rfSyslogConfig)
}
func rfSyslogOwned(raw []byte) bool {
	return bytes.HasPrefix(raw, []byte("@version: current\n"+rfSyslogMarker+"\n"))
}
func rfSyslogStored() (rfSyslogSettings, error) {
	raw, err := os.ReadFile(rfSyslogMeta)
	if errors.Is(err, os.ErrNotExist) {
		return rfSyslogDefaults(), nil
	}
	if err != nil {
		return rfSyslogSettings{}, err
	}
	if len(raw) > 8192 {
		return rfSyslogSettings{}, errors.New("oversized Syslog metadata")
	}
	var s rfSyslogSettings
	if err = json.Unmarshal(raw, &s); err != nil {
		return s, err
	}
	return s, rfSyslogValidate(s)
}
func rfSyslogPackage() (bool, string) {
	for _, p := range readPackages() {
		if p.Name == "syslog-ng" && strings.HasSuffix(strings.TrimSpace(p.Status), "installed") {
			return true, p.Version
		}
	}
	return false, ""
}
func rfSyslogService() (string, bool) {
	for _, s := range readServices() {
		if strings.EqualFold(s.Name, "syslog-ng") {
			return s.ID, s.Running
		}
	}
	return "", false
}
func rfSyslogBinary() string {
	for _, p := range []string{"/opt/sbin/syslog-ng", "/opt/bin/syslog-ng"} {
		if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() && st.Mode().Perm()&0111 != 0 {
			return p
		}
	}
	return ""
}
func rfSyslogStatus(w http.ResponseWriter, r *http.Request) {
	present, version := rfSyslogPackage()
	service, running := rfSyslogService()
	raw, err := rfSyslogReadConfig()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	managed := len(raw) > 0 && rfSyslogOwned(raw)
	s, metaErr := rfSyslogStored()
	if metaErr != nil {
		s = rfSyslogDefaults()
	}
	rootReady, reason := rfSyslogStorageReady(s.Root)
	writeJSON(w, 200, map[string]any{"installed": present, "version": version, "binary_ready": rfSyslogBinary() != "", "service_id": service, "running": running,
		"config_exists": len(raw) > 0, "managed": managed, "config_sha256": rfSyslogHash(raw), "config_path": rfSyslogConfig,
		"settings": s, "settings_error": rfSyslogErr(metaErr), "storage_ready": rootReady, "storage_error": reason,
		"requires_adoption": len(raw) > 0 && !managed, "managed_root": s.Root, "mutation_api": true})
}

// rfSyslogPreview bounds the response and does not split a UTF-8 code point.
// This is a read-only view of the exact existing syslog-ng.conf, not a parser.
func rfSyslogPreview(raw []byte, limit int) (string, bool) {
	if len(raw) <= limit {
		return string(raw), false
	}
	if limit <= 0 {
		return "", len(raw) > 0
	}
	prefix := raw[:limit]
	for len(prefix) > 0 && !utf8.Valid(prefix) {
		prefix = prefix[:len(prefix)-1]
	}
	return string(prefix), true
}

func rfSyslogConfigPreview(w http.ResponseWriter, r *http.Request) {
	raw, err := rfSyslogReadConfig()
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "cannot inspect existing syslog-ng configuration"})
		return
	}
	if raw == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "syslog-ng configuration does not exist"})
		return
	}
	if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "existing syslog-ng configuration is not UTF-8 text"})
		return
	}
	preview, truncated := rfSyslogPreview(raw, 64*1024)
	writeJSON(w, http.StatusOK, map[string]any{
		"path": rfSyslogConfig, "content": preview, "total_bytes": len(raw),
		"truncated": truncated, "sha256": rfSyslogHash(raw), "managed": rfSyslogOwned(raw),
		"read_only": true,
	})
}

func rfSyslogErr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func rfSyslogStorageReady(root string) (bool, string) {
	if !rfSyslogRootAllowed(root) {
		return false, "unsafe storage path"
	}
	parent := filepath.Dir(root)
	p, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return false, "storage parent is not mounted/available"
	}
	info, err := os.Stat(p)
	if err != nil || !info.IsDir() {
		return false, "storage parent is not a directory"
	}
	mounts, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false, "mount table unavailable"
	}
	mounted := false
	maxLen := 0
	fstype := ""
	for _, line := range strings.Split(string(mounts), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mp := strings.ReplaceAll(fields[1], `\040`, " ")
		if (p == mp || strings.HasPrefix(p, strings.TrimRight(mp, "/")+"/")) && len(mp) > maxLen {
			maxLen = len(mp)
			fstype = fields[2]
		}
	}
	// Never fall back to the router root filesystem when a USB disappears.
	if maxLen > 1 && fstype != "tmpfs" && fstype != "overlay" && fstype != "squashfs" && fstype != "rootfs" && fstype != "proc" && fstype != "sysfs" {
		mounted = true
	}
	if !mounted {
		return false, "storage is not on a persistent dedicated mount"
	}
	if st, err := os.Lstat(root); err == nil && (!st.IsDir() || st.Mode()&os.ModeSymlink != 0) {
		return false, "managed log root must be a real directory"
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, "log root inaccessible"
	}
	return true, ""
}

func rfSyslogConfigText(s rfSyslogSettings) string {
	var b strings.Builder
	b.WriteString("@version: current\n" + rfSyslogMarker + "\n@include \"scl.conf\"\n")
	b.WriteString("options { chain_hostnames(no); keep_hostname(yes); create_dirs(yes); log_fifo_size(256); log_msg_size(4096); stats(freq(0)); };\n")
	if s.UDP || s.TCP {
		b.WriteString("source s_rf_net {\n")
		for _, transport := range []struct {
			yes  bool
			name string
		}{{s.UDP, "udp"}, {s.TCP, "tcp"}} {
			if transport.yes {
				fmt.Fprintf(&b, "  network(ip(\"%s\") port(%d) transport(\"%s\"));\n", s.Bind, s.Port, transport.name)
			}
		}
		b.WriteString("};\nfilter f_rf_allowed { ")
		for i, cidr := range s.Allowed {
			if i > 0 {
				b.WriteString(" or ")
			}
			fmt.Fprintf(&b, "netmask(\"%s\")", cidr)
		}
		b.WriteString("; };\n")
		fmt.Fprintf(&b, "destination d_rf_remote { file(\"%s/${YEAR}/${MONTH}/${DAY}/syslog-${SOURCEIP}.log\" perm(0640) create_dirs(yes)); };\n", s.Root)
		b.WriteString("log { source(s_rf_net); filter(f_rf_allowed); destination(d_rf_remote); };\n")
	}
	if s.Local {
		b.WriteString("# Explicit opt-in only: /dev/log might already belong to another logger.\n")
		b.WriteString("source s_rf_local { unix-dgram(\"/dev/log\"); };\n")
		fmt.Fprintf(&b, "destination d_rf_local { file(\"%s/${YEAR}/${MONTH}/${DAY}/syslog-entware.log\" perm(0640) create_dirs(yes)); };\n", s.Root)
		b.WriteString("log { source(s_rf_local); destination(d_rf_local); };\n")
	}
	return b.String()
}

type rfSyslogApplyReq struct {
	Settings rfSyslogSettings `json:"settings"`
	Expected string           `json:"expected_sha256"`
	Adopt    bool             `json:"adopt_existing"`
	Confirm  string           `json:"confirm"`
}

func rfSyslogApply(w http.ResponseWriter, r *http.Request) {
	var req rfSyslogApplyReq
	if err := decodeMutationJSON(w, r, &req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid request"})
		return
	}
	if req.Confirm != "APPLY SYSLOG" {
		writeJSON(w, 409, map[string]any{"error": "exact confirmation required"})
		return
	}
	if err := rfSyslogValidate(req.Settings); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	if req.Settings.Local && !req.Adopt {
		writeJSON(w, 409, map[string]any{"error": "local /dev/log capture requires explicit adoption"})
		return
	}
	rfSyslogMu.Lock()
	defer rfSyslogMu.Unlock()
	installed, _ := rfSyslogPackage()
	if !installed {
		writeJSON(w, 409, map[string]any{"error": "syslog-ng is not installed"})
		return
	}
	binary := rfSyslogBinary()
	if binary == "" {
		writeJSON(w, 409, map[string]any{"error": "syslog-ng executable not found"})
		return
	}
	ready, reason := rfSyslogStorageReady(req.Settings.Root)
	if !ready {
		writeJSON(w, 409, map[string]any{"error": reason})
		return
	}
	previous, err := rfSyslogReadConfig()
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	if len(req.Expected) != 64 || req.Expected != rfSyslogHash(previous) {
		writeJSON(w, 409, map[string]any{"error": "configuration changed; refresh before applying"})
		return
	}
	if len(previous) > 0 && !rfSyslogOwned(previous) && !req.Adopt {
		writeJSON(w, 409, map[string]any{"error": "existing unmanaged config: explicit adoption required"})
		return
	}
	content := []byte(rfSyslogConfigText(req.Settings))
	temp, err := os.CreateTemp("/opt/etc", ".routerforge-syslog-*.conf")
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "cannot create candidate configuration"})
		return
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(content); err != nil {
		temp.Close()
		writeJSON(w, 500, map[string]any{"error": "candidate write failed"})
		return
	}
	if err = temp.Chmod(0600); err != nil {
		temp.Close()
		writeJSON(w, 500, map[string]any{"error": "candidate permissions failed"})
		return
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		writeJSON(w, 500, map[string]any{"error": "candidate sync failed"})
		return
	}
	if err = temp.Close(); err != nil {
		writeJSON(w, 500, map[string]any{"error": "candidate close failed"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	output, err := safety.RunCommand(ctx, 16<<10, binary, "-s", "-f", temp.Name())
	if err != nil {
		msg := string(output)
		if len(msg) > 2048 {
			msg = msg[:2048]
		}
		writeJSON(w, 409, map[string]any{"error": "syslog-ng syntax check failed", "detail": msg})
		return
	}
	// Preserve exactly the previous original config before touching it.
	backup := ""
	if len(previous) > 0 {
		if err = os.MkdirAll(rfSyslogBackup, 0700); err != nil {
			writeJSON(w, 500, map[string]any{"error": "cannot create backup directory"})
			return
		}
		backup = filepath.Join(rfSyslogBackup, "syslog-ng."+time.Now().UTC().Format("20060102T150405.000000000")+"."+rfSyslogHash(previous)[:12]+".conf")
		bf, e := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			writeJSON(w, 500, map[string]any{"error": "cannot create safety backup"})
			return
		}
		_, e = bf.Write(previous)
		if e == nil {
			e = bf.Sync()
		}
		ce := bf.Close()
		if e == nil {
			e = ce
		}
		if e != nil {
			os.Remove(backup)
			writeJSON(w, 500, map[string]any{"error": "safety backup failed"})
			return
		}
	}
	if err = os.Rename(temp.Name(), rfSyslogConfig); err != nil {
		writeJSON(w, 500, map[string]any{"error": "atomic config publish failed", "backup": backup})
		return
	}
	if err = os.MkdirAll(filepath.Dir(rfSyslogMeta), 0700); err == nil {
		var raw []byte
		raw, err = json.Marshal(req.Settings)
		if err == nil {
			err = rfSyslogAtomicMeta(append(raw, '\n'))
		}
	}
	if err != nil {
		// Fail-safe rollback: preserve old state, no restart on failure.
		if len(previous) > 0 {
			_ = rfSyslogRestoreConfig(previous)
		} else {
			_ = os.Remove(rfSyslogConfig)
		}
		writeJSON(w, 500, map[string]any{"error": "settings persist failed; previous config restored", "backup": backup})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "backup": backup, "config_sha256": rfSyslogHash(content), "restart_required": true, "service_untouched": true})
}
func rfSyslogAtomicMeta(data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(rfSyslogMeta), ".syslog-meta-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), rfSyslogMeta)
}
func rfSyslogRestoreConfig(data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(rfSyslogConfig), ".syslog-rollback-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), rfSyslogConfig)
}

type rfSyslogEntry struct {
	Date     string    `json:"date"`
	Source   string    `json:"source"`
	Filename string    `json:"filename"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

func rfSyslogFiles(s rfSyslogSettings) ([]rfSyslogEntry, error) {
	var out []rfSyslogEntry
	roots, err := os.ReadDir(s.Root)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, year := range roots {
		if !year.IsDir() || !regexp.MustCompile(`^\d{4}$`).MatchString(year.Name()) {
			continue
		}
		months, _ := os.ReadDir(filepath.Join(s.Root, year.Name()))
		for _, month := range months {
			if !month.IsDir() || !regexp.MustCompile(`^(0[1-9]|1[0-2])$`).MatchString(month.Name()) {
				continue
			}
			days, _ := os.ReadDir(filepath.Join(s.Root, year.Name(), month.Name()))
			for _, day := range days {
				if !day.IsDir() || !regexp.MustCompile(`^([0-2][0-9]|3[01])$`).MatchString(day.Name()) {
					continue
				}
				date := year.Name() + "-" + month.Name() + "-" + day.Name()
				if _, err := time.Parse("2006-01-02", date); err != nil {
					continue
				}
				files, _ := os.ReadDir(filepath.Join(s.Root, year.Name(), month.Name(), day.Name()))
				for _, entry := range files {
					if !rfSyslogFile.MatchString(entry.Name()) || entry.Type()&os.ModeSymlink != 0 {
						continue
					}
					info, e := entry.Info()
					if e != nil || !info.Mode().IsRegular() {
						continue
					}
					out = append(out, rfSyslogEntry{Date: date, Source: strings.TrimSuffix(strings.TrimPrefix(entry.Name(), "syslog-"), ".log"), Filename: entry.Name(), Size: info.Size(), Modified: info.ModTime()})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Modified.Equal(out[j].Modified) {
			return out[i].Filename < out[j].Filename
		}
		return out[i].Modified.After(out[j].Modified)
	})
	if len(out) > 5000 {
		out = out[:5000]
	}
	return out, nil
}
func rfSyslogManagedRoot() (rfSyslogSettings, error) {
	raw, err := rfSyslogReadConfig()
	if err != nil {
		return rfSyslogSettings{}, err
	}
	if !rfSyslogOwned(raw) {
		return rfSyslogSettings{}, errors.New("syslog-ng config is not managed by RouterForge")
	}
	s, err := rfSyslogStored()
	if err != nil {
		return rfSyslogSettings{}, err
	}
	ready, reason := rfSyslogStorageReady(s.Root)
	if !ready {
		return rfSyslogSettings{}, errors.New(reason)
	}
	return s, nil
}
func rfSyslogList(w http.ResponseWriter, r *http.Request) {
	s, err := rfSyslogManagedRoot()
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	files, err := rfSyslogFiles(s)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "cannot list syslog files"})
		return
	}
	if len(files) > 250 {
		files = files[:250]
	}
	writeJSON(w, 200, map[string]any{"files": files, "root": s.Root})
}
func rfSyslogSafeFile(s rfSyslogSettings, query map[string][]string) (string, error) {
	date, source := "", ""
	if v := query["date"]; len(v) == 1 {
		date = v[0]
	}
	if v := query["source"]; len(v) == 1 {
		source = v[0]
	}
	if !rfSyslogDate.MatchString(date) {
		return "", errors.New("invalid date")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", err
	}
	name := "syslog-" + source + ".log"
	if !rfSyslogFile.MatchString(name) {
		return "", errors.New("invalid source")
	}
	parts := strings.Split(date, "-")
	candidate := filepath.Join(s.Root, parts[0], parts[1], parts[2], name)
	info, err := os.Lstat(candidate)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("not a regular log file")
	}
	return candidate, nil
}
func rfSyslogReadTail(path string, limit int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	const maxRead int64 = 256 * 1024
	offset := st.Size() - maxRead
	if offset < 0 {
		offset = 0
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxRead))
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if i := bytes.IndexByte(raw, '\n'); i >= 0 {
			raw = raw[i+1:]
		} else {
			raw = nil
		}
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(raw) == 0 {
		lines = nil
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return lines, nil
}
func rfSyslogTail(w http.ResponseWriter, r *http.Request) {
	s, err := rfSyslogManagedRoot()
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	p, err := rfSyslogSafeFile(s, r.URL.Query())
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid or missing journal"})
		return
	}
	count, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	if count < 1 || count > 500 {
		count = 200
	}
	lines, err := rfSyslogReadTail(p, count)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "cannot read journal"})
		return
	}
	writeJSON(w, 200, map[string]any{"lines": lines, "count": len(lines), "date": r.URL.Query().Get("date"), "source": r.URL.Query().Get("source")})
}
func rfSyslogExport(w http.ResponseWriter, r *http.Request) {
	s, err := rfSyslogManagedRoot()
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	p, err := rfSyslogSafeFile(s, r.URL.Query())
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid journal"})
		return
	}
	f, err := os.Open(p)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "cannot open journal"})
		return
	}
	defer f.Close()
	if st, e := f.Stat(); e != nil || !st.Mode().IsRegular() || st.Size() > 100<<20 {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "journal export exceeds 100 MB limit"})
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(p)+`"`)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	io.Copy(w, io.LimitReader(f, 100<<20))
}

// Retention is confined to previously RouterForge-managed dated journal files.
// Current-day files and foreign filenames are NEVER deleted by the scheduler.
func rfSyslogRetainOnce(now time.Time) error {
	rfSyslogMu.Lock()
	defer rfSyslogMu.Unlock()
	s, err := rfSyslogManagedRoot()
	if err != nil {
		return err
	}
	ready, _ := rfSyslogStorageReady(s.Root)
	if !ready {
		return errors.New("syslog storage not mounted")
	}
	files, err := rfSyslogFiles(s)
	if err != nil {
		return err
	}
	cutoff := now.Local().AddDate(0, 0, -s.KeepDays).Format("2006-01-02")
	today := now.Local().Format("2006-01-02")
	var size int64
	for _, f := range files {
		size += f.Size
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Date == files[j].Date {
			return files[i].Modified.Before(files[j].Modified)
		}
		return files[i].Date < files[j].Date
	})
	limit := int64(s.MaxMB) * 1024 * 1024
	for _, file := range files {
		// syslog-ng YEAR/MONTH/DAY macros use router-local dates.
		// Never delete current local-day files, even across UTC rollover.
		if file.Date >= today {
			continue
		}
		if file.Date >= cutoff && size <= limit {
			continue
		}
		parts := strings.Split(file.Date, "-")
		p := filepath.Join(s.Root, parts[0], parts[1], parts[2], file.Filename)
		info, e := os.Lstat(p)
		if e != nil || !info.Mode().IsRegular() || info.Size() != file.Size {
			continue
		}
		if err := os.Remove(p); err != nil {
			return err
		}
		size -= file.Size
	}
	return nil
}
func rfSyslogRetentionLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		_ = rfSyslogRetainOnce(time.Now())
	}
}
func registerAdminSyslogRoutes(mux *http.ServeMux) {
	// All reads require the same root-session header as Admin file operations.
	mux.HandleFunc("/v1/syslog/status", adminFileAccessOnly(false, rfSyslogStatus))
	mux.HandleFunc("/v1/syslog/config", adminFileAccessOnly(false, rfSyslogConfigPreview))
	mux.HandleFunc("/v1/syslog/logs", adminFileAccessOnly(false, rfSyslogList))
	mux.HandleFunc("/v1/syslog/tail", adminFileAccessOnly(false, rfSyslogTail))
	mux.HandleFunc("/v1/syslog/export", adminFileAccessOnly(true, rfSyslogExport))
	mux.HandleFunc("/v1/syslog/apply", mutationOnly(rfSyslogApply))
}
