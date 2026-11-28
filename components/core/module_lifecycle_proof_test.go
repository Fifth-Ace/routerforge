package main

import "testing"

func TestModuleLifecycleStateProof(t *testing.T) {
	spec := moduleLifecycleSpec{ProcessNames: []string{"rf-test"}}
	if moduleLifecycleStateMatches(spec, nil, false) || moduleLifecycleStateMatches(moduleLifecycleSpec{}, map[string]bool{"other": true}, false) {
		t.Fatal("missing observation accepted")
	}
	if !moduleLifecycleStateMatches(spec, map[string]bool{"other": true}, false) || !moduleLifecycleStateMatches(spec, map[string]bool{"rf-test": true}, true) {
		t.Fatal("expected state rejected")
	}
	if moduleLifecycleStateMatches(spec, map[string]bool{"rf-test": true}, false) || moduleLifecycleStateMatches(spec, map[string]bool{"other": true}, true) {
		t.Fatal("unexpected state accepted")
	}
	item := &catalogItem{Installed: true, ServiceRunning: false, Disabled: true}
	if !moduleLifecycleResultMatches(item, false, true) || moduleLifecycleResultMatches(item, true, true) || moduleLifecycleResultMatches(item, false, false) || moduleLifecycleResultMatches(nil, false, true) {
		t.Fatal("postcondition mismatch")
	}
	item.Installed = false
	if moduleLifecycleResultMatches(item, false, true) {
		t.Fatal("missing package accepted")
	}
}
