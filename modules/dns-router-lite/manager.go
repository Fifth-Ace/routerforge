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
	mu               sync.Mutex
	operations       sync.Mutex
	intent           Intent
	intentLoaded     bool
	maintenance      bool
	recoveryAttempts int
	Store            *dr.ConfigStore
	Path             string
	Factory          Factory
	Devices          *DeviceDirectory
	engine           *Engine
	applied          dr.Config
	revision         string
	lastError        string
	Profile          *ProfileManager
	Gate             *GateManager
	Health           *UpstreamHealthTracker
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
	if e = os.Rename(f.Name(), path); e != nil {
		return e
	}
	dir, e := os.Open(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer dir.Close()
	return dir.Sync()
}

func (m *Manager) healthLocked() *UpstreamHealthTracker {
	if m.Health == nil {
		m.Health = NewUpstreamHealthTracker()
	}
	return m.Health
}
func (m *Manager) snapshotPath() string { return m.Path + ".service-active.json" }
func (m *Manager) ActiveState() bool    { m.mu.Lock(); defer m.mu.Unlock(); return m.engine != nil }
func (m *Manager) statusLocked() map[string]any {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	s := map[string]any{"desired": m.intent, "maintenance": m.maintenance, "mode": "independent-dns", "active": m.engine != nil, "applied_revision": m.revision, "last_error": m.lastError, "interception": false, "native_zone_registrations": 0, "heap_bytes": mem.HeapAlloc, "trace_limit": 128, "backend": "AdGuard DNSProxy v0.86.0"}
	if m.engine != nil {
		s["listen"] = m.applied.Service.Listen
		s["default_upstream"] = m.applied.Service.DefaultUpstream
		s["cache_bytes"] = m.engine.effectiveCacheBytes()
		s["requested_cache_bytes"] = m.applied.Service.CacheBytes
		s["cache_mode"] = m.engine.cacheMode()
		s["device_policies_active"] = m.engine.policy.DevicePoliciesActive()
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
	if strings.HasPrefix(r.URL.Path, "/v1/service/devices") {
		m.serveDevices(w, r)
		return
	}
	var err error
	switch r.URL.Path {
	case "/v1/service/gate/control":
		var body struct {
			Operation string `json:"operation"`
		}
		err = mutation(r, &body)
		if err == nil {
			err = m.gateControl(r.Context(), body.Operation, strings.Trim(r.Header.Get("If-Match"), "\""))
		}
	case "/v1/service/upstreams/inspect":
		var body struct {
			Operation string      `json:"operation"`
			Upstream  dr.Upstream `json:"upstream"`
		}
		err = mutation(r, &body)
		if err == nil && body.Operation != "inspect" {
			err = errors.New("invalid draft inspect operation")
		}
		if err == nil {
			m.mu.Lock()
			factory := m.Factory
			m.mu.Unlock()
			var result DraftInspectP194
			result, err = InspectDraftP194(r.Context(), body.Upstream, factory)
			if err == nil {
				jsonResponse(w, http.StatusOK, result)
				return
			}
		}
	case "/v1/service/upstreams/health":
		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			return
		}
		m.mu.Lock()
		c, _, loadErr := m.Store.Load()
		h := m.healthLocked()
		m.mu.Unlock()
		if loadErr != nil {
			jsonResponse(w, 503, map[string]any{"error": loadErr.Error()})
			return
		}
		jsonResponse(w, 200, map[string]any{"source": "routerforge-direct", "window_seconds": 300, "upstreams": h.Snapshot(c.Upstreams)})
		return
	case "/v1/service/upstreams/check":
		var body struct {
			Operation string `json:"operation"`
			Upstream  string `json:"upstream"`
		}
		err = mutation(r, &body)
		if err == nil && body.Operation != "check" {
			err = errors.New("invalid health check operation")
		}
		if err == nil {
			m.mu.Lock()
			c, rev, loadErr := m.Store.Load()
			h := m.healthLocked()
			factory := m.Factory
			m.mu.Unlock()
			err = loadErr
			if err == nil && strings.Trim(r.Header.Get("If-Match"), "\"") != rev {
				err = dr.ErrStale
			}
			if err == nil {
				ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
				err = h.Probe(ctx, c.Upstreams, body.Upstream, factory)
				cancel()
				if err == nil {
					jsonResponse(w, 200, map[string]any{"source": "routerforge-direct", "upstreams": h.Snapshot(c.Upstreams)})
					return
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

func (m *Manager) InterceptionState() bool { return m.Gate != nil && m.Gate.Status()["active"] == true }
