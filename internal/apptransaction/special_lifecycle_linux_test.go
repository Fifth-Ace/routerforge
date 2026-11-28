//go:build linux

package apptransaction

import (
	"context"
	"strings"
	"testing"
)

func TestSpecialLifecycleIntentDigest(t *testing.T) {
	a := DNSRouterEvidence{RuntimeVersion: "v1", PeerPID: 1, ServiceEnabled: true, AppliedRevision: "sha256:" + strings.Repeat("a", 64)}
	x, e := dnsRouterLifecycleCandidate(a)
	if e != nil {
		t.Fatal(e)
	}
	a.RuntimeVersion = "v2"
	a.PeerPID = 2
	y, e := dnsRouterLifecycleCandidate(a)
	if e != nil || x.IntentSHA256 != y.IntentSHA256 {
		t.Fatal("process replacement changed intent")
	}
	a.AppliedRevision = "sha256:" + strings.Repeat("b", 64)
	y, e = dnsRouterLifecycleCandidate(a)
	if e != nil || x.IntentSHA256 == y.IntentSHA256 {
		t.Fatal("revision change not detected")
	}
	c := CoreProfilingEvidence{RuntimeVersion: "v1", PeerPID: 1, Enabled: false, Running: false, Listen: "127.0.0.1:6061", SlowMS: 750}
	o, e := coreProfilingLifecycleCandidate("routerforge-profiling", c)
	if e != nil || o.Enabled || o.Mode != "running" {
		t.Fatal(o, e)
	}
	c.RuntimeVersion = "v2"
	c.PeerPID = 3
	p, e := coreProfilingLifecycleCandidate("routerforge-profiling", c)
	if e != nil || p.IntentSHA256 != o.IntentSHA256 {
		t.Fatal(p, e)
	}
	c.MarkerSHA256 = strings.Repeat("a", 64)
	p, e = coreProfilingLifecycleCandidate("routerforge-profiling", c)
	if e != nil || p.IntentSHA256 == o.IntentSHA256 {
		t.Fatal("marker drift missed")
	}
}
func TestSpecialLifecycleProofProfilingIndependentCoreVersion(t *testing.T) {
	s, p, pkg := lifecycleFixture(t)
	pkg.Name = "routerforge-profiling"
	pkg.Asset = strings.Replace(pkg.Asset, "routerforge-dns", "routerforge-profiling", 1)
	p.Packages = []Package{pkg}
	current := "1~dev.r1458.000000000000"
	core := "1~dev.r1457.111111111111"
	intent := strings.Repeat("a", 64)
	status := func(_ context.Context, p Package) (string, error) {
		if p.Name == "routerforge-core" {
			return core, nil
		}
		return current, nil
	}
	observation := LifecycleObservation{Mode: "running", Ready: true, RuntimeVersion: core, IntentSHA256: intent}
	proof, e := newLifecycleProof(s, p, status, func(context.Context, Package) (LifecycleObservation, error) { return observation, nil })
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	observation.IntentSHA256 = ""
	if proof.prepare(ctx, pkg) == nil {
		t.Fatal("missing special intent accepted")
	}
	observation.IntentSHA256 = intent
	if e = proof.prepare(ctx, pkg); e != nil {
		t.Fatal(e)
	}
	if e = proof.recheck(ctx, pkg); e != nil {
		t.Fatal(e)
	}
	current = pkg.Version
	observation.IntentSHA256 = strings.Repeat("b", 64)
	if proof.verify(ctx, pkg) == nil {
		t.Fatal("profiling intent drift accepted")
	}
	observation.IntentSHA256 = intent
	oldCore := core
	core = pkg.Version
	observation.RuntimeVersion = core
	if proof.verify(ctx, pkg) == nil {
		t.Fatal("unexpected Core version change accepted")
	}
	core = oldCore
	observation.RuntimeVersion = core
	if e = proof.verify(ctx, pkg); e != nil {
		t.Fatal(e)
	}
}
