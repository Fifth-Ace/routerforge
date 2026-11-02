package lite

import (
	"context"
	"errors"
	"fmt"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"os"
	"time"
)

func (m *Manager) intentStore() IntentStore { return IntentStore{ConfigPath: m.Path} }
func (m *Manager) loadIntent() error {
	m.mu.Lock()
	loaded := m.intentLoaded
	m.mu.Unlock()
	if loaded {
		return nil
	}
	s, err := m.intentStore().LoadOrMigrate()
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.intent = s
	m.intentLoaded = true
	m.mu.Unlock()
	if m.Gate != nil {
		m.Gate.SetOwnershipPath(m.Path + ".gate-owned.json")
	}
	return nil
}
func (m *Manager) saveIntent(s Intent) error {
	if err := m.intentStore().Save(s); err != nil {
		return err
	}
	m.mu.Lock()
	m.intent = s
	m.intentLoaded = true
	m.mu.Unlock()
	return nil
}
func (m *Manager) Resume() (err error) {
	defer func() {
		if err != nil {
			m.mu.Lock()
			m.lastError = err.Error()
			m.mu.Unlock()
		}
	}()
	m.operations.Lock()
	defer m.operations.Unlock()
	if err := m.loadIntent(); err != nil {
		return err
	}
	if m.Gate != nil {
		if err := m.finishLegacyCleanup(); err != nil {
			return fmt.Errorf("legacy DNS cleanup pending: %w", err)
		}
	}
	m.mu.Lock()
	s := m.intent
	m.maintenance = false
	m.mu.Unlock()
	if !s.ServiceEnabled {
		if m.Gate != nil {
			return m.Gate.Close()
		}
		return nil
	}
	c, err := m.intentStore().Applied(s)
	if err != nil {
		return err
	}
	if err = m.start(c, s.AppliedRevision, false); err != nil {
		return err
	}
	if err = atomicFile(m.snapshotPath(), dr.ExportContent(c)); err != nil {
		return err
	}
	if m.Gate != nil {
		if s.InterceptionEnabled {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			return m.Gate.Restore(ctx, c.Service.Listen)
		}
		_ = m.Gate.Configure(c.Service.Listen)
	}
	return nil
}
func (m *Manager) start(c dr.Config, rev string, persist bool) error {
	m.mu.Lock()
	running := m.engine != nil
	previous := m.intent
	m.mu.Unlock()
	if running {
		return errors.New("DNS service already running")
	}
	ledger, _, err := dr.NewLedgerStoreO(m.Path + ".managed-state.json").Load()
	if err != nil {
		return err
	}
	if ledger != nil && ledger.State != "disabled" {
		return errors.New("disable legacy P10 ownership first")
	}
	eng, err := NewEngine(c, m.Factory)
	if err != nil {
		return err
	}
	m.mu.Lock()
	directory := m.Devices
	m.mu.Unlock()
	if eng.policy.DevicePoliciesActive() {
		directory = m.deviceDirectory()
		directory.Refresh(context.Background())
	}
	eng.directory.Store(directory)
	tracker := NewUpstreamHealthTracker()
	eng.health = tracker
	if err = eng.Start(context.Background()); err != nil {
		eng.closeServers()
		return err
	}
	mirrorError := ""
	if persist {
		if err = m.intentStore().Snapshot(c, rev); err == nil {
			s := previous
			s.Version = 1
			s.ServiceEnabled = true
			s.AppliedRevision = rev
			err = m.saveIntent(s)
		}
		if err != nil {
			_ = eng.Close(context.Background())
			return err
		}
		// Compatibility snapshot is not the commit pointer. A crash while writing
		// this old-format mirror cannot invalidate the immutable applied snapshot.
		if e := atomicFile(m.snapshotPath(), dr.ExportContent(c)); e != nil {
			mirrorError = "legacy mirror write failed: " + e.Error()
		}
	}
	if m.Gate != nil {
		m.Gate.SetProbe(eng.GateReady)
	}
	m.mu.Lock()
	m.engine = eng
	m.applied = c
	m.revision = rev
	m.Health = tracker
	m.maintenance = false
	m.lastError = mirrorError
	m.mu.Unlock()
	return nil
}
func (m *Manager) closeEngine() error {
	m.mu.Lock()
	eng := m.engine
	m.mu.Unlock()
	if eng == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := eng.Close(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	if m.engine == eng {
		m.engine = nil
		m.revision = ""
	}
	m.mu.Unlock()
	return nil
}
func (m *Manager) Close() error {
	m.operations.Lock()
	defer m.operations.Unlock()
	if m.Gate != nil {
		if err := m.Gate.Close(); err != nil {
			return err
		}
	}
	return m.closeEngine()
}
func (m *Manager) control(op, expected string) error {
	m.operations.Lock()
	defer m.operations.Unlock()
	if err := m.loadIntent(); err != nil {
		return err
	}
	c, rev, err := m.Store.Load()
	if op != "stop" && op != "maintenance" {
		if err != nil {
			return err
		}
		if rev != expected {
			return dr.ErrStale
		}
	}
	m.mu.Lock()
	s := m.intent
	old := m.applied
	oldRev := m.revision
	running := m.engine != nil
	m.mu.Unlock()
	switch op {
	case "start":
		return m.start(c, rev, true)
	case "stop":
		s.ServiceEnabled = false
		s.InterceptionEnabled = false
		s.Consent = ""
		if err = m.saveIntent(s); err != nil {
			return err
		}
		if m.Gate != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			err = m.Gate.Disable(ctx)
			cancel()
			if err != nil {
				return err
			}
		}
		if m.Profile != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			err = m.Profile.Disconnect(ctx)
			cancel()
			if err != nil {
				return err
			}
		}
		if err = m.closeEngine(); err != nil {
			return err
		}
		if err = os.Remove(m.snapshotPath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	case "maintenance":
		m.mu.Lock()
		m.maintenance = true
		m.mu.Unlock()
		if m.Gate != nil {
			err = m.Gate.Close()
		}
		if err == nil {
			err = m.closeEngine()
		}
		if err != nil {
			m.mu.Lock()
			m.maintenance = false
			m.mu.Unlock()
			if m.Gate != nil && s.InterceptionEnabled && old.Service != nil {
				restore, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				_ = m.Gate.Restore(restore, old.Service.Listen)
				cancel()
			}
		}
		return err
	case "apply":
		if !running {
			return m.start(c, rev, true)
		}
		if m.Profile != nil && m.Profile.Connected() && (c.Service == nil || c.Service.Listen != old.Service.Listen) {
			return errors.New("disconnect client profile before changing listener")
		}
		staged, e := NewEngine(c, m.Factory)
		if e != nil {
			return e
		}
		staged.closeServers()
		if m.Gate != nil {
			if e = m.Gate.Close(); e != nil {
				return e
			}
		}
		if e = m.closeEngine(); e != nil {
			return e
		}
		if e = m.start(c, rev, true); e != nil {
			if restore := m.start(old, oldRev, false); restore != nil {
				return fmt.Errorf("apply failed: %v; restore failed: %w", e, restore)
			}
			if m.Gate != nil && s.InterceptionEnabled {
				ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				restore := m.Gate.Restore(ctx, old.Service.Listen)
				cancel()
				if restore != nil {
					return fmt.Errorf("DNS restored; Gate recovery pending: %w", restore)
				}
			}
			return fmt.Errorf("previous DNS restored after failed apply: %w", e)
		}
		if m.Gate != nil && s.InterceptionEnabled {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if err = m.Gate.Restore(ctx, c.Service.Listen); err != nil {
				return fmt.Errorf("DNS applied; Gate recovery pending: %w", err)
			}
		}
		return nil
	default:
		return errors.New("unknown service operation")
	}
}
func (m *Manager) gateControl(ctx context.Context, op, expected string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	m.operations.Lock()
	defer m.operations.Unlock()
	if err := m.loadIntent(); err != nil {
		return err
	}
	m.mu.Lock()
	g := m.Gate
	eng := m.engine
	applied := m.applied
	rev := m.revision
	s := m.intent
	m.mu.Unlock()
	if g == nil {
		return errors.New("DNS controller unavailable")
	}
	if op == "disable" {
		s.InterceptionEnabled = false
		s.Consent = ""
		if err := m.saveIntent(s); err != nil {
			return err
		}
		if applied.Service != nil {
			_ = g.Configure(applied.Service.Listen)
		}
		return g.Disable(ctx)
	}
	if op == "notify" {
		if s.InterceptionEnabled {
			g.Notify()
		}
		return nil
	}
	if op == "retry" {
		if !s.InterceptionEnabled {
			return errors.New("explicit Gate enable required")
		}
		if eng == nil || applied.Service == nil {
			return errors.New("restore the applied DNS service before retrying interception")
		}
		return g.Restore(ctx, applied.Service.Listen)
	}
	if eng == nil || applied.Service == nil {
		return errors.New("start DNS before enabling interception")
	}
	if expected != rev {
		return dr.ErrStale
	}
	if m.Profile != nil && m.Profile.Connected() {
		return errors.New("disconnect individual client profile first")
	}
	switch op {
	case "preflight":
		return g.Preflight(ctx, applied.Service.Listen)
	case "enable":
		if err := g.Enable(ctx, applied.Service.Listen); err != nil {
			return err
		}
		s.InterceptionEnabled = true
		s.Consent = "explicit-enable-v1"
		if err := m.saveIntent(s); err != nil {
			_ = g.Disable(ctx)
			return err
		}
		return nil
	default:
		return errors.New("invalid controller operation")
	}
}

// StopOffline is only for an absent daemon. Both locks prevent racing a live
// supervisor or a manually started daemon; no foreign socket is unlinked.
func StopOffline(path, socket string) error {
	release, err := AcquireProcessLock(path + ".supervisor.lock")
	if err != nil {
		return err
	}
	defer release()
	daemon, err := AcquireProcessLock(path + ".daemon.lock")
	if err != nil {
		return err
	}
	defer daemon()
	if err = RecoverOwnedSocket(socket); err != nil {
		return err
	}
	m := &Manager{Path: path, Store: dr.NewConfigStore(path), Gate: NewGateManager(), Profile: &ProfileManager{Path: path + ".profile-state.json", Executor: dr.NewRealNDMCM()}}
	if err = m.loadIntent(); err != nil {
		return err
	}
	if err = m.finishLegacyCleanup(); err != nil {
		return err
	}
	return m.control("stop", "")
}
