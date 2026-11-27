package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
)

// Startup state is distinct from HTTP liveness. Only the startup endpoint
// is reachable until restoration has finished; degraded state permits recovery.
type startupGate struct {
	state atomic.Uint32
	next  http.Handler
	build string
}

const (
	startupStarting uint32 = iota
	startupReady
	startupDegraded
)

type startupStatus struct {
	Component string `json:"component"`
	Version   string `json:"version"`
	State     string `json:"state"`
}

func (g *startupGate) complete(resumeErr, profileErr error) {
	if resumeErr != nil || profileErr != nil {
		g.state.Store(startupDegraded)
		return
	}
	g.state.Store(startupReady)
}
func (g *startupGate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	state := g.state.Load()
	if r.URL.Path == "/v1/startup" {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		label := "STARTING"
		if state == startupReady {
			label = "READY"
		} else if state == startupDegraded {
			label = "DEGRADED"
		}
		_ = json.NewEncoder(w).Encode(startupStatus{"dns-router", g.build, label})
		return
	}
	if state == startupStarting {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Retry-After", "1")
		http.Error(w, "DNS Router startup in progress", http.StatusServiceUnavailable)
		return
	}
	g.next.ServeHTTP(w, r)
}

type startupServiceStatus struct {
	Active       bool   `json:"active"`
	Maintenance  bool   `json:"maintenance"`
	Revision     string `json:"applied_revision"`
	Interception bool   `json:"interception"`
	Desired      struct {
		Version      int    `json:"version"`
		Enabled      bool   `json:"service_enabled"`
		Interception bool   `json:"interception_enabled"`
		Revision     string `json:"applied_revision"`
	} `json:"desired"`
	Connection struct {
		State string `json:"state"`
	} `json:"connection"`
	LastError string `json:"last_error"`
}

func (s startupServiceStatus) ready() bool {
	if s.Desired.Version != 1 || s.Maintenance || s.LastError != "" || s.Active != s.Desired.Enabled || s.Interception != s.Desired.Interception {
		return false
	}
	if s.Active && (s.Revision == "" || s.Revision != s.Desired.Revision) {
		return false
	}
	if s.Connection.State != "disconnected" && s.Connection.State != "connected" {
		return false
	}
	return true
}
func checkedStartupState(s startupStatus, build string) (string, error) {
	if s.Component != "dns-router" || s.Version != build {
		return "", fmt.Errorf("startup identity mismatch")
	}
	switch s.State {
	case "STARTING", "READY", "DEGRADED":
		return s.State, nil
	}
	return "", fmt.Errorf("invalid startup state")
}
