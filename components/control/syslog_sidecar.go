package main

// P45.3: non-destructive sidecar orchestration. Existing main configuration is
// kept byte-for-byte except for one explicit include line added on first apply.
// No syslog-ng process or router firmware service is restarted automatically.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	rfSyslogSidecarPath   = "/opt/etc/routerforge/syslog.d/routerforge.conf"
	rfSyslogIncludeLine   = `@include "/opt/etc/routerforge/syslog.d/routerforge.conf"`
	rfSyslogSidecarMarker = "# ROUTERFORGE SYSLOG SIDECAR P45.3 - DO NOT EDIT BY HAND"
)

var rfSyslogSourcesRE = regexp.MustCompile(`(?m)^\s*source\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{`)
var rfSyslogUnmanagedListenerRE = regexp.MustCompile(`(?m)\b(udp|tcp|network|network_localhost|syslog|default-network-drivers)\s*\(`)
var rfSyslogEntwarePrograms = []string{
	"syslog-ng", "dropbear", "cron", "crond", "curl", "wget", "tcpdump", "ipset", "iptables", "nginx", "python", "sh", "sshd", "samba", "dnsmasq", "mosquitto", "openvpn", "wireguard", "xray", "routerforge",
}

func rfSyslogReadSidecar() ([]byte, error) {
	st, err := os.Lstat(rfSyslogSidecarPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > 256*1024 {
		return nil, errors.New("unsafe syslog sidecar file")
	}
	return os.ReadFile(rfSyslogSidecarPath)
}
func rfSyslogSidecarAttached(parent []byte) bool {
	if bytes.Count(parent, []byte(rfSyslogIncludeLine)) != 1 {
		return false
	}
	side, err := rfSyslogReadSidecar()
	return err == nil && bytes.HasPrefix(side, []byte(rfSyslogSidecarMarker+"\n"))
}
func rfSyslogSources(parent []byte) []string {
	clean := rfSyslogStripComments(string(parent))
	matches := rfSyslogSourcesRE.FindAllStringSubmatch(clean, -1)
	unique := map[string]bool{}
	names := []string{}
	for _, m := range matches {
		if !unique[m[1]] {
			unique[m[1]] = true
			names = append(names, m[1])
		}
	}
	sort.Strings(names)
	return names
}
func rfSyslogStripComments(raw string) string {
	var b strings.Builder
	// Ignore # only outside quoted strings. Preserve newlines for anchored source matching.
	inQuote := false
	escaped := false
	comment := false
	for _, c := range raw {
		if c == '\n' {
			comment = false
			escaped = false
			b.WriteRune(c)
			continue
		}
		if comment {
			continue
		}
		if escaped {
			escaped = false
			b.WriteRune(c)
			continue
		}
		if c == '\\' && inQuote {
			escaped = true
			b.WriteRune(c)
			continue
		}
		if c == '"' {
			inQuote = !inQuote
			b.WriteRune(c)
			continue
		}
		if c == '#' && !inQuote {
			comment = true
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}
func rfSyslogHasSource(sources []string, name string) bool {
	for _, s := range sources {
		if s == name {
			return true
		}
	}
	return false
}
func rfSyslogSnippet(s rfSyslogSettings) string {
	return rfSyslogSnippetEasy(s)
}

func rfSyslogParentWithInclude(parent []byte, include string) ([]byte, bool, error) {
	if !strings.HasPrefix(strings.TrimSpace(string(parent)), "@version:") && !strings.Contains(string(parent), "\n@version:") {
		return nil, false, errors.New("existing config is missing @version; explicit manual fix required")
	}
	count := bytes.Count(parent, []byte(rfSyslogIncludeLine))
	if count > 1 {
		return nil, false, errors.New("duplicate RouterForge includes in parent config")
	}
	if count == 1 {
		if include == rfSyslogIncludeLine {
			return append([]byte(nil), parent...), false, nil
		}
		return bytes.Replace(parent, []byte(rfSyslogIncludeLine), []byte(include), 1), false, nil
	}
	out := append([]byte(nil), parent...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, []byte("\n# RouterForge Syslog Manager: optional managed sidecar\n"+include+"\n")...)
	return out, true, nil
}
func rfSyslogPreflight(s rfSyslogSettings, parent, existing []byte) (string, []byte, string, error) {
	if err := rfSyslogValidate(s); err != nil {
		return "", nil, "", err
	}
	if parent == nil {
		return "", nil, "", errors.New("main syslog-ng.conf does not exist; install/configure syslog-ng first")
	}
	if len(existing) > 0 && !bytes.HasPrefix(existing, []byte(rfSyslogSidecarMarker+"\n")) {
		return "", nil, "", errors.New("existing sidecar belongs to another application; refusing overwrite")
	}
	if rfSyslogOwned(parent) {
		return "", nil, "", errors.New("legacy full-file RouterForge config found; migration requires separate action")
	}
	if err := rfSyslogCheckSource(s, parent); err != nil {
		return "", nil, "", err
	}
	sources := rfSyslogSources(parent)
	if (s.UDP || s.TCP) && s.RemoteSource != "" && !rfSyslogHasSource(sources, s.RemoteSource) {
		return "", nil, "", errors.New("selected remote source is not defined in main config")
	}
	if s.Local && !rfSyslogHasSource(sources, s.LocalSource) {
		return "", nil, "", errors.New("selected local source is not defined in main config")
	}
	// Conservative collision boundary: never bind a second listener where the
	// foreign config already has network input. Reuse an existing source.
	if (s.UDP || s.TCP) && s.RemoteSource == "" && rfSyslogUnmanagedListenerRE.MatchString(rfSyslogStripComments(string(parent))) {
		return "", nil, "", errors.New("existing config has network listener(s); select existing remote source to avoid port collisions")
	}
	if s.RemoteSource == "s_routerforge_net" || s.LocalSource == "s_routerforge_net" {
		return "", nil, "", errors.New("reserved RouterForge source name")
	}
	snippet := rfSyslogSnippet(s)
	out, action, err := func() ([]byte, string, error) {
		p, appendNeeded, e := rfSyslogParentWithInclude(parent, rfSyslogIncludeLine)
		if appendNeeded {
			return p, "append-include", e
		}
		return p, "unchanged", e
	}()
	if err != nil {
		return "", nil, "", err
	}
	if len(existing) > 0 && bytes.Equal(existing, []byte(snippet)) && action == "unchanged" {
		action = "unchanged"
	}
	return snippet, out, action, nil
}
func rfSyslogPlanHash(s rfSyslogSettings, parent, existing []byte) string {
	raw, _ := json.Marshal(s)
	return rfSyslogHash(append(append(append(raw, 0), parent...), append([]byte{0}, existing...)...))
}

type rfSyslogPlanRequest struct {
	Settings rfSyslogSettings `json:"settings"`
}

func rfSyslogPlanHandler(w http.ResponseWriter, r *http.Request) {
	var req rfSyslogPlanRequest
	if err := decodeMutationJSON(w, r, &req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid preflight request"})
		return
	}
	parent, err := rfSyslogReadConfig()
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	existing, err := rfSyslogReadSidecar()
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	snippet, _, action, err := rfSyslogPreflight(req.Settings, parent, existing)
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error(), "source_names": rfSyslogSources(parent)})
		return
	}
	ready, reason := rfSyslogStorageReady(req.Settings.Root)
	if !ready {
		writeJSON(w, 409, map[string]any{"error": reason})
		return
	}
	installed, _ := rfSyslogPackage()
	if !installed || rfSyslogBinary() == "" {
		writeJSON(w, 409, map[string]any{"error": "syslog-ng package or binary missing"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "plan_sha256": rfSyslogPlanHash(req.Settings, parent, existing), "parent_sha256": rfSyslogHash(parent), "sidecar_sha256": rfSyslogHash(existing), "parent_action": action, "include_line": rfSyslogIncludeLine, "sidecar_path": rfSyslogSidecarPath, "sidecar_content": snippet, "sources": rfSyslogSources(parent), "requires_confirmation": true, "service_untouched": true, "storage_ready": ready})
}

type rfSyslogSidecarRequest struct {
	Settings        rfSyslogSettings `json:"settings"`
	Expected        string           `json:"expected_sha256"`
	ExpectedSidecar string           `json:"expected_sidecar_sha256"`
	PlanSHA         string           `json:"plan_sha256"`
	Confirm         string           `json:"confirm"`
}

func rfSyslogBackupRaw(path string, data []byte) error {
	if err := os.MkdirAll(rfSyslogBackup, 0700); err != nil {
		return err
	}
	name := filepath.Join(rfSyslogBackup, filepath.Base(path)+"."+time.Now().UTC().Format("20060102T150405.000000000")+"."+rfSyslogHash(data)[:12]+".bak")
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(name)
	}
	return err
}
func rfSyslogAtomic(path string, data []byte) error {
	parent := filepath.Dir(path)
	st, err := os.Lstat(path)
	if err == nil && !st.Mode().IsRegular() {
		return errors.New("non-regular configuration path")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.CreateTemp(parent, ".rf-syslog-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	mode := os.FileMode(0600)
	if st != nil {
		mode = st.Mode().Perm()
	}
	if err = f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if st != nil {
		if sys, ok := st.Sys().(*syscall.Stat_t); ok {
			if err = f.Chown(int(sys.Uid), int(sys.Gid)); err != nil {
				f.Close()
				return err
			}
		}
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func rfSyslogApplySidecar(w http.ResponseWriter, r *http.Request) {
	var req rfSyslogSidecarRequest
	if err := decodeMutationJSON(w, r, &req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid request"})
		return
	}
	if req.Confirm != "APPLY SYSLOG SIDECAR" {
		writeJSON(w, 409, map[string]any{"error": "exact consent required"})
		return
	}
	rfSyslogMu.Lock()
	defer rfSyslogMu.Unlock()
	installed, _ := rfSyslogPackage()
	binary := rfSyslogBinary()
	if !installed || binary == "" {
		writeJSON(w, 409, map[string]any{"error": "syslog-ng is not installed"})
		return
	}
	ready, reason := rfSyslogStorageReady(req.Settings.Root)
	if !ready {
		writeJSON(w, 409, map[string]any{"error": reason})
		return
	}
	original, err := rfSyslogReadConfig()
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	previous, err := rfSyslogReadSidecar()
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	if rfSyslogHash(original) != req.Expected || rfSyslogHash(previous) != req.ExpectedSidecar || rfSyslogPlanHash(req.Settings, original, previous) != req.PlanSHA {
		writeJSON(w, 409, map[string]any{"error": "config or plan changed; review again before applying"})
		return
	}
	snippet, parent, action, err := rfSyslogPreflight(req.Settings, original, previous)
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	if len(original) == 0 {
		writeJSON(w, 409, map[string]any{"error": "main config missing"})
		return
	}
	if err := os.MkdirAll(filepath.Dir(rfSyslogSidecarPath), 0700); err != nil {
		writeJSON(w, 500, map[string]any{"error": "cannot create sidecar directory"})
		return
	}
	// Validate full effective parent with a temporary include pointing to the
	// staged snippet. The real syslog-ng.conf and sidecar remain untouched.
	sf, err := os.CreateTemp(filepath.Dir(rfSyslogSidecarPath), ".rf-stage-*.conf")
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "cannot stage sidecar"})
		return
	}
	staged := sf.Name()
	defer os.Remove(staged)
	if _, err = io.WriteString(sf, snippet); err == nil {
		err = sf.Sync()
	}
	cerr := sf.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "cannot stage config"})
		return
	}
	candidate := bytes.Replace(parent, []byte(rfSyslogIncludeLine), []byte(`@include "`+staged+`"`), 1)
	pf, err := os.CreateTemp(filepath.Dir(rfSyslogConfig), ".rf-syslog-validation-*.conf")
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "cannot stage parent"})
		return
	}
	stagedParent := pf.Name()
	defer os.Remove(stagedParent)
	if _, err = pf.Write(candidate); err == nil {
		err = pf.Sync()
	}
	cerr = pf.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "cannot write validation config"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	output, err := safety.RunCommand(ctx, 16<<10, binary, "-s", "-f", stagedParent)
	if err != nil {
		detail := string(output)
		if len(detail) > 2000 {
			detail = detail[:2000]
		}
		writeJSON(w, 409, map[string]any{"error": "syslog-ng syntax preflight rejected", "detail": detail})
		return
	}
	if err := rfSyslogPrepareFolders(req.Settings.Root, time.Now()); err != nil {
		writeJSON(w, 409, map[string]any{"error": "cannot prepare mounted log folders: " + err.Error()})
		return
	}
	// Check external config has not changed between preview and publication.
	now, err := rfSyslogReadConfig()
	if err != nil || rfSyslogHash(now) != req.Expected {
		writeJSON(w, 409, map[string]any{"error": "parent changed during validation"})
		return
	}
	nowSide, err := rfSyslogReadSidecar()
	if err != nil || rfSyslogHash(nowSide) != req.ExpectedSidecar {
		writeJSON(w, 409, map[string]any{"error": "sidecar changed during validation"})
		return
	}
	if action != "unchanged" {
		if err = rfSyslogBackupRaw(rfSyslogConfig, original); err != nil {
			writeJSON(w, 500, map[string]any{"error": "parent safety backup failed"})
			return
		}
	}
	if previous != nil {
		if err = rfSyslogBackupRaw(rfSyslogSidecarPath, previous); err != nil {
			writeJSON(w, 500, map[string]any{"error": "sidecar safety backup failed"})
			return
		}
	}
	// Publish sidecar first; only then add the include to the parent.
	if err = rfSyslogAtomic(rfSyslogSidecarPath, []byte(snippet)); err != nil {
		writeJSON(w, 500, map[string]any{"error": "sidecar publication failed"})
		return
	}
	restoreSidecar := func() {
		if previous == nil {
			_ = os.Remove(rfSyslogSidecarPath)
		} else {
			_ = rfSyslogAtomic(rfSyslogSidecarPath, previous)
		}
	}
	if action != "unchanged" {
		if err = rfSyslogAtomic(rfSyslogConfig, parent); err != nil {
			restoreSidecar()
			writeJSON(w, 500, map[string]any{"error": "main include publication failed; sidecar restored"})
			return
		}
	}
	if err = os.MkdirAll(filepath.Dir(rfSyslogMeta), 0700); err == nil {
		var encoded []byte
		encoded, err = json.Marshal(req.Settings)
		if err == nil {
			err = rfSyslogAtomicMeta(append(encoded, '\n'))
		}
	}
	if err != nil {
		// Stop before runtime reload; restore both config files.
		if action != "unchanged" {
			_ = rfSyslogAtomic(rfSyslogConfig, original)
		}
		restoreSidecar()
		writeJSON(w, 500, map[string]any{"error": "metadata save failed; rolled back configs; no restart"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "parent_action": action, "sidecar_path": rfSyslogSidecarPath, "sidecar_sha256": rfSyslogHash([]byte(snippet)), "config_sha256": rfSyslogHash(parent), "restart_required": true, "service_untouched": true})
}
