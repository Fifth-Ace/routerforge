// Package telemetry implements RouterForge's optional diagnostic reports and
// separate installation/presence accounting. No network request is ever made
// without an explicitly configured HTTPS collector endpoint.
package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	MaxPayload  = 2048
	DailyPeriod = 24 * time.Hour
)

var versionRE = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+_~\-]{0,49}$`)

type Settings struct {
	DiagnosticsEnabled bool `json:"diagnostics_enabled"`
}

type State struct {
	InstallationID       string    `json:"installation_id"`
	RegisterEventID      string    `json:"register_event_id"`
	Registered           bool      `json:"registered"`
	LastPresence         time.Time `json:"last_presence,omitempty"`
	PresenceEventID      string    `json:"presence_event_id,omitempty"`
	LastDiagnostic       time.Time `json:"last_diagnostic,omitempty"`
	LastDiagnosticSchema int       `json:"last_diagnostic_schema,omitempty"`
	DiagnosticEventID    string    `json:"diagnostic_event_id,omitempty"`
}

type Status struct {
	CollectorConfigured   bool      `json:"collector_configured"`
	RegistrationConfirmed bool      `json:"registration_confirmed"`
	DiagnosticsEnabled    bool      `json:"diagnostics_enabled"`
	LastPresence          time.Time `json:"last_presence,omitempty"`
	LastDiagnostic        time.Time `json:"last_diagnostic,omitempty"`
	LastError             string    `json:"last_error,omitempty"`
}

// Module describes only installed official RouterForge modules, never external packages.
type Module struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// ModuleSnapshot returns a bounded, canonical list of installed module versions.
type ModuleSnapshot func() []Module

// VersionChannel reflects the binary's installed build, not the selected update feed.
func VersionChannel(v string) string {
	x := strings.ToLower(v)
	if x == "dev" || strings.Contains(x, "-dev") || strings.Contains(x, "~dev") {
		return "dev"
	}
	if strings.Contains(x, "-beta") || strings.Contains(x, "~beta") {
		return "beta"
	}
	return "stable"
}

var officialModules = map[string]bool{
	"admin": true, "monitoring": true, "dns": true, "network-tools": true,
	"nfqws-manager": true, "antiscan-manager": true, "port-access-manager": true,
	"crowdsec-manager": true, "profiling": true,
}

// CanonicalModules discards unrecognized identifiers or invalid versions, limits size,
// and sorts entries to prevent exposing arbitrary configuration through telemetry.
func CanonicalModules(source []Module) []Module {
	out := make([]Module, 0, len(officialModules))
	seen := map[string]bool{}
	for _, mod := range source {
		if !officialModules[mod.ID] || seen[mod.ID] || !versionRE.MatchString(mod.Version) {
			continue
		}
		seen[mod.ID] = true
		out = append(out, mod)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

type Event struct {
	SchemaVersion  int       `json:"schema_version"`
	Kind           string    `json:"kind"`
	InstallationID string    `json:"installation_id"`
	EventID        string    `json:"event_id"`
	CoreVersion    string    `json:"core_version,omitempty"`
	ReleaseChannel string    `json:"release_channel,omitempty"`
	Architecture   string    `json:"architecture,omitempty"`
	Modules        *[]Module `json:"modules,omitempty"`
}

type Manager struct {
	mu                                       sync.Mutex
	configPath, statePath, endpoint, version string
	httpClient                               *http.Client
	modules                                  ModuleSnapshot
	now                                      func() time.Time
	config                                   Settings
	state                                    State
	loaded                                   bool
	lastError                                string
	sendCancel                               context.CancelFunc
	sendingKind                              string
}

func NewManager(configPath, statePath, endpoint, version string, modules ModuleSnapshot, client *http.Client) *Manager {
	if client == nil {
		client = &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &Manager{configPath: configPath, statePath: statePath, endpoint: endpoint, version: version, modules: modules, httpClient: client, now: func() time.Time { return time.Now().UTC() }, config: Settings{DiagnosticsEnabled: true}}
}

func randID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
func validID(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func safeRead(path string) ([]byte, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() || fi.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("unsafe state file")
	}
	return os.ReadFile(path)
}
func atomicWrite(path string, data []byte) error {
	if fi, err := os.Lstat(path); err == nil {
		if !fi.Mode().IsRegular() || fi.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe target file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(parent, ".rftelemetry-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
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
	return os.Rename(tmp, path)
}
func persist(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return atomicWrite(path, append(data, '\n'))
}

func (m *Manager) initLocked() error {
	if m.loaded {
		return nil
	}
	raw, err := safeRead(m.statePath)
	if err == nil {
		if err = json.Unmarshal(raw, &m.state); err != nil {
			return fmt.Errorf("corrupt installation state: %w", err)
		}
		if !validID(m.state.InstallationID) || (!m.state.Registered && !validID(m.state.RegisterEventID)) ||
			(m.state.PresenceEventID != "" && !validID(m.state.PresenceEventID)) ||
			(m.state.DiagnosticEventID != "" && !validID(m.state.DiagnosticEventID)) {
			return errors.New("invalid installation state: refuse replacement ID")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		id, e := randID()
		if e != nil {
			return e
		}
		ev, e := randID()
		if e != nil {
			return e
		}
		m.state = State{InstallationID: id, RegisterEventID: ev}
		if err = persist(m.statePath, m.state); err != nil {
			return err
		}
	} else {
		return err
	}
	m.loaded = true
	return nil
}
func (m *Manager) settingsLocked() Settings {
	raw, err := safeRead(m.configPath)
	if errors.Is(err, os.ErrNotExist) {
		m.config = Settings{DiagnosticsEnabled: true}
		return m.config
	}
	if err != nil {
		m.config = Settings{}
		return m.config
	} // bad config fail-closed for diagnostics
	var cfg struct {
		DiagnosticsEnabled *bool `json:"diagnostics_enabled"`
	}
	if err = json.Unmarshal(raw, &cfg); err != nil || cfg.DiagnosticsEnabled == nil {
		m.config = Settings{}
		return m.config
	}
	m.config = Settings{DiagnosticsEnabled: *cfg.DiagnosticsEnabled}
	return m.config
}
func (m *Manager) SetDiagnostics(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := persist(m.configPath, Settings{DiagnosticsEnabled: enabled}); err != nil {
		return err
	}
	m.config.DiagnosticsEnabled = enabled
	if !enabled {
		if m.sendingKind == "diagnostic" && m.sendCancel != nil {
			m.sendCancel()
		}
		if m.loaded && m.state.DiagnosticEventID != "" {
			m.state.DiagnosticEventID = ""
			if err := persist(m.statePath, m.state); err != nil {
				return err
			}
		}
	}
	return nil
}
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg := m.settingsLocked()
	s := Status{CollectorConfigured: validEndpoint(m.endpoint), DiagnosticsEnabled: cfg.DiagnosticsEnabled, LastError: m.lastError}
	if err := m.initLocked(); err != nil {
		s.LastError = "state-unavailable"
		return s
	}
	s.RegistrationConfirmed = m.state.Registered
	s.LastPresence = m.state.LastPresence
	s.LastDiagnostic = m.state.LastDiagnostic
	return s
}
func validEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == "/v1/events"
}
func arch() string {
	switch runtime.GOARCH {
	case "mips":
		return "mips"
	case "mipsle":
		return "mipsel"
	case "arm64":
		return "arm64"
	default:
		return "unknown"
	}
}
func (m *Manager) nextLocked() (*Event, error) {
	if err := m.initLocked(); err != nil {
		return nil, err
	}
	now := m.now()
	s := &m.state
	if !versionRE.MatchString(m.version) {
		return nil, errors.New("bad core version")
	}
	e := &Event{SchemaVersion: 2, InstallationID: s.InstallationID, CoreVersion: m.version, ReleaseChannel: VersionChannel(m.version)}
	if !s.Registered {
		e.Kind = "register"
		e.EventID = s.RegisterEventID
		return e, nil
	}
	if s.LastPresence.IsZero() || now.Sub(s.LastPresence) >= DailyPeriod {
		if s.PresenceEventID == "" {
			v, err := randID()
			if err != nil {
				return nil, err
			}
			s.PresenceEventID = v
			if err = persist(m.statePath, s); err != nil {
				return nil, err
			}
		}
		e.Kind = "presence"
		e.EventID = s.PresenceEventID
		return e, nil
	}
	cfg := m.settingsLocked()
	if cfg.DiagnosticsEnabled && (s.LastDiagnosticSchema < 2 || s.LastDiagnostic.IsZero() || now.Sub(s.LastDiagnostic) >= DailyPeriod) {
		if s.DiagnosticEventID == "" {
			v, err := randID()
			if err != nil {
				return nil, err
			}
			s.DiagnosticEventID = v
			if err = persist(m.statePath, s); err != nil {
				return nil, err
			}
		}
		e.Kind = "diagnostic"
		e.EventID = s.DiagnosticEventID
		e.Architecture = arch()
		mods := []Module{}
		if m.modules != nil {
			mods = CanonicalModules(m.modules())
		}
		e.Modules = &mods
		return e, nil
	}
	return nil, nil
}

// Tick performs at most three bounded requests (register, presence, diagnostic)
// and is called by the background scheduler; it never blocks Core startup.
func (m *Manager) Tick(ctx context.Context) {
	if !validEndpoint(m.endpoint) {
		m.mu.Lock()
		m.lastError = "collector-not-configured"
		m.mu.Unlock()
		return
	}
	for i := 0; i < 3; i++ {
		m.mu.Lock()
		evt, err := m.nextLocked()
		if err != nil {
			m.lastError = "state-unavailable"
			m.mu.Unlock()
			return
		}
		if evt == nil {
			m.mu.Unlock()
			return
		}
		child, cancel := context.WithTimeout(ctx, 6*time.Second)
		m.sendCancel = cancel
		m.sendingKind = evt.Kind
		m.mu.Unlock()
		err = m.send(child, *evt)
		cancel()
		m.mu.Lock()
		m.sendCancel = nil
		m.sendingKind = ""
		if err != nil {
			m.lastError = "send-failed"
			m.mu.Unlock()
			return
		}
		m.lastError = ""
		now := m.now()
		switch evt.Kind {
		case "register":
			m.state.Registered = true
			m.state.RegisterEventID = ""
			m.state.LastPresence = now
		case "presence":
			m.state.LastPresence = now
			m.state.PresenceEventID = ""
		case "diagnostic":
			m.state.LastDiagnostic = now
			m.state.LastDiagnosticSchema = 2
			m.state.DiagnosticEventID = ""
		}
		if err = persist(m.statePath, m.state); err != nil {
			m.lastError = "state-write-failed"
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()
	}
}
func (m *Manager) send(ctx context.Context, e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if len(body) > MaxPayload {
		return errors.New("payload too large")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "routerforge-telemetry/1")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("collector returned %d", resp.StatusCode)
	}
	var ack struct {
		Accepted bool `json:"accepted"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 257))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&ack); err != nil || !ack.Accepted {
		return errors.New("bad collector acknowledgment")
	}
	return nil
}
func (m *Manager) Run(ctx context.Context) {
	// Persist installation identity independently of network availability.
	_ = m.Status()
	// Fleet startup jitter reduces thundering herd. No outbound network in main().
	id, e := randID()
	if e != nil {
		return
	}
	wait := 20*time.Second + time.Duration(int(id[0])%13)*time.Second
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	m.Tick(ctx)
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Tick(ctx)
		}
	}
}
func (m *Manager) Preview() map[string]any {
	status := m.Status()
	result := map[string]any{"schema_version": 2, "collector_configured": status.CollectorConfigured, "registration_fields": []string{"schema_version", "kind", "installation_id", "event_id", "core_version", "release_channel"}, "diagnostic_fields": []string{"schema_version", "kind", "installation_id", "event_id", "core_version", "release_channel", "architecture", "modules"}, "diagnostics_enabled": status.DiagnosticsEnabled, "disclosure": "Installation accounting and presence remain active when diagnostics are off. IDs are random and may be present at the HTTPS endpoint."}
	return result
}
