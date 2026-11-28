//go:build linux

package apptransaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// LifecycleObservation is supplied by a trusted, package-specific local probe.
// A process name or an installed package version alone is not a running proof.
// RuntimeVersion must be obtained from the running implementation itself.
// A disabled/stopped observation requires positively established non-running
// state; unknown or unsupported must be returned as an error, not "stopped".
type LifecycleObservation struct {
	Mode           string `json:"mode"`
	Enabled        bool   `json:"enabled"`
	Ready          bool   `json:"ready"`
	RuntimeVersion string `json:"runtime_version,omitempty"`
	IntentSHA256   string `json:"intent_sha256,omitempty"`
}

type LifecycleProbe func(context.Context, Package) (LifecycleObservation, error)

type lifecycleEvidence struct {
	PlanSHA256       string               `json:"plan_sha256"`
	Package          string               `json:"package"`
	InstalledVersion string               `json:"installed_version"`
	Observation      LifecycleObservation `json:"observation"`
}

func validateLifecycleObservation(o LifecycleObservation, installed string) error {
	if _, err := devRevision(installed); err != nil {
		return err
	}
	switch o.Mode {
	case "running":
		if !o.Ready || o.RuntimeVersion != installed {
			return errors.New("running lifecycle version/readiness not proven")
		}
	case "stopped", "disabled":
		if o.Ready || o.RuntimeVersion != "" || (o.Mode == "disabled" && o.Enabled) {
			return errors.New("inconsistent stopped/disabled lifecycle evidence")
		}
	default:
		return errors.New("unknown or unsupported lifecycle observation")
	}
	return nil
}

type lifecycleProof struct {
	store  *Store
	hash   string
	status func(context.Context, Package) (string, error)
	probe  LifecycleProbe
	before map[string]lifecycleEvidence
}

func newLifecycleProof(s *Store, p Plan, status func(context.Context, Package) (string, error), probe LifecycleProbe) (*lifecycleProof, error) {
	if s == nil || status == nil || probe == nil {
		return nil, errors.New("missing lifecycle proof dependencies")
	}
	h, err := p.Hash()
	if err != nil {
		return nil, err
	}
	return &lifecycleProof{store: s, hash: h, status: status, probe: probe, before: map[string]lifecycleEvidence{}}, nil
}

func (l *lifecycleProof) observe(ctx context.Context, p Package) (lifecycleEvidence, error) {
	if !official[p.Name] {
		return lifecycleEvidence{}, errors.New("unsupported lifecycle package")
	}
	v, err := l.status(ctx, p)
	if err != nil {
		return lifecycleEvidence{}, err
	}
	// Profiling runs inside Core, not a separate executable. Its package version
	// and the independently installed Core runtime version are distinct evidence.
	runtimeExpected := v
	if p.Name == "routerforge-profiling" {
		if _, err = devRevision(v); err != nil {
			return lifecycleEvidence{}, err
		}
		runtimeExpected, err = l.status(ctx, Package{Name: "routerforge-core"})
		if err != nil {
			return lifecycleEvidence{}, err
		}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	o, err := l.probe(probeCtx, p)
	if err != nil {
		return lifecycleEvidence{}, err
	}
	if err = probeCtx.Err(); err != nil {
		return lifecycleEvidence{}, err
	}
	if err = validateLifecycleObservation(o, runtimeExpected); err != nil {
		return lifecycleEvidence{}, err
	}
	if o.IntentSHA256 != "" && !digest.MatchString(o.IntentSHA256) {
		return lifecycleEvidence{}, errors.New("invalid lifecycle intent digest")
	}
	if specialLifecyclePackage(p.Name) && (o.Mode != "running" || !digest.MatchString(o.IntentSHA256)) {
		return lifecycleEvidence{}, errors.New("special lifecycle intent not proven")
	}
	if p.Name == "routerforge-core" && !o.Enabled {
		return lifecycleEvidence{}, errors.New("Core running intent not proven")
	}
	if p.Name == "routerforge-profiling" {
		again, err := l.status(ctx, Package{Name: "routerforge-core"})
		if err != nil || again != runtimeExpected {
			return lifecycleEvidence{}, errors.New("Core package version drift during Profiling probe")
		}
	}
	// Detect package metadata drift while probing the runtime.
	again, err := l.status(ctx, p)
	if err != nil {
		return lifecycleEvidence{}, err
	}
	if again != v {
		return lifecycleEvidence{}, errors.New("package version changed during lifecycle probe")
	}
	return lifecycleEvidence{l.hash, p.Name, v, o}, nil
}

func (l *lifecycleProof) save(stage string, e lifecycleEvidence) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return durableExclusive(l.store, "lifecycle-"+stage+"-"+e.Package+".json", b)
}

func (l *lifecycleProof) prepare(ctx context.Context, p Package) error {
	if _, ok := l.before[p.Name]; ok {
		return errors.New("duplicate lifecycle pre-proof")
	}
	e, err := l.observe(ctx, p)
	if err != nil {
		return err
	}
	if err = allowUpgrade(e.InstalledVersion, p.Version); err != nil {
		return err
	}
	if err = l.save("before", e); err != nil {
		return err
	}
	l.before[p.Name] = e
	return nil
}

func (l *lifecycleProof) recheck(ctx context.Context, p Package) error {
	old, ok := l.before[p.Name]
	if !ok {
		return errors.New("missing lifecycle pre-proof")
	}
	e, err := l.observe(ctx, p)
	if err != nil {
		return err
	}
	if e != old {
		return errors.New("lifecycle drift before installation")
	}
	return nil
}

func (l *lifecycleProof) verify(ctx context.Context, p Package) error {
	old, ok := l.before[p.Name]
	if !ok {
		return errors.New("missing lifecycle pre-proof")
	}
	e, err := l.observe(ctx, p)
	if err != nil {
		return err
	}
	if e.InstalledVersion != p.Version || e.Observation.Mode != old.Observation.Mode || e.Observation.Enabled != old.Observation.Enabled || e.Observation.IntentSHA256 != old.Observation.IntentSHA256 || (p.Name == "routerforge-profiling" && e.Observation.RuntimeVersion != old.Observation.RuntimeVersion) {
		return fmt.Errorf("installed version or lifecycle intent not preserved: %s", p.Name)
	}
	return l.save("after", e)
}

// All packages are prepared before Store.Execute enters its mutation loop.
// Recheck again immediately before the install. Any uncertainty stops the
// existing engine; it must not silently replay or repair the failed operation.
type lifecycleProofHooks struct {
	Hooks
	proof *lifecycleProof
}

func (h *lifecycleProofHooks) Prepare(ctx context.Context, p Package) error {
	if err := h.Hooks.Prepare(ctx, p); err != nil {
		return err
	}
	return h.proof.prepare(ctx, p)
}
func (h *lifecycleProofHooks) Install(ctx context.Context, p Package) error {
	if err := h.proof.recheck(ctx, p); err != nil {
		return err
	}
	return h.Hooks.Install(ctx, p)
}
