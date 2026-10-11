package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Entware's opkg can run longer than the module HTTP read timeout. An install
// job therefore outlives its initiating request and serializes opkg globally.
type engineInstallSnapshot struct {
	Engine string `json:"engine,omitempty"`
	State  string `json:"state"`
	Error  string `json:"error,omitempty"`
}

type engineInstallManager struct {
	mu       sync.Mutex
	busy     bool
	snapshot engineInstallSnapshot
}

var engineInstalls engineInstallManager

func (m *engineInstallManager) status(engine string) engineInstallSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.snapshot.Engine != engine {
		return engineInstallSnapshot{Engine: engine, State: "idle"}
	}
	return m.snapshot
}

func (m *engineInstallManager) start(engine string, runner func(context.Context, string) error) error {
	m.mu.Lock()
	if m.busy {
		m.mu.Unlock()
		return errors.New("another Entware installation is running")
	}
	if _, ok := engineServiceNames[engine]; !ok {
		m.mu.Unlock()
		return errors.New("unknown engine")
	}
	if runner == nil {
		m.mu.Unlock()
		return errors.New("missing package runner")
	}
	m.busy = true
	m.snapshot = engineInstallSnapshot{Engine: engine, State: "running"}
	m.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		err := installExistingEngine(ctx, engine, runner)
		m.mu.Lock()
		defer m.mu.Unlock()
		m.busy = false
		if err != nil {
			m.snapshot = engineInstallSnapshot{Engine: engine, State: "failed", Error: err.Error()}
		} else {
			m.snapshot = engineInstallSnapshot{Engine: engine, State: "completed"}
		}
	}()
	return nil
}
