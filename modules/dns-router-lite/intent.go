package lite

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

// Intent is user intent, never inferred from NAT objects or a recovery error.
// Revision-addressed snapshots make a failed/partial write leave the previous
// committed ON/OFF state and its configuration recoverable.
type Intent struct {
	Version             int    `json:"version"`
	ServiceEnabled      bool   `json:"service_enabled"`
	InterceptionEnabled bool   `json:"interception_enabled"`
	AppliedRevision     string `json:"applied_revision,omitempty"`
	Consent             string `json:"gate_consent,omitempty"`
	Migration           string `json:"migration,omitempty"`
}
type IntentStore struct{ ConfigPath string }

func validRevision(rev string) bool {
	if !strings.HasPrefix(rev, "sha256:") || len(rev) != 71 {
		return false
	}
	_, err := hex.DecodeString(rev[7:])
	return err == nil
}
func validateIntent(s Intent) error {
	if s.Version != 1 {
		return errors.New("unsupported DNS intent version")
	}
	if s.AppliedRevision != "" && !validRevision(s.AppliedRevision) {
		return errors.New("invalid applied revision")
	}
	if s.ServiceEnabled && !validRevision(s.AppliedRevision) {
		return errors.New("service ON requires a verified applied snapshot")
	}
	if s.InterceptionEnabled && (!s.ServiceEnabled || (s.Consent != "explicit-enable-v1" && s.Consent != "verified-legacy-live-active")) {
		return errors.New("interception ON has no verified user consent")
	}
	return nil
}
func readState(path string, dst any, limit int64) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() || fi.Size() > limit {
		return errors.New("state must be a bounded regular file")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("invalid trailing state data")
	}
	return nil
}
func (s IntentStore) Path() string { return s.ConfigPath + ".intent.json" }
func (s IntentStore) Load() (Intent, error) {
	var state Intent
	err := readState(s.Path(), &state, 8192)
	if err != nil {
		return state, err
	}
	return state, validateIntent(state)
}
func (s IntentStore) Save(state Intent) error {
	if err := validateIntent(state); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicFile(s.Path(), append(b, '\n'))
}
func (s IntentStore) SnapshotPath(revision string) (string, error) {
	if !validRevision(revision) {
		return "", errors.New("invalid snapshot revision")
	}
	dir := s.ConfigPath + ".applied"
	if fi, err := os.Lstat(dir); err == nil && (!fi.IsDir() || fi.Mode()&os.ModeSymlink != 0) {
		return "", errors.New("snapshot directory must not be a symlink")
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return filepath.Join(dir, revision[7:]+".json"), nil
}
func (s IntentStore) Snapshot(c dr.Config, revision string) error {
	p, err := s.SnapshotPath(revision)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(p); err == nil {
		_, actual, err := dr.NewConfigStore(p).Load()
		if err != nil {
			return err
		}
		if actual != revision {
			return errors.New("immutable snapshot revision mismatch")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = atomicFile(p, dr.ExportContent(c)); err != nil {
		return err
	}
	_, actual, err := dr.NewConfigStore(p).Load()
	if err != nil {
		return err
	}
	if actual != revision {
		return errors.New("staged snapshot revision mismatch")
	}
	return nil
}
func (s IntentStore) Applied(state Intent) (dr.Config, error) {
	p, err := s.SnapshotPath(state.AppliedRevision)
	if err != nil {
		return dr.Config{}, err
	}
	if _, err = os.Lstat(p); err != nil {
		return dr.Config{}, fmt.Errorf("applied snapshot unavailable: %w", err)
	}
	c, actual, err := dr.NewConfigStore(p).Load()
	if err != nil {
		return c, err
	}
	if actual != state.AppliedRevision {
		return c, errors.New("applied snapshot does not match persisted intent")
	}
	return c, nil
}
func (s IntentStore) LoadOrMigrate() (Intent, error) {
	state, err := s.Load()
	if err == nil || !os.IsNotExist(err) {
		return state, err
	}
	state = Intent{Version: 1, Migration: "fresh-install: OFF; no Gate consent"}
	legacy := s.ConfigPath + ".service-active.json"
	if _, err = os.Lstat(legacy); os.IsNotExist(err) {
		return state, nil
	} else if err != nil {
		return state, err
	}
	c, rev, err := dr.NewConfigStore(legacy).Load()
	if err != nil {
		return state, err
	}
	if c.Service == nil {
		return state, errors.New("legacy applied snapshot has no DNS service")
	}
	if err = s.Snapshot(c, rev); err != nil {
		return state, err
	}
	state.ServiceEnabled = true
	state.AppliedRevision = rev
	state.Migration = "legacy service ON restored; Gate requires explicit confirmation"
	if err = s.Save(state); err != nil {
		return state, err
	}
	return state, nil
}
