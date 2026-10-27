package dnsrouter

// DPR2-E manages ONLY a manually activated, loopback/high-port shadow DNS
// listener. It never changes Keenetic DNS, firewall, DHCP or port 53.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrInactive = errors.New("shadow DNS forwarder is not active")
var ErrAlreadyActive = errors.New("shadow DNS forwarder already active")

type LifecycleStatus struct {
	Active                       bool   `json:"active"`
	Interception                 bool   `json:"interception"`
	Listen                       string `json:"listen"`
	AppliedRevision              string `json:"applied_revision,omitempty"`
	LastError                    string `json:"last_error,omitempty"`
	RuntimeRequiresExplicitStart bool   `json:"runtime_requires_explicit_start"`
}

type LifecycleManager struct {
	mu              sync.Mutex
	store           *ConfigStore
	opts            ShadowOptions
	forward         *ShadowForwarder
	cancel          context.CancelFunc
	done            chan error
	appliedConfig   Config
	appliedRevision string
	lastError       string
}

func NewLifecycleManager(store *ConfigStore, opts ShadowOptions) *LifecycleManager {
	return &LifecycleManager{store: store, opts: opts}
}

func (l *LifecycleManager) pollLocked() {
	if l.done == nil {
		return
	}
	select {
	case err := <-l.done:
		l.forward = nil
		l.cancel = nil
		l.done = nil
		l.appliedRevision = ""
		if err != nil {
			l.lastError = err.Error()
		} else {
			l.lastError = ""
		}
	default:
	}
}

func (l *LifecycleManager) Status() LifecycleStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pollLocked()
	return LifecycleStatus{Active: l.forward != nil && l.forward.Active(), Interception: false, Listen: l.opts.Listen, AppliedRevision: l.appliedRevision, LastError: l.lastError, RuntimeRequiresExplicitStart: true}
}

func (l *LifecycleManager) Forwarder() *ShadowForwarder {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pollLocked()
	return l.forward
}

func (l *LifecycleManager) startLocked(cfg Config, rev string) error {
	f, err := NewShadowForwarder(cfg, l.opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- f.Serve(ctx) }()
	timeout := time.NewTimer(4 * time.Second)
	defer timeout.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		// Prefer immediate startup failure over false ready.
		select {
		case err := <-done:
			cancel()
			if err == nil {
				return errors.New("shadow DNS listener stopped before becoming ready")
			}
			return err
		default:
		}
		if f.Active() {
			l.forward, l.cancel, l.done = f, cancel, done
			l.appliedConfig, l.appliedRevision = cfg, rev
			l.lastError = ""
			return nil
		}
		select {
		case <-timeout.C:
			cancel()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
			}
			return errors.New("shadow DNS listener readiness timeout")
		case <-tick.C:
		}
	}
}

func (l *LifecycleManager) stopLocked() error {
	l.pollLocked()
	if l.done == nil {
		return ErrInactive
	}
	l.cancel()
	select {
	case err := <-l.done:
		l.forward, l.cancel, l.done = nil, nil, nil
		l.appliedRevision = ""
		if err != nil {
			l.lastError = err.Error()
			return err
		}
		l.lastError = ""
		return nil
	case <-time.After(6 * time.Second):
		// Do NOT discard a possibly live listener. This state must be inspected.
		l.lastError = "stop timeout: runtime state ambiguous"
		return errors.New(l.lastError)
	}
}

func (l *LifecycleManager) Start(expected string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pollLocked()
	if l.done != nil {
		return ErrAlreadyActive
	}
	cfg, rev, err := l.store.Load()
	if err != nil {
		return err
	}
	if expected == "" || rev != expected {
		return ErrStale
	}
	return l.startLocked(cfg, rev)
}
func (l *LifecycleManager) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stopLocked()
}

// Apply atomically selects a saved configuration revision for the *shadow*
// process. A failed replacement restores the old in-memory snapshot; persistent
// user configuration is never changed by Apply or rollback.
func (l *LifecycleManager) Apply(expected string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pollLocked()
	if l.done == nil {
		return "inactive", ErrInactive
	}
	cfg, rev, err := l.store.Load()
	if err != nil {
		return "precheck", err
	}
	if expected == "" || rev != expected {
		return "stale", ErrStale
	}
	if _, err := NewShadowForwarder(cfg, l.opts); err != nil {
		return "precheck", err
	}
	oldCfg, oldRev := l.appliedConfig, l.appliedRevision
	if err := l.stopLocked(); err != nil {
		return "ambiguous", err
	}
	if err := l.startLocked(cfg, rev); err != nil {
		if undo := l.startLocked(oldCfg, oldRev); undo != nil {
			return "ambiguous", fmt.Errorf("shadow apply failed: %v; rollback failed: %w", err, undo)
		}
		return "rolled_back", fmt.Errorf("shadow apply failed and previous runtime restored: %w", err)
	}
	return "committed", nil
}

func (l *LifecycleManager) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pollLocked()
	if l.done == nil {
		return nil
	}
	return l.stopLocked()
}
