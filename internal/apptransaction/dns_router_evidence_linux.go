//go:build linux

package apptransaction

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// DNSRouterEvidence is a management-plane candidate, not a hardware or full
// lifecycle proof. In particular, a running supervisor is not DNS ON.
// Connected client profiles are excluded until native route ownership has
// been independently verified. No repair, migration or DNS control is invoked.
type DNSRouterEvidence struct {
	RuntimeVersion      string
	PeerPID             int
	ServiceEnabled      bool
	InterceptionEnabled bool
	AppliedRevision     string
	GateConsent         string
}

// ProbeDNSRouterEvidence uses the known native socket, not the Core proxy.
// installedVersion must be independently read from the installed package.
func ProbeDNSRouterEvidence(ctx context.Context, installedVersion string) (DNSRouterEvidence, error) {
	return probeDNSRouterEvidence(ctx, "/opt/var/run/routerforge-dns-router.sock", "/opt/bin/routerforge-dns-router", 0, installedVersion)
}
func probeDNSRouterEvidence(ctx context.Context, socket, binary string, uid uint32, version string) (DNSRouterEvidence, error) {
	var out DNSRouterEvidence
	if ctx == nil || version == "" || len(version) > 128 {
		return out, errors.New("missing DNS Router identity")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	startup := func(body []byte) error { return validateDNSRouterStartup(body, version) }
	a, err := probeNativeEndpoint(ctx, socket, binary, uid, "/v1/startup", startup)
	if err != nil {
		return out, err
	}
	var state DNSRouterEvidence
	b, err := probeNativeEndpoint(ctx, socket, binary, uid, "/v1/service/status", func(body []byte) error { var e error; state, e = decodeDNSRouterEvidence(body); return e })
	if err != nil {
		return out, err
	}
	c, err := probeNativeEndpoint(ctx, socket, binary, uid, "/v1/startup", startup)
	if err != nil {
		return out, err
	}
	// Re-observe intent/state after the second startup check, also guarding
	// against changes to the endpoint or the installed executable.
	var again DNSRouterEvidence
	d, err := probeNativeEndpoint(ctx, socket, binary, uid, "/v1/service/status", func(body []byte) error { var e error; again, e = decodeDNSRouterEvidence(body); return e })
	if err != nil {
		return out, err
	}
	if !a.same(b) || !a.same(c) || !a.same(d) || state != again || ctx.Err() != nil {
		return out, errors.New("DNS Router identity or intent drift")
	}
	state.RuntimeVersion = version
	state.PeerPID = a.pid
	return state, nil
}
func requiredNativeField[T any](fields map[string]json.RawMessage, key string) (T, error) {
	var v T
	raw, ok := fields[key]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &v) != nil {
		return v, errors.New("missing or invalid native field: " + key)
	}
	return v, nil
}
func optionalNativeString(fields map[string]json.RawMessage, key string) (string, error) {
	if _, ok := fields[key]; !ok {
		return "", nil
	}
	return requiredNativeField[string](fields, key)
}
func validateDNSRouterStartup(body []byte, version string) error {
	f, e := nativeObject(body)
	if e != nil {
		return e
	}
	c, e1 := requiredNativeField[string](f, "component")
	v, e2 := requiredNativeField[string](f, "version")
	s, e3 := requiredNativeField[string](f, "state")
	if e1 != nil || e2 != nil || e3 != nil || c != "dns-router" || v != version || s != "READY" {
		return errors.New("DNS Router startup identity or readiness rejected")
	}
	return nil
}
func evidenceRevision(s string) bool {
	if !strings.HasPrefix(s, "sha256:") || len(s) != 71 || strings.ToLower(s) != s {
		return false
	}
	_, e := hex.DecodeString(s[7:])
	return e == nil
}
func decodeDNSRouterEvidence(body []byte) (DNSRouterEvidence, error) {
	var out DNSRouterEvidence
	f, e := nativeObject(body)
	if e != nil {
		return out, e
	}
	mode, e1 := requiredNativeField[string](f, "mode")
	active, e2 := requiredNativeField[bool](f, "active")
	maintenance, e3 := requiredNativeField[bool](f, "maintenance")
	interception, e4 := requiredNativeField[bool](f, "interception")
	last, e5 := requiredNativeField[string](f, "last_error")
	revision, e6 := requiredNativeField[string](f, "applied_revision")
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || e6 != nil || mode != "independent-dns" || maintenance || last != "" {
		return out, errors.New("DNS Router service not proven ready")
	}
	desired, e := nativeObject(f["desired"])
	if e != nil {
		return out, e
	}
	schema, e1 := requiredNativeField[int](desired, "version")
	on, e2 := requiredNativeField[bool](desired, "service_enabled")
	gate, e3 := requiredNativeField[bool](desired, "interception_enabled")
	applied, e4 := optionalNativeString(desired, "applied_revision")
	consent, e5 := optionalNativeString(desired, "gate_consent")
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || schema != 1 || active != on || interception != gate {
		return out, errors.New("DNS Router desired/runtime state mismatch")
	}
	if (applied != "" && !evidenceRevision(applied)) || (on && (!evidenceRevision(applied) || revision != applied)) || (!on && revision != "") || (gate && (!on || (consent != "explicit-enable-v1" && consent != "verified-legacy-live-active"))) || (!gate && consent != "") {
		return out, errors.New("DNS Router applied revision or gate consent rejected")
	}
	conn, e := nativeObject(f["connection"])
	if e != nil {
		return out, e
	}
	s, e := requiredNativeField[string](conn, "state")
	if e != nil || s != "disconnected" {
		return out, errors.New("DNS Router client profile is not proven disconnected")
	}
	return DNSRouterEvidence{ServiceEnabled: on, InterceptionEnabled: gate, AppliedRevision: applied, GateConsent: consent}, nil
}

// CompareDNSRouterIntent compares two already validated native observations.
// It does not authorize an upgrade or assert hardware readiness.
func CompareDNSRouterIntent(before, after DNSRouterEvidence, target string) error {
	if target == "" || before.RuntimeVersion == "" || after.RuntimeVersion != target || before.PeerPID <= 0 || after.PeerPID <= 0 || before.ServiceEnabled != after.ServiceEnabled || before.InterceptionEnabled != after.InterceptionEnabled || before.AppliedRevision != after.AppliedRevision || before.GateConsent != after.GateConsent {
		return errors.New("DNS Router version or intent changed")
	}
	return nil
}
