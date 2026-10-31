package lite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

type Manager struct {
	mu        sync.Mutex
	Store     *dr.ConfigStore
	Path      string
	Factory   Factory
	engine    *Engine
	applied   dr.Config
	revision  string
	lastError string
	Profile   *ProfileManager
	Gate      *GateManager
}

func atomicFile(path string, data []byte) error {
	if fi, e := os.Lstat(path); e == nil && !fi.Mode().IsRegular() {
		return errors.New("state path must be a regular file")
	} else if e != nil && !os.IsNotExist(e) {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".dns-lite-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}

func (m *Manager) snapshotPath() string { return m.Path + ".service-active.json" }
func (m *Manager) ActiveState() bool    { m.mu.Lock(); defer m.mu.Unlock(); return m.engine != nil }
func (m *Manager) statusLocked() map[string]any {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	s := map[string]any{"mode": "independent-dns", "active": m.engine != nil, "applied_revision": m.revision, "last_error": m.lastError, "interception": false, "native_zone_registrations": 0, "heap_bytes": mem.HeapAlloc, "trace_limit": 128, "backend": "AdGuard DNSProxy v0.86.0"}
	if m.engine != nil {
		s["listen"] = m.applied.Service.Listen
		s["default_upstream"] = m.applied.Service.DefaultUpstream
		s["cache_bytes"] = m.applied.Service.CacheBytes
		s["requests"] = m.engine.requests.Load()
		s["denied_requests"] = m.engine.denied.Load()
		s["cache_hits"] = m.engine.cacheHits.Load()
		s["failures"] = m.engine.failures.Load()
		s["rules"] = len(m.applied.Rules)
	}
	if m.Profile != nil {
		s["connection"] = m.Profile.Status()
	}
	if m.Gate != nil {
		s["system_gate"] = m.Gate.Status()
		s["interception"] = m.Gate.Status()["active"]
	}
	return s
}
func (m *Manager) Resume() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, e := os.Lstat(m.snapshotPath()); os.IsNotExist(e) {
		return nil
	} else if e != nil {
		return e
	}
	c, rev, e := dr.NewConfigStore(m.snapshotPath()).Load()
	if e != nil {
		return e
	}
	e = m.startLocked(c, rev, false)
	if e != nil {
		m.lastError = e.Error()
	}
	return e
}
func (m *Manager) startLocked(c dr.Config, rev string, persist bool) error {
	if m.engine != nil {
		return errors.New("DNS service is already running; use apply")
	}
	ledger, _, e := dr.NewLedgerStoreO(m.Path + ".managed-state.json").Load()
	if e != nil {
		return e
	}
	if ledger != nil && ledger.State != "disabled" {
		return errors.New("disable legacy P10 zone ownership before starting independent DNS")
	}
	eng, e := NewEngine(c, m.Factory)
	if e != nil {
		return e
	}
	if e = eng.Start(context.Background()); e != nil {
		eng.closeServers()
		return e
	}
	if persist {
		if e = atomicFile(m.snapshotPath(), dr.ExportContent(c)); e != nil {
			_ = eng.Close(context.Background())
			return e
		}
	}
	m.engine = eng
	m.applied = c
	m.revision = rev
	m.lastError = ""
	return nil
}
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.engine == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	e := m.engine.Close(ctx)
	if e == nil {
		m.engine = nil
	}
	return e
}
func (m *Manager) control(op, expected string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, rev, e := m.Store.Load()
	if e != nil {
		return e
	}
	if expected != rev {
		return dr.ErrStale
	}
	switch op {
	case "start":
		return m.startLocked(c, rev, true)
	case "stop":
		if m.Gate != nil {
			if e = m.Gate.Disable(context.Background()); e != nil {
				return e
			}
		}
		if m.Profile != nil {
			if e = m.Profile.Disconnect(context.Background()); e != nil {
				return e
			}
		}
		if m.engine != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if e = m.engine.Close(ctx); e != nil {
				return e
			}
			m.engine = nil
			m.revision = ""
		}
		if e = os.Remove(m.snapshotPath()); e != nil && !os.IsNotExist(e) {
			return e
		}
		return nil
	case "apply":
		if m.Gate != nil && m.Gate.Status()["active"] == true {
			if e = m.Gate.Disable(context.Background()); e != nil {
				return e
			}
		}
		if m.engine == nil {
			return m.startLocked(c, rev, true)
		}
		if m.Profile != nil && m.Profile.Connected() && (c.Service == nil || c.Service.Listen != m.applied.Service.Listen) {
			return errors.New("disconnect Keenetic before changing the service address")
		}
		staged, e := NewEngine(c, m.Factory)
		if e != nil {
			return e
		}
		staged.closeServers()
		old, oldRev := m.applied, m.revision
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		e = m.engine.Close(ctx)
		cancel()
		if e != nil {
			return e
		}
		m.engine = nil
		if e = m.startLocked(c, rev, true); e != nil {
			original := e
			if restore := m.startLocked(old, oldRev, false); restore != nil {
				m.lastError = fmt.Sprintf("apply failed: %v; restore failed: %v", original, restore)
				return errors.New(m.lastError)
			}
			m.lastError = "new configuration rejected; previous DNS restored: " + original.Error()
			return errors.New(m.lastError)
		}
		return nil
	default:
		return errors.New("unknown service operation")
	}
}
func jsonResponse(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
func mutation(r *http.Request, dst any) error {
	if r.Method != "POST" || r.Header.Get("X-RouterForge-Action") != "service-control" {
		return errors.New("POST with service-control action required")
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("application/json required")
	}
	raw, e := io.ReadAll(io.LimitReader(r.Body, 4097))
	if e != nil {
		return e
	}
	if len(raw) > 4096 {
		return errors.New("service request exceeds 4096 bytes")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(dst); e != nil {
		return e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("invalid trailing JSON")
	}
	return nil
}
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.URL.Path {
	case "/v1/service/gate/control":
		var body struct {
			Operation string `json:"operation"`
		}
		err = mutation(r, &body)
		if err == nil {
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.Gate == nil || m.engine == nil || m.applied.Service == nil {
				err = errors.New("start DNS Lite before controlling global DNS gate")
			} else if strings.Trim(r.Header.Get("If-Match"), "\"") != m.revision {
				err = dr.ErrStale
			} else if m.Profile != nil && m.Profile.Connected() {
				err = errors.New("disconnect individual Keenetic DNS client profile first")
			} else {
				switch body.Operation {
				case "preflight":
					err = m.Gate.Preflight(r.Context(), m.applied.Service.Listen)
				case "enable":
					err = m.Gate.Enable(r.Context(), m.applied.Service.Listen)
				case "disable":
					err = m.Gate.Disable(r.Context())
				default:
					err = errors.New("invalid Gate operation")
				}
			}
		}
	case "/v1/service/status":
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		jsonResponse(w, 200, m.statusLocked())
		return
	case "/v1/service/traces":
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		traces := []Trace{}
		if m.engine != nil {
			traces = m.engine.Traces(r.URL.Query().Get("name"))
		}
		jsonResponse(w, 200, map[string]any{"traces": traces})
		return
	case "/v1/service/control":
		var body struct {
			Operation string `json:"operation"`
		}
		err = mutation(r, &body)
		if err == nil {
			err = m.control(body.Operation, strings.Trim(r.Header.Get("If-Match"), "\""))
		}
	case "/v1/service/verify":
		var body struct {
			Operation string `json:"operation"`
		}
		err = mutation(r, &body)
		if err == nil {
			var result map[string]any
			result, err = m.Verify(strings.Trim(r.Header.Get("If-Match"), "\""))
			if err == nil {
				jsonResponse(w, 200, result)
				return
			}
		}
	case "/v1/service/migrate-native":
		var body struct {
			Operation string `json:"operation"`
		}
		err = mutation(r, &body)
		if err == nil {
			err = m.migrateNative(strings.Trim(r.Header.Get("If-Match"), "\""))
		}
	case "/v1/service/connect":
		var body struct {
			MAC     string `json:"mac"`
			Persist bool   `json:"persist"`
		}
		err = mutation(r, &body)
		if err == nil {
			m.mu.Lock()
			defer m.mu.Unlock()
			_, rev, e := m.Store.Load()
			err = e
			if err == nil && (m.engine == nil || m.revision != rev || strings.Trim(r.Header.Get("If-Match"), "\"") != rev) {
				err = errors.New("apply the exact saved DNS configuration before connecting a client")
			}
			if err == nil && m.Profile == nil {
				err = errors.New("Keenetic connection is unavailable")
			}
			if err == nil {
				err = m.Profile.Connect(r.Context(), body.MAC, m.applied.Service.Listen, body.Persist)
			}
		}
	case "/v1/service/disconnect":
		var body struct {
			Operation string `json:"operation"`
		}
		err = mutation(r, &body)
		if err == nil {
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.Profile != nil {
				err = m.Profile.Disconnect(r.Context())
			}
		}
	default:
		w.WriteHeader(404)
		return
	}
	if err != nil {
		jsonResponse(w, 409, map[string]any{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]any{"ok": true})
}

// Migration is an explicit config-only operation. Native endpoint metadata is
// inspected before DNS is connected; runtime forwarding never reads stubby.
func (m *Manager) migrateNative(expected string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, rev, e := m.Store.Load()
	if e != nil {
		return e
	}
	if rev != expected {
		return dr.ErrStale
	}
	inv := dr.ReadOnlyNativeDNSInventoryP82("/var")
	needed, e := usedUpstreams(c)
	if e != nil {
		return e
	}
	for i, u := range c.Upstreams {
		if u.Transport != "native" || !needed[u.ID] {
			continue
		}
		probe := "example.org"
		parts := strings.Split(strings.TrimPrefix(u.Address, "native://"), "/")
		if len(parts) != 3 {
			return errors.New("invalid native selector")
		}
		if parts[1] != "." {
			probe = parts[1]
		}
		address, e := dr.ResolveNativeDestinationP83(u, probe, inv, "")
		if e != nil {
			return e
		}
		hint := ""
		for _, instance := range inv.Instances {
			for _, ep := range instance.Endpoints {
				if instance.Name == parts[0] && ep.Address == address && ep.Domain == parts[1] {
					hint = ep.UpstreamHint
				}
			}
		}
		hint = strings.TrimSuffix(hint, "@dnsm")
		endpoint, er := url.Parse(hint)
		if er != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.RawQuery != "" {
			return fmt.Errorf("DNS %s has no unambiguous DoH URL; add its direct DoT/DoH settings manually", u.ID)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ips, er := net.DefaultResolver.LookupIPAddr(ctx, endpoint.Hostname())
		cancel()
		if er != nil || len(ips) == 0 {
			return fmt.Errorf("cannot resolve native DNS %s before migration", u.ID)
		}
		ip := ips[0].IP
		for _, a := range ips {
			if a.IP.To4() != nil {
				ip = a.IP
				break
			}
		}
		port := endpoint.Port()
		if port == "" {
			port = "443"
		}
		u.Transport = "doh"
		u.Address = net.JoinHostPort(ip.String(), port)
		u.ServerName = strings.ToLower(endpoint.Hostname())
		u.Path = endpoint.Path
		if u.Path == "" {
			u.Path = "/dns-query"
		}
		u.CertificatePEM = ""
		c.Upstreams[i] = u
	}
	_, e = m.Store.Save(c, rev)
	return e
}
