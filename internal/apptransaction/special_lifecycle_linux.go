//go:build linux

package apptransaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

func specialLifecyclePackage(name string) bool {
	return name == "routerforge-core" || name == "routerforge-profiling" || name == "routerforge-dns-router"
}
func lifecycleIntentHash(v any) (string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func dnsRouterLifecycleCandidate(e DNSRouterEvidence) (LifecycleObservation, error) {
	if e.PeerPID <= 0 || e.RuntimeVersion == "" {
		return LifecycleObservation{}, errors.New("missing DNS Router runtime evidence")
	}
	// PID and executable version change across a valid upgrade; user intent must not.
	h, err := lifecycleIntentHash(struct {
		Service, Gate     bool
		Revision, Consent string
	}{e.ServiceEnabled, e.InterceptionEnabled, e.AppliedRevision, e.GateConsent})
	return LifecycleObservation{Mode: "running", Enabled: e.ServiceEnabled, Ready: true, RuntimeVersion: e.RuntimeVersion, IntentSHA256: h}, err
}
func coreProfilingLifecycleCandidate(name string, e CoreProfilingEvidence) (LifecycleObservation, error) {
	if (name != "routerforge-core" && name != "routerforge-profiling") || e.PeerPID <= 0 || e.RuntimeVersion == "" {
		return LifecycleObservation{}, errors.New("missing Core/Profiling runtime evidence")
	}
	h, err := lifecycleIntentHash(struct {
		Enabled, Running bool
		Listen           string
		Slow             int
		Marker, Config   string
	}{e.Enabled, e.Running, e.Listen, e.SlowMS, e.MarkerSHA256, e.ConfigSHA256})
	return LifecycleObservation{Mode: "running", Enabled: name == "routerforge-core" || e.Enabled, Ready: true, RuntimeVersion: e.RuntimeVersion, IntentSHA256: h}, err
}

// ProbeSpecialLifecycleCandidate bridges the existing local read-only probes to
// the journal contract. The caller must obtain installed versions independently.
// It is NOT wired into an executable worker: disk migration, full DNS dataplane,
// mixed-version cooperation and hardware lease lifetime remain acceptance gates.
func ProbeSpecialLifecycleCandidate(ctx context.Context, name, installedVersion, coreVersion string) (LifecycleObservation, error) {
	if _, err := devRevision(installedVersion); err != nil {
		return LifecycleObservation{}, err
	}
	switch name {
	case "routerforge-dns-router":
		e, err := ProbeDNSRouterEvidence(ctx, installedVersion)
		if err != nil {
			return LifecycleObservation{}, err
		}
		return dnsRouterLifecycleCandidate(e)
	case "routerforge-core", "routerforge-profiling":
		if _, err := devRevision(coreVersion); err != nil {
			return LifecycleObservation{}, err
		}
		if name == "routerforge-core" && installedVersion != coreVersion {
			return LifecycleObservation{}, errors.New("Core package identity mismatch")
		}
		e, err := ProbeCoreProfilingEvidence(ctx, coreVersion)
		if err != nil {
			return LifecycleObservation{}, err
		}
		return coreProfilingLifecycleCandidate(name, e)
	default:
		return LifecycleObservation{}, errors.New("unsupported special lifecycle package")
	}
}
