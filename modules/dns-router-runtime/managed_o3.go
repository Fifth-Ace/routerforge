package dnsrouter

// DPR2-O3: explicit DSTest-only startup reconciliation. A saved ownership
// ledger, never an observed DNS entry, is the sole authority to repair zones.
// Incomplete ledger transitions are never guessed or silently adopted.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type StartupResultO3 struct {
	State            string   `json:"state"`
	Revision         string   `json:"revision,omitempty"`
	LedgerRevision   string   `json:"ledger_revision,omitempty"`
	Present          []string `json:"present"`
	Missing          []string `json:"missing"`
	Restored         []string `json:"restored"`
	Changed          bool     `json:"changed"`
	WatchdogArmed    bool     `json:"watchdog_armed"`
	RecoveryRequired bool     `json:"recovery_required"`
	ReadOnly         bool     `json:"read_only"`
	Error            string   `json:"error,omitempty"`
}

type StartupManagerO3 struct {
	Durable *DurableManagerO2
	// Only tests inject readiness; production always checks live lifecycle.
	Ready func(string, string) bool
	// Only tests inject watchdog conflict status. Production checks /tmp.
	NoCompetingGuard func() error
}

func newStartupResultO3() StartupResultO3 {
	return StartupResultO3{State: "precheck", Present: []string{}, Missing: []string{}, Restored: []string{}, ReadOnly: true}
}

func checkGuardsO3() error {
	for _, pattern := range []string{"/tmp/rf-dpr2n-guard-*", "/tmp/rf-dpr2o2-guard-*"} {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		if len(paths) != 0 {
			return fmt.Errorf("unresolved watchdog journal: %s", paths[0])
		}
	}
	return nil
}

// classifyStartupO3 strictly accounts for all overlaps, including subzones,
// and compares a canonical inventory with OWNED entries removed to the exact
// baseline captured before first activation. It cannot adopt foreign zones.
func classifyStartupO3(snapshot string, ledger LedgerO) (present, missing []string, err error) {
	present, missing = []string{}, []string{}
	var doc map[string]any
	if err = json.Unmarshal([]byte(snapshot), &doc); err != nil {
		return
	}
	raw, ok := doc["server"].([]any)
	if !ok {
		err = errors.New("invalid native DNS inventory")
		return
	}
	host, portString, e := net.SplitHostPort(ledger.Listen)
	if e != nil {
		err = e
		return
	}
	var port int
	if _, e = fmt.Sscanf(portString, "%d", &port); e != nil {
		err = e
		return
	}
	owned := map[string]int{}
	for _, domain := range ledger.Zones {
		owned[domain] = 0
	}
	remaining := make([]any, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			err = errors.New("untrusted DNS inventory entry")
			return
		}
		domain, ok := entry["domain"].(string)
		if !ok {
			err = errors.New("invalid native domain")
			return
		}
		domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
		overlap := false
		for _, zone := range ledger.Zones {
			if domain == zone || strings.HasSuffix(domain, "."+zone) || strings.HasSuffix(zone, "."+domain) && domain != "" {
				overlap = true
				if domain != zone {
					err = fmt.Errorf("foreign overlapping zone %q", domain)
					return
				}
				addr, _ := entry["address"].(string)
				p, good := entry["port"].(float64)
				if addr != host || !good || p != float64(port) {
					err = fmt.Errorf("zone %q changed owner", domain)
					return
				}
				owned[zone]++
				if owned[zone] > 1 {
					err = fmt.Errorf("duplicate zone %q", zone)
					return
				}
			}
		}
		if !overlap {
			remaining = append(remaining, item)
		}
	}
	doc["server"] = remaining
	stripped, e := json.Marshal(doc)
	if e != nil {
		err = e
		return
	}
	if string(stripped) != ledger.Baseline {
		err = errors.New("foreign DNS drift from exact activation baseline")
		return
	}
	for _, zone := range ledger.Zones {
		if owned[zone] == 1 {
			present = append(present, zone)
		} else {
			missing = append(missing, zone)
		}
	}
	sort.Strings(present)
	sort.Strings(missing)
	return
}

func (s *StartupManagerO3) status(ctx context.Context) (StartupResultO3, *LedgerO, error) {
	r := newStartupResultO3()
	if s == nil || s.Durable == nil {
		return r, nil, errors.New("startup recovery disabled")
	}
	m := s.Durable
	if err := m.guard(); err != nil {
		return r, nil, err
	}
	ledger, sha, err := m.Ledger.Load()
	if err != nil {
		return r, nil, err
	}
	if ledger == nil {
		r.State = "unmanaged"
		return r, nil, nil
	}
	r.Revision = ledger.Revision
	r.LedgerRevision = sha
	if ledger.Baseline == "" {
		return r, ledger, errors.New("durable startup requires exact saved baseline")
	}
	if ledger.State != "active" {
		r.State = ledger.State + "_recovery_required"
		r.RecoveryRequired = true
		return r, ledger, nil
	}
	cfg, rev, err := m.Store.Load()
	if err != nil {
		return r, ledger, err
	}
	if rev != ledger.Revision {
		r.State = "stale_policy"
		r.RecoveryRequired = true
		return r, ledger, nil
	}
	listener, zones, err := managedZonesLimitedN(cfg, 63)
	if err != nil {
		return r, ledger, err
	}
	if listener != ledger.Listen || len(zones) != len(ledger.Zones) {
		r.State = "stale_zone_plan"
		r.RecoveryRequired = true
		return r, ledger, nil
	}
	expected := map[string]bool{}
	for _, z := range zones {
		expected[z.Domain] = true
	}
	for _, z := range ledger.Zones {
		if !expected[z] {
			r.State = "stale_zone_plan"
			r.RecoveryRequired = true
			return r, ledger, nil
		}
	}
	snapshot, err := trialSnapshotM(ctx, m.Executor)
	if err != nil {
		return r, ledger, err
	}
	r.Present, r.Missing, err = classifyStartupO3(snapshot, *ledger)
	if err != nil {
		r.State = "blocked_foreign_drift"
		r.RecoveryRequired = true
		return r, ledger, err
	}
	if len(r.Missing) > 0 {
		r.State = "repairable_missing_owned_zones"
	} else {
		r.State = "present_verified_inventory"
	}
	return r, ledger, nil
}

// Inspect is read-only and never starts a listener, applies DNS zones or alters
// the ownership ledger. It is useful for showing fail-closed boot blockers.
func (s *StartupManagerO3) Inspect(ctx context.Context) (StartupResultO3, error) {
	r, _, err := s.status(ctx)
	if err != nil {
		r.Error = err.Error()
		r.RecoveryRequired = true
	}
	return r, err
}

// Recover requires explicit command-line opt-in, active exact-revision LAN
// listener, no competing watchdog and a valid durable active ownership ledger.
// A transaction guard is armed before re-adding ANY missing zone. If probes or
// registration fail the independent watchdog removes only the ledger-owned
// registrations; the active ledger is preserved for a future reconciliation.
func (s *StartupManagerO3) Recover(ctx context.Context) (out StartupResultO3, retErr error) {
	out = newStartupResultO3()
	if s == nil || s.Durable == nil {
		return out, errors.New("startup recovery disabled")
	}
	m := s.Durable
	if err := m.guard(); err != nil {
		return out, err
	}
	unlock, err := lockNDMCForO2()
	if err != nil {
		return out, err
	}
	defer unlock()
	if s.NoCompetingGuard != nil {
		err = s.NoCompetingGuard()
	} else {
		err = checkGuardsO3()
	}
	if err != nil {
		return out, err
	}
	out, ledger, err := s.status(ctx)
	if err != nil {
		return out, err
	}
	if ledger == nil {
		return out, nil
	} // never infer or adopt ownership
	if ledger.State != "active" || out.RecoveryRequired {
		return out, fmt.Errorf("manual recovery required for %s", out.State)
	}
	ready := false
	if s.Ready != nil {
		ready = s.Ready(ledger.Revision, ledger.Listen)
	} else if m.Lifecycle != nil {
		runtime := m.Lifecycle.Status()
		ready = runtime.Active && runtime.AppliedRevision == ledger.Revision && runtime.Listen == ledger.Listen && m.Lifecycle.Forwarder() != nil
	}
	if !ready {
		return out, errors.New("exact-revision managed LAN forwarder is not running")
	}
	if len(out.Missing) == 0 {
		if err = m.Probe(ctx, ledger.Zones); err != nil {
			return out, fmt.Errorf("startup forwarding probe failed: %w", err)
		}
		out.State = "already_active_verified"
		return out, nil
	}
	journal := managedJournalN{Extended63: true, Parent: os.Getpid(), Deadline: time.Now().Add(115 * time.Second).Unix(), Baseline: ledger.Baseline, Listen: ledger.Listen, Domains: append([]string(nil), ledger.Zones...)}
	guard, err := m.Arm(ctx, journal)
	if err != nil {
		return out, fmt.Errorf("startup rollback watchdog unavailable: %w", err)
	}
	out.WatchdogArmed = true
	committed := false
	defer func() {
		if committed {
			return
		}
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		if e := rollbackManagedN(rollbackCtx, m.Executor, ledger.Baseline, ledger.Listen, zonesFromLedgerO2(*ledger)); e != nil {
			out.State = "ambiguous"
			out.RecoveryRequired = true
			retErr = fmt.Errorf("startup failure: %v; exact rollback: %w", retErr, e)
			return
		}
		if e := guard.Finish(); e != nil {
			out.State = "ambiguous"
			out.RecoveryRequired = true
			retErr = fmt.Errorf("startup rollback watchdog acknowledgement: %w", e)
			return
		}
		out.State = "rolled_back_after_failure"
		out.Missing = append([]string(nil), ledger.Zones...)
	}()
	workCtx, cancel := context.WithTimeout(ctx, 65*time.Second)
	defer cancel()
	missing := append([]string(nil), out.Missing...)
	for _, z := range missing {
		if _, err = m.Executor.Command(workCtx, "ip name-server "+ledger.Listen+" "+z); err != nil {
			return out, err
		}
	}
	var verified bool
	for attempt := 0; attempt < 6; attempt++ {
		current, e := trialSnapshotM(workCtx, m.Executor)
		if e == nil {
			present, remaining, ce := classifyStartupO3(current, *ledger)
			verified = ce == nil && len(remaining) == 0 && len(present) == len(ledger.Zones)
		}
		if verified {
			break
		}
		select {
		case <-workCtx.Done():
			return out, workCtx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	if !verified {
		return out, errors.New("startup did not register all owned zones")
	}
	if err = m.Probe(workCtx, ledger.Zones); err != nil {
		return out, fmt.Errorf("startup forwarding probe failed: %w", err)
	}
	// No persistent ledger rewrite: ownership remains the explicit user's
	// original active ledger, and all mutations are bounded by its zones.
	if err = guard.Finish(); err != nil {
		committed = true // may still race watchdog: preserve exact diagnosis
		out.State = "ambiguous"
		out.RecoveryRequired = true
		return out, fmt.Errorf("startup watchdog ack failed: %w", err)
	}
	committed = true
	out.State = "recovered_active"
	out.Restored = missing
	out.Present = append([]string(nil), ledger.Zones...)
	out.Missing = []string{}
	out.Changed = true
	out.ReadOnly = false
	return out, nil
}
