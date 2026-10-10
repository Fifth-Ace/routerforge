package main

import (
	"testing"
	"time"
)

func TestTransactionSimulationFailClosed(t *testing.T) {
	now := time.Unix(1000, 0)
	base := newTransactionPlan(now)
	if base.ReadyForApply || base.RollbackRequired || base.Stage != "prepared" {
		t.Fatal(base)
	}
	simulated, err := base.beginSimulation(now.Add(time.Second))
	if err != nil || !simulated.RollbackRequired || simulated.ReadyForApply {
		t.Fatalf("%+v: %v", simulated, err)
	}
	if _, err := simulated.confirmSimulation(now.Add(2*time.Second), false); err == nil {
		t.Fatal("accepted lost management")
	}
	timedOut := simulated.expire(now.Add(transactionWindow))
	if timedOut.Stage != "rollback-required" || !timedOut.RollbackRequired || timedOut.ReadyForApply {
		t.Fatal(timedOut)
	}
	if _, err := timedOut.confirmSimulation(now.Add(transactionWindow), true); err == nil {
		t.Fatal("accepted expired confirmation")
	}
	confirmed, err := simulated.confirmSimulation(now.Add(2*time.Second), true)
	if err != nil || !confirmed.Confirmed || confirmed.RollbackRequired || confirmed.ReadyForApply {
		t.Fatalf("%+v: %v", confirmed, err)
	}
	if base.abort().Stage != "rollback-required" {
		t.Fatal("abort failed")
	}
	if _, err := base.beginSimulation(now.Add(transactionWindow)); err == nil {
		t.Fatal("accepted expired plan")
	}
}
