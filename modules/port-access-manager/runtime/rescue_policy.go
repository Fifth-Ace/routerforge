package main

import (
	"errors"
	"time"
)

// RescuePolicy is a pure simulation input, never an authorization to alter rules.
type RescuePolicy struct {
	ManagementPort     int
	RescuePort         int
	ManagementVerified bool
	RescueVerified     bool
	RollbackVerified   bool
	NDMOrderVerified   bool
	WANScopeVerified   bool
}

func (p RescuePolicy) validate() error {
	if p.ManagementPort < 1 || p.ManagementPort > 65535 ||
		p.RescuePort < 1 || p.RescuePort > 65535 ||
		p.ManagementPort == p.RescuePort {
		return errors.New("distinct management and rescue ports are required")
	}
	if !p.ManagementVerified || !p.RescueVerified ||
		!p.RollbackVerified || !p.NDMOrderVerified || !p.WANScopeVerified {
		return errors.New("management, rescue, rollback, NDM and WAN evidence must all be independently verified")
	}
	return nil
}

// simulatedRescueTransition never opens a firewall or authorizes applying.
func simulatedRescueTransition(plan TransactionPlan, policy RescuePolicy, nowUnix int64, linkUp bool) TransactionPlan {
	if policy.validate() != nil || !linkUp {
		return plan.abort()
	}
	now := time.Unix(nowUnix, 0)
	if !now.Before(plan.Deadline) {
		return plan.expire(now).abort()
	}
	return plan
}
