package lite

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

func (m *Manager) deviceDirectory() *DeviceDirectory {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Devices == nil {
		m.Devices = NewDeviceDirectory()
		if m.engine != nil {
			m.engine.directory.Store(m.Devices)
		}
	}
	return m.Devices
}
func (m *Manager) RunDeviceDiscovery(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.mu.Lock()
			d := m.Devices
			m.mu.Unlock()
			if d != nil {
				d.Refresh(ctx)
			}
		}
	}
}
func (m *Manager) serveDevices(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/service/devices":
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		directory := m.deviceDirectory()
		directory.Refresh(r.Context())
		rows, issues := directory.Snapshot()
		m.mu.Lock()
		rev := m.revision
		applied := m.applied.Advanced
		active := m.engine != nil
		if !active {
			applied = nil
			rev = ""
		}
		policyActive := active && m.engine.policy.DevicePoliciesActive()
		cacheMode := "service_stopped"
		if active {
			cacheMode = m.engine.cacheMode()
		}
		m.mu.Unlock()
		jsonResponse(w, 200, map[string]any{"devices": rows, "issues": issues, "applied_revision": rev, "applied": applied, "device_policies_active": policyActive, "cache_mode": cacheMode, "identity_ttl_seconds": 20, "limitations": "Direct LAN neighbors only. MAC is not authentication. Clients behind NAT/VPN/another router cannot be distinguished.", "hardware_e2e": "HARDWARE_PENDING"})
	case "/v1/service/devices/evaluate":
		var request struct {
			Operation  string `json:"operation"`
			Domain     string `json:"domain"`
			MAC        string `json:"mac"`
			UseApplied bool   `json:"use_applied"`
		}
		if err := mutation(r, &request); err != nil {
			jsonResponse(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if request.Operation != "evaluate" {
			jsonResponse(w, 400, map[string]string{"error": "invalid operation"})
			return
		}
		var cfg dr.Config
		var rev string
		var err error
		source := "saved_draft"
		if request.UseApplied {
			source = "applied"
			m.mu.Lock()
			cfg, rev = m.applied, m.revision
			active := m.engine != nil
			m.mu.Unlock()
			if !active {
				err = errors.New("DNS service has no active applied configuration")
			}
		} else {
			cfg, rev, err = m.Store.Load()
			if err == nil && strings.Trim(r.Header.Get("If-Match"), "\"") != rev {
				err = dr.ErrStale
			}
		}
		var compiled *dr.Engine
		if err == nil {
			compiled, err = dr.Compile(cfg)
		}
		var decision dr.ClientDecision
		if err == nil {
			decision, err = compiled.EvaluateClientDNSQuestion(request.Domain, request.MAC)
		}
		chain := []dr.Upstream{}
		if err == nil && decision.Matched {
			chain, err = compiled.UpstreamChainForDecision(decision.Decision)
		}
		if err != nil {
			status := 400
			if errors.Is(err, dr.ErrStale) {
				status = 409
			}
			jsonResponse(w, status, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, 200, map[string]any{"source": source, "scope": "CONFIGURATION_ONLY", "network_verified": false, "identity_verified": false, "revision": rev, "decision": decision, "allowed_chain": chain, "note": "Selected MAC is a manual simulation, not proof of a real client query."})
	default:
		w.WriteHeader(404)
	}
}
