package lite

import (
	"context"
	"errors"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"os"
	"time"
)

func (m *Manager) finishLegacyCleanup() error {
	path := m.Path + ".legacy-gate-cleanup.json"
	var p GatePlan
	if err := readState(path, &p, 8192); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err := validateGatePlan(p); err != nil {
		return err
	}
	k, ok := m.Gate.kernel.(*LinuxGate)
	if !ok {
		return errors.New("legacy cleanup backend unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := k.Remove(ctx, p, true); err != nil {
		return err
	}
	return os.Remove(path)
}

type UpgradeEvidence struct {
	Active          bool   `json:"active"`
	AppliedRevision string `json:"applied_revision"`
	Connection      struct {
		State string `json:"state"`
	} `json:"connection"`
	Gate struct {
		Active bool   `json:"active"`
		State  string `json:"state"`
	} `json:"system_gate"`
}

func PrepareUpgrade(path string, evidence UpgradeEvidence) (Intent, error) {
	store := IntentStore{ConfigPath: path}
	if state, err := store.Load(); err == nil {
		if evidence.Connection.State != "disconnected" {
			return state, errors.New("restore legacy client profile before upgrade")
		}
		if state.ServiceEnabled != evidence.Active || (state.ServiceEnabled && state.AppliedRevision != evidence.AppliedRevision) {
			return state, errors.New("prepared intent differs from live legacy service; inspect before retry")
		}
		if state.ServiceEnabled {
			if _, err = store.Applied(state); err != nil {
				return state, err
			}
		}
		if state.InterceptionEnabled && (!evidence.Gate.Active || evidence.Gate.State != "ACTIVE") {
			return state, errors.New("prepared Gate consent differs from live state; inspect before retry")
		}
		return state, nil
	} else if !os.IsNotExist(err) {
		return state, err
	}
	state := Intent{Version: 1, Migration: "legacy service OFF; Gate OFF"}
	if evidence.Connection.State != "disconnected" {
		return state, errors.New("restore legacy client profile before the first P19 upgrade")
	}
	if evidence.Active {
		legacy := path + ".service-active.json"
		if _, err := os.Lstat(legacy); err != nil {
			return state, err
		}
		c, rev, err := dr.NewConfigStore(legacy).Load()
		if err != nil {
			return state, err
		}
		if rev != evidence.AppliedRevision || c.Service == nil {
			return state, errors.New("live DNS revision does not match the legacy applied snapshot")
		}
		if err = store.Snapshot(c, rev); err != nil {
			return state, err
		}
		state.ServiceEnabled = true
		state.AppliedRevision = rev
		state.Migration = "legacy service ON preserved; Gate requires explicit confirmation"
		if evidence.Gate.Active && evidence.Gate.State == "ACTIVE" {
			state.InterceptionEnabled = true
			state.Consent = "verified-legacy-live-active"
			state.Migration = "live explicit Gate ACTIVE consent preserved; revalidation required before restore"
		}
	}
	if err := store.Save(state); err != nil {
		return state, err
	}
	return state, nil
}
func RestoreUpgradeMirror(path string) error {
	store := IntentStore{ConfigPath: path}
	state, err := store.Load()
	if err != nil {
		return err
	}
	if !state.ServiceEnabled {
		err = os.Remove(path + ".service-active.json")
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	c, err := store.Applied(state)
	if err != nil {
		return err
	}
	return atomicFile(path+".service-active.json", dr.ExportContent(c))
}
