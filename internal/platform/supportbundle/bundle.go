// Package supportbundle defines the small, allowlisted support archive format.
package supportbundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"strings"
	"time"
)

const MaxArchive = 512 << 10
const MaxEntry = 64 << 10
const readme = "RouterForge Support Bundle v2\nAllowlisted diagnostic metadata only. No raw logs or configuration.\nSources sampled independently; unavailable is not proof of failure.\nVerify each payload against manifest.json. Review before sharing.\n"
const Header = "X-RouterForge-Support-Observation"

var ErrInvalid = errors.New("invalid support bundle")
var versionRE = regexp.MustCompile(`^(dev|[0-9]{1,4}\.[0-9]{1,4}\.[0-9]{1,4}(-[0-9]{1,6}|-beta[.-]?[0-9]{1,6})?|[0-9]{1,4}~(dev|beta)\.r[0-9]{1,12}\.[a-f0-9]{12})$`)
var kernelRE = regexp.MustCompile(`^[0-9]{1,3}\.[0-9]{1,3}(\.[0-9]{1,4})?`)
var modelRE = regexp.MustCompile(`(?i)^(?:Keenetic(?: Ltd\.?| Limited)?[ ,]+)?(?:[A-Za-z][A-Za-z0-9 -]{0,40}[ (]+)?(KN-[0-9]{4})(?:\))?$`)
var packageIDs = []string{"core", "admin", "dns", "monitoring", "network-tools", "nfqws-manager", "antiscan-manager", "port-access-manager", "crowdsec-manager", "dns-router", "profiling"}
var probeIDs = []string{"admin", "dns", "monitoring", "network-tools", "nfqws-manager", "dns-router"}
var sections = []string{"system", "packages", "modules", "foundation", "logs"}

func PackageIDs() []string { return append([]string(nil), packageIDs...) }
func ProbeIDs() []string   { return append([]string(nil), probeIDs...) }
func Sections() []string   { return append([]string(nil), sections...) }
func known(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
func Selection(input []string) ([]string, error) {
	if len(input) == 0 || len(input) > len(sections) {
		return nil, ErrInvalid
	}
	seen := map[string]bool{}
	for _, s := range input {
		if !known(sections, s) || seen[s] {
			return nil, ErrInvalid
		}
		seen[s] = true
	}
	out := []string{}
	for _, s := range sections {
		if seen[s] {
			out = append(out, s)
		}
	}
	return out, nil
}
func Modules(input []string) ([]string, error) {
	if len(input) > len(probeIDs) {
		return nil, ErrInvalid
	}
	if len(input) == 0 {
		return ProbeIDs(), nil
	}
	seen := map[string]bool{}
	for _, s := range input {
		if !known(probeIDs, s) || seen[s] {
			return nil, ErrInvalid
		}
		seen[s] = true
	}
	out := []string{}
	for _, s := range probeIDs {
		if seen[s] {
			out = append(out, s)
		}
	}
	return out, nil
}
func SelectedModules(selected, input []string) ([]string, error) {
	if !known(selected, "modules") {
		if len(input) != 0 {
			return nil, ErrInvalid
		}
		return []string{}, nil
	}
	return Modules(input)
}
func Version(s string) string {
	if versionRE.MatchString(s) {
		return s
	}
	return "unknown"
}
func Kernel(s string) string {
	if m := kernelRE.FindString(s); m != "" {
		return m
	}
	return "unknown"
}
func Model(s string) string {
	m := modelRE.FindStringSubmatch(strings.TrimSpace(strings.TrimRight(s, "\x00")))
	if len(m) == 2 {
		return strings.ToUpper(m[1])
	}
	return "unknown"
}
func Architecture(s string) string {
	switch s {
	case "arm64", "arm", "mips", "mipsle", "amd64", "386":
		return s
	}
	return "unknown"
}
func Hash(b []byte) string { d := sha256.Sum256(b); return hex.EncodeToString(d[:]) }

// No raw event messages, identifiers, labels, context or arbitrary keys cross this boundary.
type Observation struct {
	Available   bool     `json:"available"`
	Info        int      `json:"info"`
	Warning     int      `json:"warning"`
	Critical    int      `json:"critical"`
	CPUPercent  *float64 `json:"cpu_percent"`
	CoreVersion string   `json:"core_version"`
}

func ParseObservation(s string) Observation {
	var o Observation
	if len(s) > 1024 {
		return Observation{CoreVersion: "unknown"}
	}
	d := json.NewDecoder(strings.NewReader(s))
	d.DisallowUnknownFields()
	if d.Decode(&o) != nil {
		return Observation{CoreVersion: "unknown"}
	}
	var x any
	if d.Decode(&x) == nil || !json.Valid([]byte(s)) {
		return Observation{CoreVersion: "unknown"}
	}
	if o.Info < 0 || o.Warning < 0 || o.Critical < 0 || o.Info > 512 || o.Warning > 512 || o.Critical > 512 || o.Info+o.Warning+o.Critical > 512 || (!o.Available && (o.Info+o.Warning+o.Critical != 0)) {
		return Observation{CoreVersion: "unknown"}
	}
	if o.CPUPercent != nil && (*o.CPUPercent < 0 || *o.CPUPercent > 100 || math.IsNaN(*o.CPUPercent) || math.IsInf(*o.CPUPercent, 0)) {
		o.CPUPercent = nil
	}
	o.CoreVersion = Version(o.CoreVersion)
	return o
}

type Package struct {
	ID        string `json:"id"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
}

func Packages(raw []byte) []Package {
	result := map[string]Package{}
	for _, block := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n\n") {
		var name, version, status string
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "Package: "):
				name = strings.TrimSpace(strings.TrimPrefix(line, "Package: "))
			case strings.HasPrefix(line, "Version: "):
				version = strings.TrimSpace(strings.TrimPrefix(line, "Version: "))
			case strings.HasPrefix(line, "Status: "):
				status = strings.TrimSpace(strings.TrimPrefix(line, "Status: "))
			}
		}
		id := strings.TrimPrefix(name, "routerforge-")
		if name == "routerforge-"+id && known(packageIDs, id) {
			fields := strings.Fields(status)
			installed := len(fields) == 3 && fields[2] == "installed"
			result[id] = Package{id, installed, Version(version)}
		}
	}
	out := []Package{}
	for _, id := range packageIDs {
		if v, ok := result[id]; ok {
			out = append(out, v)
		}
	}
	return out
}

type Health struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Version   string `json:"version"`
	Running   *bool  `json:"running,omitempty"`
	Available *bool  `json:"available,omitempty"`
	Rules     *int   `json:"rules,omitempty"`
	Upstreams *int   `json:"upstreams,omitempty"`
}

func ProjectHealth(id string, raw []byte) Health {
	out := Health{ID: id, State: "unavailable", Version: "unknown"}
	if !known(probeIDs, id) || len(raw) > MaxEntry {
		return out
	}
	var p struct {
		OK            *bool  `json:"ok"`
		Status        string `json:"status"`
		Version       string `json:"version"`
		TargetRunning *bool  `json:"target_running"`
		Available     *bool  `json:"available"`
		DNSListener   *bool  `json:"dns_listener"`
		Rules         *int   `json:"rules"`
		Upstreams     *int   `json:"upstreams"`
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' || json.Unmarshal(raw, &p) != nil {
		return out
	}
	out.State = "responded"
	if p.OK != nil {
		if *p.OK {
			out.State = "ok"
		} else {
			out.State = "degraded"
		}
	} else {
		switch p.Status {
		case "ok", "healthy":
			out.State = "ok"
		case "degraded", "warning", "error", "inactive":
			out.State = p.Status
		case "independent-dns", "shadow-loopback":
			out.State = "active"
		}
	}
	out.Version = Version(p.Version)
	if id == "nfqws-manager" {
		out.Running = p.TargetRunning
		out.Available = p.Available
	}
	if id == "dns-router" {
		out.Running = p.DNSListener
		if p.Rules != nil && *p.Rules >= 0 && *p.Rules <= 100000 {
			out.Rules = p.Rules
		}
		if p.Upstreams != nil && *p.Upstreams >= 0 && *p.Upstreams <= 10000 {
			out.Upstreams = p.Upstreams
		}
	}
	return out
}

type LogSummary struct {
	SampleBytes    int  `json:"sample_bytes"`
	Lines          int  `json:"lines"`
	Error          int  `json:"error"`
	Warning        int  `json:"warning"`
	Info           int  `json:"info"`
	Other          int  `json:"other"`
	Timeout        int  `json:"timeout_mentions"`
	Conflict       int  `json:"conflict_mentions"`
	Permission     int  `json:"permission_mentions"`
	SuppressedText bool `json:"suppressed_text"`
}

func SummarizeLogs(raw []byte) LogSummary {
	s := LogSummary{SampleBytes: len(raw), SuppressedText: true}
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		s.Lines++
		l := strings.ToLower(string(line))
		switch {
		case strings.Contains(l, "error") || strings.Contains(l, "fatal") || strings.Contains(l, "panic"):
			s.Error++
		case strings.Contains(l, "warn"):
			s.Warning++
		case strings.Contains(l, "info"):
			s.Info++
		default:
			s.Other++
		}
		if strings.Contains(l, "timeout") || strings.Contains(l, "timed out") {
			s.Timeout++
		}
		if strings.Contains(l, "conflict") || strings.Contains(l, "stale") {
			s.Conflict++
		}
		if strings.Contains(l, "permission denied") || strings.Contains(l, "unauthorized") {
			s.Permission++
		}
	}
	return s
}

type Source struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
	Name   string `json:"name,omitempty"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256,omitempty"`
}
type Entry struct {
	ID     string          `json:"id"`
	State  string          `json:"state"`
	Reason string          `json:"reason,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}
type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
	Privacy       string    `json:"privacy"`
	Selection     []string  `json:"selection"`
	Modules       []string  `json:"modules"`
	Sources       []Source  `json:"sources"`
	Excluded      []string  `json:"excluded"`
}

var reasons = []string{"NOT_AVAILABLE", "PARTIAL", "NOT_SELECTED", "NOT_COLLECTED", "TIMEOUT", "INVALID_SOURCE", "NO_LOG"}

func Build(now time.Time, selected, modules []string, entries []Entry) ([]byte, Manifest, error) {
	var m Manifest
	sel, e := Selection(selected)
	if e != nil {
		return nil, m, e
	}
	mods, e := SelectedModules(sel, modules)
	if e != nil {
		return nil, m, e
	}
	m = Manifest{2, now.UTC(), "allowlisted-metadata-only", sel, mods, []Source{}, []string{"raw_logs", "hostnames", "ip_addresses", "mac_addresses", "usernames", "process_arguments", "configuration_contents", "credentials", "keys", "tokens", "observer_ids", "object_ids", "mount_sources", "third_party_security_runtime"}}
	if len(entries) != len(sel) {
		return nil, m, ErrInvalid
	}
	byID := map[string]Entry{}
	for _, v := range entries {
		if !known(sel, v.ID) || byID[v.ID].ID != "" {
			return nil, m, ErrInvalid
		}
		byID[v.ID] = v
	}
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	total := 0
	add := func(name string, raw []byte) error {
		if len(raw) > MaxEntry || total+len(raw) > MaxArchive-8192 {
			return ErrInvalid
		}
		total += len(raw)
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		h.SetMode(0600)
		h.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
		w, e := z.CreateHeader(h)
		if e != nil {
			return e
		}
		_, e = w.Write(raw)
		return e
	}
	for _, id := range sel {
		v := byID[id]
		s := Source{ID: id, State: v.State, Reason: v.Reason}
		switch v.State {
		case "ok":
			if v.Reason != "" {
				return nil, m, ErrInvalid
			}
		case "partial", "skipped":
			if !known(reasons, v.Reason) {
				return nil, m, ErrInvalid
			}
		default:
			return nil, m, ErrInvalid
		}
		if v.State == "skipped" {
			if len(v.Data) != 0 {
				return nil, m, ErrInvalid
			}
		} else {
			if !json.Valid(v.Data) {
				return nil, m, ErrInvalid
			}
			s.Name = id + ".json"
			s.Bytes = len(v.Data)
			s.SHA256 = Hash(v.Data)
			if e = add(s.Name, v.Data); e != nil {
				return nil, m, e
			}
		}
		m.Sources = append(m.Sources, s)
	}
	raw, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return nil, m, e
	}
	if e = add("manifest.json", raw); e != nil {
		return nil, m, e
	}
	if e = add("README.txt", []byte(readme)); e != nil {
		return nil, m, e
	}
	if e = z.Close(); e != nil || b.Len() > MaxArchive {
		return nil, m, ErrInvalid
	}
	return b.Bytes(), m, nil
}

// Verify the staged bytes before they are published. Only expected entries and
// exact hashes/lengths are accepted. The manifest itself is covered by ZIP SHA.
func Verify(raw []byte, m Manifest) error {
	if len(raw) > MaxArchive || m.SchemaVersion != 2 {
		return ErrInvalid
	}
	r, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if e != nil {
		return e
	}
	expected := map[string]Source{}
	for _, s := range m.Sources {
		if s.Name != "" {
			if s.Name != s.ID+".json" || !known(sections, s.ID) || expected[s.Name].Name != "" {
				return ErrInvalid
			}
			expected[s.Name] = s
		}
	}
	manifest, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return e
	}
	seen := map[string]bool{}
	total := 0
	for _, f := range r.File {
		if seen[f.Name] || f.UncompressedSize64 > MaxEntry || f.Method != zip.Store || !f.Mode().IsRegular() {
			return ErrInvalid
		}
		seen[f.Name] = true
		b, e := f.Open()
		if e != nil {
			return e
		}
		data, e := io.ReadAll(io.LimitReader(b, MaxEntry+1))
		ce := b.Close()
		if e != nil || ce != nil || len(data) > MaxEntry || uint64(len(data)) != f.UncompressedSize64 {
			return ErrInvalid
		}
		total += len(data)
		if total > MaxArchive {
			return ErrInvalid
		}
		switch f.Name {
		case "manifest.json":
			if !bytes.Equal(data, manifest) {
				return ErrInvalid
			}
		case "README.txt":
			if !bytes.Equal(data, []byte(readme)) {
				return ErrInvalid
			}
		default:
			s, ok := expected[f.Name]
			if !ok || len(data) != s.Bytes || Hash(data) != s.SHA256 {
				return ErrInvalid
			}
		}
	}
	if !seen["manifest.json"] || !seen["README.txt"] || len(seen) != len(expected)+2 {
		return ErrInvalid
	}
	return nil
}

func JSON(v any) json.RawMessage {
	b, e := json.Marshal(v)
	if e != nil {
		return nil
	}
	return b
}
