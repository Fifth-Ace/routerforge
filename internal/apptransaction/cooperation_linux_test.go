//go:build linux

package apptransaction

import "testing"

func TestCooperativeTransitionRequiresExactIntermediateVectors(t *testing.T) {
	_, p, pkg := lifecycleFixture(t)
	core := pkg
	core.Name = "routerforge-core"
	core.Asset = "routerforge-core" + pkg.Asset[len("routerforge-dns"):]
	p.Packages = append(p.Packages, core)
	old := "1~dev.r1458.000000000000"
	initial := CooperativeVersionSet{}
	for n := range official {
		initial[n] = ""
	}
	initial[pkg.Name] = old
	initial[core.Name] = old
	copySet := func(s CooperativeVersionSet) CooperativeVersionSet {
		out := CooperativeVersionSet{}
		for n, v := range s {
			out[n] = v
		}
		return out
	}
	intermediate := copySet(initial)
	intermediate[pkg.Name] = pkg.Version
	final := copySet(intermediate)
	final[core.Name] = core.Version
	if e := CheckCooperativeTransition(p, initial, []CooperativeVersionSet{initial, intermediate, final}); e != nil {
		t.Fatal(e)
	}
	if e := CheckCooperativeTransition(p, initial, []CooperativeVersionSet{initial, final}); e == nil {
		t.Fatal("missing intermediate accepted")
	}
	if e := CheckCooperativeTransition(p, initial, nil); e == nil {
		t.Fatal("empty acceptance accepted")
	}
	drift := copySet(initial)
	drift["routerforge-admin"] = "1~dev.r1456.111111111111"
	if e := CheckCooperativeTransition(p, drift, []CooperativeVersionSet{initial, intermediate, final}); e == nil {
		t.Fatal("unaccepted mixed version")
	}
	delete(drift, "routerforge-admin")
	drift["dns-monitor-admin"] = old
	if e := CheckCooperativeTransition(p, drift, []CooperativeVersionSet{initial, intermediate, final}); e == nil {
		t.Fatal("legacy alias accepted")
	}
	if initial[pkg.Name] != old {
		t.Fatal("inventory mutated")
	}
}
