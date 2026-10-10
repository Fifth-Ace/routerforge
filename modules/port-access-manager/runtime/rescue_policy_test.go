package main

import (
	"testing"
	"time"
)

func TestRescuePolicyRefusesUnverifiedPaths(t *testing.T) {
	now := time.Unix(2000, 0)
	base := newTransactionPlan(now)
	ok := RescuePolicy{ManagementPort: 22, RescuePort: 2222, ManagementVerified: true, RescueVerified: true, RollbackVerified: true, NDMOrderVerified: true, WANScopeVerified: true}
	invalid := []RescuePolicy{
		{},
		{ManagementPort: 22, RescuePort: 22, ManagementVerified: true, RescueVerified: true, RollbackVerified: true, NDMOrderVerified: true, WANScopeVerified: true},
		{ManagementPort: 22, RescuePort: 2222, ManagementVerified: true, RescueVerified: true, RollbackVerified: false, NDMOrderVerified: true, WANScopeVerified: true},
		{ManagementPort: 22, RescuePort: 2222, ManagementVerified: true, RescueVerified: false, RollbackVerified: true, NDMOrderVerified: true, WANScopeVerified: true},
		{ManagementPort: 22, RescuePort: 2222, ManagementVerified: true, RescueVerified: true, RollbackVerified: true, NDMOrderVerified: false, WANScopeVerified: true},
		{ManagementPort: 22, RescuePort: 2222, ManagementVerified: true, RescueVerified: true, RollbackVerified: true, NDMOrderVerified: true, WANScopeVerified: false},
	}
	for _, p := range invalid {
		x := simulatedRescueTransition(base, p, now.Unix()+1, true)
		if x.Stage != "rollback-required" || !x.RollbackRequired || x.ReadyForApply {
			t.Fatalf("unsafe state: %+v", x)
		}
	}
	if ok.validate() != nil {
		t.Fatal("valid fixture rejected")
	}
	live := simulatedRescueTransition(base, ok, now.Unix()+1, true)
	if live.Stage != "prepared" || live.ReadyForApply {
		t.Fatalf("simulation must remain inert: %+v", live)
	}
	for _, tc := range []struct {
		at int64
		up bool
	}{{now.Unix() + 1, false}, {now.Add(transactionWindow).Unix(), true}} {
		x := simulatedRescueTransition(base, ok, tc.at, tc.up)
		if x.Stage != "rollback-required" || !x.RollbackRequired || x.ReadyForApply {
			t.Fatalf("failed close: %+v", x)
		}
	}
}
