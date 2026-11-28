//go:build linux

package apptransaction

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lifecycleFixture(t *testing.T) (*Store, Plan, Package) {
	t.Helper()
	d := t.TempDir()
	if err := os.Chmod(d, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(d)
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.Repeat("a", 40)
	v := "1~dev.r1459." + commit[:12]
	pkg := Package{Name: "routerforge-dns", Version: v, SHA256: strings.Repeat("b", 64), Size: 123, Asset: "routerforge-dns_" + strings.ReplaceAll(v, "~", "-") + "_aarch64-3.10.ipk"}
	p := Plan{ID: "proof-test", Channel: "dev", Target: "aarch64-3.10", Commit: commit, Packages: []Package{pkg}}
	return s, p, pkg
}

func TestLifecycleProofPreservesRuntimeAndPersists(t *testing.T) {
	for _, mode := range []string{"running", "stopped", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			s, p, pkg := lifecycleFixture(t)
			ctx := context.Background()
			v := "1~dev.r1458.000000000000"
			enabled := mode != "disabled"
			proof, err := newLifecycleProof(s, p, func(context.Context, Package) (string, error) { return v, nil }, func(context.Context, Package) (LifecycleObservation, error) {
				o := LifecycleObservation{Mode: mode, Enabled: enabled}
				if mode == "running" {
					o.Ready = true
					o.RuntimeVersion = v
				}
				return o, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if err = proof.prepare(ctx, pkg); err != nil {
				t.Fatal(err)
			}
			if err = proof.recheck(ctx, pkg); err != nil {
				t.Fatal(err)
			}
			enabled = !enabled
			if proof.recheck(ctx, pkg) == nil {
				t.Fatal("intent drift accepted")
			}
			enabled = !enabled
			if proof.verify(ctx, pkg) == nil {
				t.Fatal("old installed version accepted")
			}
			v = pkg.Version
			if err = proof.verify(ctx, pkg); err != nil {
				t.Fatal(err)
			}
			if proof.verify(ctx, pkg) == nil {
				t.Fatal("evidence overwrite accepted")
			}
			for _, stage := range []string{"before", "after"} {
				b, e := os.ReadFile(filepath.Join(s.dir, "lifecycle-"+stage+"-"+pkg.Name+".json"))
				if e != nil {
					t.Fatal(e)
				}
				var got lifecycleEvidence
				if e = json.Unmarshal(b, &got); e != nil || got.Package != pkg.Name || got.PlanSHA256 != proof.hash || got.Observation.Mode != mode {
					t.Fatal(got, e)
				}
			}
		})
	}
}

func TestLifecycleProofRejectsUnknownAndUnhealthy(t *testing.T) {
	for _, obs := range []LifecycleObservation{{}, {Mode: "running", Ready: true, RuntimeVersion: "old"}, {Mode: "running", Ready: false}, {Mode: "disabled", Enabled: true}, {Mode: "stopped", Ready: true}} {
		s, p, pkg := lifecycleFixture(t)
		v := "1~dev.r1458.000000000000"
		proof, err := newLifecycleProof(s, p, func(context.Context, Package) (string, error) { return v, nil }, func(context.Context, Package) (LifecycleObservation, error) { return obs, nil })
		if err != nil {
			t.Fatal(err)
		}
		if proof.prepare(context.Background(), pkg) == nil {
			t.Fatalf("accepted %+v", obs)
		}
		if _, err = os.Stat(filepath.Join(s.dir, "lifecycle-before-"+pkg.Name+".json")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
}

type lifecycleFixtureHooks struct {
	install int
	verify  func(context.Context, Package) error
	mutate  func()
}

func (h *lifecycleFixtureHooks) Prepare(context.Context, Package) error { return nil }
func (h *lifecycleFixtureHooks) Check(context.Context, Package) error   { return nil }
func (h *lifecycleFixtureHooks) Install(context.Context, Package) error {
	h.install++
	h.mutate()
	return nil
}
func (h *lifecycleFixtureHooks) Verify(ctx context.Context, p Package) error { return h.verify(ctx, p) }

func TestLifecycleProofWorkerStopsAndNeverReplays(t *testing.T) {
	for _, which := range []string{"pre", "post"} {
		t.Run(which, func(t *testing.T) {
			s, p, pkg := lifecycleFixture(t)
			v := "1~dev.r1458.000000000000"
			o := LifecycleObservation{Mode: "running", Enabled: true, Ready: true, RuntimeVersion: v}
			if which == "pre" {
				o.Ready = false
			}
			proof, err := newLifecycleProof(s, p, func(context.Context, Package) (string, error) { return v, nil }, func(context.Context, Package) (LifecycleObservation, error) { return o, nil })
			if err != nil {
				t.Fatal(err)
			}
			base := &lifecycleFixtureHooks{verify: proof.verify, mutate: func() { v = pkg.Version; o.RuntimeVersion = v; o.Ready = false }}
			h := &lifecycleProofHooks{Hooks: base, proof: proof}
			j, err := s.Execute(context.Background(), p, h)
			if err == nil {
				t.Fatal("failure accepted")
			}
			if which == "pre" && (j.Phase != Failed || base.install != 0) {
				t.Fatal(j, base.install)
			}
			if which == "post" && (j.Phase != RecoveryRequired || base.install != 1) {
				t.Fatal(j, base.install)
			}
			n := base.install
			if _, err = s.Execute(context.Background(), p, h); !errors.Is(err, ErrRecovery) || base.install != n {
				t.Fatal(err, base.install)
			}
		})
	}
}
