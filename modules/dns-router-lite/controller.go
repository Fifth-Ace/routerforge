package lite

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// A listener can fail while the management process remains alive. Supervision
// covers process death; this loop separately restores only the applied engine.
func (m *Manager) controllerTick(ctx context.Context) (err error) {
	if !m.operations.TryLock() {
		return nil
	}
	defer m.operations.Unlock()
	m.mu.Lock()
	s := m.intent
	eng := m.engine
	maintenance := m.maintenance
	attempts := m.recoveryAttempts
	m.mu.Unlock()
	if !s.ServiceEnabled || maintenance || attempts >= 3 {
		return nil
	}
	defer func() {
		if err != nil {
			m.mu.Lock()
			m.recoveryAttempts++
			m.lastError = fmt.Sprintf("listener recovery %d/3: %v", m.recoveryAttempts, err)
			m.mu.Unlock()
		}
	}()
	var failure error
	if eng != nil {
		plan := eng.readinessPlan()
		if s.InterceptionEnabled && m.Gate != nil {
			plan, failure = m.Gate.makePlan(eng.cfg.Service.Listen)
		}
		if failure == nil {
			failure = eng.GateReady(ctx, eng.cfg.Service.Listen, plan)
		}
		if failure == nil {
			return nil
		}
	} else {
		failure = errors.New("applied listener is not running")
	}
	if m.Gate != nil {
		if e := m.Gate.Quiesce(ctx); e != nil {
			return fmt.Errorf("engine failed: %v; DNS deactivation blocked: %w", failure, e)
		}
	}
	if err := m.closeEngine(); err != nil {
		return err
	}
	c, err := m.intentStore().Applied(s)
	if err == nil {
		err = m.start(c, s.AppliedRevision, false)
	}

	if err != nil {
		return err
	}
	if s.InterceptionEnabled && m.Gate != nil {
		if err = m.Gate.Restore(ctx, c.Service.Listen); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.recoveryAttempts = 0
	m.lastError = ""
	m.mu.Unlock()
	return nil
}
func (m *Manager) RunController(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check, cancel := context.WithTimeout(ctx, 8*time.Second)
			err := m.controllerTick(check)
			cancel()
			if err != nil {
				m.mu.Lock()
				m.lastError = err.Error()
				m.mu.Unlock()
			}
		}
	}
}
