package main

// RouterForge Syslog Manager: an explicit, opt-in configuration manager for
// Entware's syslog-ng. Reading this API never installs packages or changes files.

import (
	"bytes"
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
var rfSyslogFile = regexp.MustCompile(`^syslog-[\p{L}\p{N}_.:-]{1,100}\.log$`)
var rfSyslogSourceName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,63}$`)

type rfSyslogSettings struct {
	Bind         string            `json:"bind"`
	Port         int               `json:"port"`
	UDP          bool              `json:"udp"`
	TCP          bool              `json:"tcp"`
	Local        bool              `json:"local"`
	Allowed      []string          `json:"allowed"`
	Root         string            `json:"root"`
	KeepDays     int               `json:"keep_days"`
	MaxMB        int               `json:"max_mb"`
	RemoteSource string            `json:"remote_source"`
	LocalSource  string            `json:"local_source"`
	HeadIP       string            `json:"head_ip"`
	RotateHours  int               `json:"rotate_hours"`
	DeviceNames  map[string]string `json:"device_names"`
}

func rfSyslogDefaults() rfSyslogSettings {
	return rfSyslogSettings{
		Bind: "0.0.0.0", Port: 514, UDP: true, TCP: false, Local: false,
		Allowed: []string{"192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"},
		Root:    rfSyslogDefaultRoot, KeepDays: 30, MaxMB: 500, HeadIP: "192.168.10.1", RotateHours: 24, DeviceNames: map[string]string{},
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
	if err := rfSyslogValidateSimple(s); err != nil {
		return err
	}
	if s.RemoteSource != "" && !rfSyslogSourceName.MatchString(s.RemoteSource) {
		return errors.New("invalid existing remote source name")
	}
	if s.LocalSource != "" && !rfSyslogSourceName.MatchString(s.LocalSource) {
		return errors.New("invalid existing local source name")
	}
	if s.Local && s.LocalSource == "" {
		return errors.New("local events require an existing syslog-ng source; /dev/log is not re-opened")
	}
	if s.Local && net.ParseIP(s.HeadIP).To4() == nil {
		return errors.New("local router IPv4 address is required")
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
	return len(parts) == 5 && parts[0] == "" && (parts[1] == "opt" || parts[1] == "tmp") && parts[2] == "mnt" && parts[3] != "" && parts[3] != "." && parts[3] != ".." && parts[4] == "routerforge-syslog" && regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`).MatchString(parts[3])
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
	if s.RotateHours == 0 {
		s.RotateHours = 24
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
	managed := len(raw) > 0 && (rfSyslogOwned(raw) || rfSyslogSidecarAttached(raw))
	s, metaErr := rfSyslogStored()
	if metaErr != nil {
		s = rfSyslogDefaults()
	}
	rootReady, reason := rfSyslogStorageReady(s.Root)
	sidecar, _ := rfSyslogReadSidecar()
	writeJSON(w, 200, map[string]any{"installed": present, "version": version, "binary_ready": rfSyslogBinary() != "", "service_id": service, "running": running,
		"config_exists": len(raw) > 0, "managed": managed, "config_sha256": rfSyslogHash(raw), "config_path": rfSyslogConfig,
		"settings": s, "settings_error": rfSyslogErr(metaErr), "storage_ready": rootReady, "storage_error": reason,
		"requires_adoption": false, "managed_root": s.Root, "mutation_api": true,
		"source_names": rfSyslogSources(raw), "source_details": rfSyslogSourceDetails(raw), "storage_options": rfSyslogStorageOptions(), "sidecar_included": rfSyslogSidecarAttached(raw),
		"sidecar_sha256": rfSyslogHash(sidecar), "sidecar_path": rfSyslogSidecarPath,
		"mode": "non-destructive-sidecar"})
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
	return rfSyslogStorageCheck(root)
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
	rfSyslogApplySidecar(w, r)
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
	Display  string    `json:"display"`
	Interval string    `json:"interval,omitempty"`
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
					source := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), "syslog-"), ".log")
					display, period := rfSyslogDisplaySource(source, rfSyslogHours(s))
					out = append(out, rfSyslogEntry{Date: date, Source: source, Display: display, Interval: period, Filename: entry.Name(), Size: info.Size(), Modified: info.ModTime()})
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
	if !rfSyslogOwned(raw) && !rfSyslogSidecarAttached(raw) {
		return rfSyslogSettings{}, errors.New("syslog-ng has no active RouterForge log rules")
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
	if err := rfSyslogPrepareFolders(s.Root, now); err != nil {
		return err
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
	mux.HandleFunc("/v1/syslog/smb/status", adminFileAccessOnly(false, rfSyslogSMBStatus))
	mux.HandleFunc("/v1/syslog/smb/connect", mutationOnly(rfSyslogSMBConnect))
	mux.HandleFunc("/v1/syslog/smb/disconnect", mutationOnly(rfSyslogSMBDisconnect))
	go rfSyslogSMBReconnectLoop()
	mux.HandleFunc("/v1/syslog/plan", mutationOnly(rfSyslogPlanHandler))
	mux.HandleFunc("/v1/syslog/summary", adminFileAccessOnly(false, rfSyslogSummary))
}
