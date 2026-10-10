package main

import (
	"errors"
	"time"
)

// TransactionPlan models fail-closed deadlines. No method performs I/O or
// mutates a firewall. Real execution needs a separate, audited implementation.
type TransactionPlan struct {
	Stage            string    `json:"stage"`
	Deadline         time.Time `json:"deadline"`
	Confirmed        bool      `json:"confirmed"`
	RollbackRequired bool      `json:"rollback_required"`
	ReadyForApply    bool      `json:"ready_for_apply"`
}

const transactionWindow = 120 * time.Second

func newTransactionPlan(now time.Time) TransactionPlan {
	return TransactionPlan{Stage: "prepared", Deadline: now.Add(transactionWindow), ReadyForApply: false}
}

func (p TransactionPlan) beginSimulation(now time.Time) (TransactionPlan, error) {
	if p.Stage != "prepared" || !now.Before(p.Deadline) {
		return p, errors.New("invalid or expired transaction plan")
	}
	p.Stage = "simulated"
	p.RollbackRequired = true
	return p, nil
}

func (p TransactionPlan) confirmSimulation(now time.Time, managementReachable bool) (TransactionPlan, error) {
	if p.Stage != "simulated" || !now.Before(p.Deadline) || !managementReachable {
		return p, errors.New("confirmation rejected: invalid stage, expired timer or management unreachable")
	}
	p.Stage = "confirmed-simulation"
	p.Confirmed = true
	p.RollbackRequired = false
	return p, nil
}

func (p TransactionPlan) expire(now time.Time) TransactionPlan {
	if p.Stage == "simulated" && !now.Before(p.Deadline) {
		p.Stage = "rollback-required"
		p.RollbackRequired = true
	}
	return p
}

func (p TransactionPlan) abort() TransactionPlan {
	if p.Stage != "confirmed-simulation" {
		p.Stage = "rollback-required"
		p.RollbackRequired = true
	}
	return p
}
