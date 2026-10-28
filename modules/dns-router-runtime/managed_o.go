package dnsrouter

// DPR2-O1: durable ownership ledger and read-only reconciliation for future
// managed DNS activation. This stage NEVER executes ndmc mutations, performs
// autostart, or treats a matching foreign DNS registration as owned.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const managedLedgerMaxBytesO = 32768

// LedgerO stores explicitly confirmed ownership, not a snapshot inferred from
// the current ndmc output. A pending record MUST be recovered before new work.
type LedgerO struct {
	SchemaVersion int      `json:"schema_version"`
	State         string   `json:"state"` // pending, active or deactivating; never guessed
	Revision      string   `json:"revision"`
	Listen        string   `json:"listen"`
	Zones         []string `json:"zones"`
	BaselineHash  string   `json:"baseline_hash"`
	Baseline      string   `json:"baseline,omitempty"` // exact canonical ndmc baseline; required for O2 mutations
}

type LedgerStoreO struct {
	path string
	mu   sync.Mutex
}

func NewLedgerStoreO(path string) *LedgerStoreO { return &LedgerStoreO{path: path} }

func digestLedgerO(ledger LedgerO) string {
	raw, _ := json.Marshal(ledger)
	h := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(h[:])
}

func hashSyntaxO(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	_, e := hex.DecodeString(s[7:])
	return e == nil && strings.ToLower(s) == s
}

func validateLedgerO(v LedgerO) error {
	if v.SchemaVersion != 1 {
		return errors.New("unsupported ownership ledger schema")
	}
	if v.State != "pending" && v.State != "active" && v.State != "deactivating" {
		return errors.New("ownership ledger has invalid state")
	}
	if !hashSyntaxO(v.Revision) || !hashSyntaxO(v.BaselineHash) {
		return errors.New("ownership ledger requires exact SHA256 revisions")
	}
	if v.Baseline != "" {
		canonical, e := canonicalNDMCSnapshotM(v.Baseline)
		if e != nil || canonical != v.Baseline || hashSnapshotO2(canonical) != v.BaselineHash {
			return errors.New("ownership ledger baseline is invalid or mismatched")
		}
	}
	host, port, e := net.SplitHostPort(v.Listen)
	if e != nil || host != "192.168.10.1" || port != "55358" {
		return errors.New("ownership ledger limited to verified DSTest listener")
	}
	if len(v.Zones) < 1 || len(v.Zones) > managedMaxZonesN {
		return errors.New("ownership ledger requires 1..8 zones")
	}
	seen := make(map[string]bool)
	for _, d := range v.Zones {
		n, e := normalizedDomain(d)
		if e != nil || n != d || isProtected(d) || !strings.Contains(d, ".") || seen[d] {
			return errors.New("invalid ownership ledger zone")
		}
		seen[d] = true
		for x := range seen {
			if x != d && (strings.HasSuffix(d, "."+x) || strings.HasSuffix(x, "."+d)) {
				return errors.New("overlapping ownership ledger zones")
			}
		}
	}
	if !sort.StringsAreSorted(v.Zones) {
		return errors.New("ownership ledger zones must be ordered")
	}
	return nil
}

func (s *LedgerStoreO) loadLocked() (*LedgerO, string, error) {
	if s == nil || !filepath.IsAbs(s.path) {
		return nil, "", errors.New("ownership ledger requires absolute path")
	}
	st, e := os.Lstat(s.path)
	if errors.Is(e, os.ErrNotExist) {
		return nil, "", nil
	}
	if e != nil {
		return nil, "", e
	}
	if !st.Mode().IsRegular() || st.Size() > managedLedgerMaxBytesO {
		return nil, "", errors.New("unsafe ownership ledger file")
	}
	f, e := os.Open(s.path)
	if e != nil {
		return nil, "", e
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, managedLedgerMaxBytesO+1))
	dec.DisallowUnknownFields()
	var v LedgerO
	if e = dec.Decode(&v); e != nil {
		return nil, "", fmt.Errorf("ownership ledger parse: %w", e)
	}
	var extra any
	if e = dec.Decode(&extra); e != io.EOF {
		return nil, "", errors.New("ownership ledger has trailing data")
	}
	if e = validateLedgerO(v); e != nil {
		return nil, "", e
	}
	return &v, digestLedgerO(v), nil
}

// Load is strictly read-only. Missing ledger means NO ownership: it must not
// result in an attempt to delete or adopt any existing ndmc entry.
func (s *LedgerStoreO) Load() (*LedgerO, string, error) {
	if s == nil {
		return nil, "", errors.New("ownership ledger unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// SaveCAS is not exposed by O1 API. O2 activation will use it to persist a
// pending lease BEFORE the first ndmc add. A CAS mismatch never writes.
func (s *LedgerStoreO) SaveCAS(next LedgerO, expected string) (string, error) {
	if s == nil {
		return "", errors.New("ownership ledger unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := validateLedgerO(next); e != nil {
		return "", e
	}
	_, current, e := s.loadLocked()
	if e != nil {
		return "", e
	}
	if current != expected {
		return "", ErrStale
	}
	raw, e := json.MarshalIndent(next, "", "  ")
	if e != nil {
		return "", e
	}
	raw = append(raw, '\n')
	if len(raw) > managedLedgerMaxBytesO {
		return "", errors.New("ownership ledger too large")
	}
	if e = os.MkdirAll(filepath.Dir(s.path), 0700); e != nil {
		return "", e
	}
	if e = safety.WriteFileAtomic(s.path, raw, 0600); e != nil {
		return "", e
	}
	if dir, de := os.Open(filepath.Dir(s.path)); de == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return digestLedgerO(next), nil
}

// ManagedOwnershipStatusO describes observed state; it authorizes NO mutation.
type ManagedOwnershipStatusO struct {
	State            string   `json:"state"`
	Revision         string   `json:"revision,omitempty"`
	LedgerHash       string   `json:"ledger_hash,omitempty"`
	OwnedZones       []string `json:"owned_zones"`
	MissingZones     []string `json:"missing_zones"`
	ConflictZones    []string `json:"conflict_zones"`
	ReadOnly         bool     `json:"read_only"`
	RecoveryRequired bool     `json:"recovery_required"`
	NDMCInvoked      bool     `json:"ndmc_invoked"`
	ChangesApplied   bool     `json:"changes_applied"`
}

// ReconcileLedgerO never adopts an owner based only on host:port/domain.
// Pending ledger or unexpected changes always require separate recovery.
func ReconcileLedgerO(v *LedgerO, rev, snapshot string) (ManagedOwnershipStatusO, error) {
	r := ManagedOwnershipStatusO{State: "disabled", OwnedZones: []string{}, MissingZones: []string{}, ConflictZones: []string{}, ReadOnly: true, NDMCInvoked: false, ChangesApplied: false}
	if v == nil {
		return r, nil
	}
	if e := validateLedgerO(*v); e != nil {
		return r, e
	}
	r.Revision = v.Revision
	r.LedgerHash = rev
	servers, e := snapshotDomainsN(snapshot)
	if e != nil {
		return r, e
	}
	host, portStr, _ := net.SplitHostPort(v.Listen)
	var port int
	if _, e = fmt.Sscanf(portStr, "%d", &port); e != nil {
		return r, e
	}
	for _, zone := range v.Zones {
		own, conflict := 0, false
		for _, s := range servers {
			d := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s.Domain)), ".")
			if d == "" {
				continue
			}
			if d == zone || strings.HasSuffix(d, "."+zone) || strings.HasSuffix(zone, "."+d) {
				if d == zone && s.Address == host && s.Port == port {
					own++
				} else {
					conflict = true
				}
			}
		}
		if own == 1 && !conflict {
			r.OwnedZones = append(r.OwnedZones, zone)
		}
		if own == 0 {
			r.MissingZones = append(r.MissingZones, zone)
		}
		if own > 1 || conflict {
			r.ConflictZones = append(r.ConflictZones, zone)
		}
	}
	if v.State == "pending" {
		r.State = "pending_recovery"
		r.RecoveryRequired = true
	} else if v.State == "deactivating" {
		r.State = "deactivating_recovery"
		r.RecoveryRequired = true
	} else if len(r.MissingZones) > 0 || len(r.ConflictZones) > 0 {
		r.State = "drift"
		r.RecoveryRequired = true
	} else {
		r.State = "verified_active"
	}
	return r, nil
}

func InspectManagedOwnershipO(ctx context.Context, store *LedgerStoreO, executor NDMCExecutorM, currentRevision string) (ManagedOwnershipStatusO, error) {
	r := ManagedOwnershipStatusO{State: "disabled", OwnedZones: []string{}, MissingZones: []string{}, ConflictZones: []string{}, ReadOnly: true}
	ledger, sha, e := store.Load()
	if e != nil {
		return r, e
	}
	if ledger == nil {
		return r, nil
	}
	if executor == nil {
		return r, errors.New("ownership reconciliation requires explicitly enabled ndmc capability")
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	snapshot, e := trialSnapshotM(ctx, executor)
	if e != nil {
		return r, e
	}
	r, e = ReconcileLedgerO(ledger, sha, snapshot)
	r.NDMCInvoked = true
	if e == nil && ledger.Revision != currentRevision {
		r.State = "stale_policy"
		r.RecoveryRequired = true
	}
	return r, e
}

// DeleteCAS removes only the ledger at the exact expected revision. It never
// deletes DNS entries, and cannot be invoked when concurrent state has changed.
func (s *LedgerStoreO) DeleteCAS(expected string) error {
	if s == nil {
		return errors.New("ownership ledger unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, sha, err := s.loadLocked()
	if err != nil {
		return err
	}
	if current == nil || expected == "" || sha != expected {
		return ErrStale
	}
	if err := os.Remove(s.path); err != nil {
		return err
	}
	if dir, err := os.Open(filepath.Dir(s.path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}
