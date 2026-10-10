package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// commandRunner permits tests to prove cleanup without touching a router.
type commandRunner interface {
	Run(context.Context, []string) error
}

// isolatedExecutor can create and remove only its three unhooked module-owned
// chains. It deliberately cannot link INPUT or FORWARD, and never grants access.
type isolatedExecutor struct {
	runner  commandRunner
	timeout time.Duration
}

var isolatedChainNames = map[string]bool{"RF_PORT_KNOCK": true, "RF_KNOCK_STEP2": true, "RF_KNOCK_STEP3": true}

func validateIsolatedPlan(commands [][]string) error {
	if len(commands) == 0 || len(commands) > 32 {
		return errors.New("invalid isolated plan size")
	}
	created := map[string]bool{}
	for _, cmd := range commands {
		if len(cmd) < 5 || cmd[0] != "iptables" || cmd[1] != "-t" || cmd[2] != "filter" {
			return errors.New("unsupported firewall command")
		}
		action, chain := cmd[3], cmd[4]
		if !isolatedChainNames[chain] || (action != "-N" && action != "-A") {
			return errors.New("command outside isolated scope")
		}
		if action == "-N" {
			if len(cmd) != 5 || created[chain] {
				return errors.New("duplicate or malformed chain creation")
			}
			created[chain] = true
			continue
		}
		if !created[chain] || len(cmd) < 7 {
			return errors.New("append without created chain")
		}
		for _, token := range cmd[5:] {
			if strings.ContainsAny(token, "\n\r\x00") {
				return errors.New("invalid rule argument")
			}
			if token == "ACCEPT" || token == "-I" || token == "-D" || token == "-F" || token == "-X" {
				return errors.New("forbidden rule action")
			}
		}
		// Only the known stage transitions, local return, and target DROP may appear.
		validTarget := false
		for i := 5; i+1 < len(cmd); i++ {
			if cmd[i] == "-j" && (cmd[i+1] == "RETURN" || cmd[i+1] == "DROP" || cmd[i+1] == "RF_KNOCK_STEP2" || cmd[i+1] == "RF_KNOCK_STEP3") {
				validTarget = true
			}
		}
		if !validTarget {
			return errors.New("unknown jump target")
		}
	}
	if len(created) != len(isolatedChainNames) {
		return errors.New("missing isolated chain")
	}
	return nil
}

// exercise creates only unhooked chains, leaves them for a bounded verification
// callback, then ALWAYS attempts rollback (including timeout or verifier error).
// It never commits persistent firewall state.
func (e isolatedExecutor) exercise(ctx context.Context, plan KnockRules, verify func(context.Context) error) (err error) {
	if e.runner == nil || verify == nil || e.timeout < time.Second || e.timeout > 120*time.Second {
		return errors.New("invalid isolated exercise")
	}
	if plan.Hooked || plan.Applied || plan.StrictSequenceVerified || !plan.DeploymentBlocked {
		return errors.New("unsafe plan state")
	}
	if err := validateCanonicalIsolatedPlan(plan.Commands); err != nil {
		return err
	}
	activeCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	// Track ownership: if -N fails because a chain already exists, never
	// flush or delete that pre-existing chain during cleanup.
	created := make([]string, 0, 3)
	defer func() {
		rollbackCtx, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		for i := len(created) - 1; i >= 0; i-- {
			cmd := []string{"iptables", "-t", "filter", "-F", created[i]}
			if runErr := e.runner.Run(rollbackCtx, cmd); runErr != nil {
				err = errors.Join(err, fmt.Errorf("flush %s: %w", created[i], runErr))
			}
		}
		for i := len(created) - 1; i >= 0; i-- {
			cmd := []string{"iptables", "-t", "filter", "-X", created[i]}
			if runErr := e.runner.Run(rollbackCtx, cmd); runErr != nil {
				err = errors.Join(err, fmt.Errorf("delete %s: %w", created[i], runErr))
			}
		}
	}()
	for _, cmd := range plan.Commands {
		if activeCtx.Err() != nil {
			return activeCtx.Err()
		}
		if runErr := e.runner.Run(activeCtx, cmd); runErr != nil {
			return fmt.Errorf("stage isolated chain: %w", runErr)
		}
		if cmd[3] == "-N" {
			created = append(created, cmd[4])
		}
	}
	return verify(activeCtx)
}
