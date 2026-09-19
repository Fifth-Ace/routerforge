package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
)

var benchSessionIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{7,39}$`)

type benchRuleSpec struct {
	Name     string   `json:"name"`
	Table    string   `json:"table"`
	Chain    string   `json:"chain"`
	Relation string   `json:"relation"`
	Anchor   string   `json:"anchor"`
	Priority int      `json:"priority"`
	Comment  string   `json:"comment"`
	RuleArgs []string `json:"rule_args"`
}

type benchTransactionSpec struct {
	SessionID       string `json:"session_id"`
	DestinationIPv4 string `json:"destination_ipv4"`
	LocalPort       int    `json:"local_port"`
	Queue           int    `json:"queue"`
}

type benchTransactionContract struct {
	Implemented              bool `json:"implemented"`
	MutationEnabled          bool `json:"mutation_enabled"`
	ControlledSmokeOnly      bool `json:"controlled_smoke_only"`
	SystemMutatorImplemented bool `json:"system_mutator_implemented"`
	RuleCount                int  `json:"rule_count"`
	ReverseOrderRollback     bool `json:"reverse_order_rollback"`
	CleanupMandatory         bool `json:"cleanup_mandatory"`
	CleanupVerification      bool `json:"cleanup_verification"`
	FailClosed               bool `json:"fail_closed"`
	CandidateStartedFirst    bool `json:"candidate_started_first"`
	ProductionConfigMutation bool `json:"production_config_mutation"`
	ProductionRestart        bool `json:"production_restart"`
}

type benchTransactionResult struct {
	State            string   `json:"state"`
	AppliedRules     []string `json:"applied_rules"`
	CleanupAttempted bool     `json:"cleanup_attempted"`
	CleanupProven    bool     `json:"cleanup_proven"`
	Error            string   `json:"error,omitempty"`
	CleanupErrors    []string `json:"cleanup_errors"`
}

type benchTransactionOps interface {
	StartCandidate(context.Context, benchTransactionSpec) error
	InstallRule(context.Context, benchRuleSpec) error
	VerifyInstalled(context.Context, benchTransactionSpec, []benchRuleSpec) error
	Probe(context.Context, benchTransactionSpec) error
	DeleteRule(context.Context, benchRuleSpec) error
	StopCandidate(context.Context, benchTransactionSpec) error
	VerifyCleanup(context.Context, benchTransactionSpec) error
}

func buildBenchTransactionContract() benchTransactionContract {
	return benchTransactionContract{
		Implemented:              true,
		MutationEnabled:          true,
		ControlledSmokeOnly:      true,
		SystemMutatorImplemented: true,
		RuleCount:                6,
		ReverseOrderRollback:     true,
		CleanupMandatory:         true,
		CleanupVerification:      true,
		FailClosed:               true,
		CandidateStartedFirst:    true,
		ProductionConfigMutation: false,
		ProductionRestart:        false,
	}
}

func validateBenchTransactionSpec(spec benchTransactionSpec) error {
	if !benchSessionIDPattern.MatchString(spec.SessionID) {
		return errors.New("invalid bench session id")
	}
	ip := net.ParseIP(spec.DestinationIPv4)
	if ip == nil || ip.To4() == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() ||
		ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
		return errors.New("destination must be an exact public unicast IPv4 address")
	}
	if spec.LocalPort < 1024 || spec.LocalPort > 65535 {
		return errors.New("local port must be in range 1024-65535")
	}
	if spec.Queue < benchQueueMin || spec.Queue > benchQueueMax {
		return fmt.Errorf("bench queue must be in reserved range %d-%d", benchQueueMin, benchQueueMax)
	}
	return nil
}

func benchRuleComment(sessionID, suffix string) string {
	return "routerforge-bench:" + sessionID + ":" + suffix
}

func buildBenchRulePlan(spec benchTransactionSpec) ([]benchRuleSpec, error) {
	if err := validateBenchTransactionSpec(spec); err != nil {
		return nil, err
	}

	port := strconv.Itoa(spec.LocalPort)
	queue := strconv.Itoa(spec.Queue)
	mark := "0x40000000/0x40000000"
	clearMark := "0x0/0x40000000"

	outTuple := []string{"-p", "tcp", "-d", spec.DestinationIPv4, "--sport", port, "--dport", "443"}
	inTuple := []string{"-p", "tcp", "-s", spec.DestinationIPv4, "--sport", "443", "--dport", port}

	rule := func(name, chain, relation, anchor string, priority int, comment string, args []string) benchRuleSpec {
		return benchRuleSpec{
			Name: name, Table: "mangle", Chain: chain, Relation: relation,
			Anchor: anchor, Priority: priority, Comment: comment, RuleArgs: args,
		}
	}

	return []benchRuleSpec{
		rule(
			"out-mark", "POSTROUTING", "before", "nfqws_post", 10,
			benchRuleComment(spec.SessionID, "out-mark"),
			append(append([]string{}, outTuple...), "-m", "comment", "--comment", benchRuleComment(spec.SessionID, "out-mark"), "-j", "MARK", "--set-xmark", mark),
		),
		rule(
			"out-queue", "POSTROUTING", "before", "nfqws_post", 20,
			benchRuleComment(spec.SessionID, "out-queue"),
			append(append([]string{}, outTuple...), "-m", "mark", "--mark", mark, "-m", "comment", "--comment", benchRuleComment(spec.SessionID, "out-queue"), "-j", "NFQUEUE", "--queue-num", queue, "--queue-bypass"),
		),
		rule(
			"out-clear", "POSTROUTING", "after", "nfqws_post", 30,
			benchRuleComment(spec.SessionID, "out-clear"),
			append(append([]string{}, outTuple...), "-m", "mark", "--mark", mark, "-m", "comment", "--comment", benchRuleComment(spec.SessionID, "out-clear"), "-j", "MARK", "--set-xmark", clearMark),
		),
		rule(
			"in-mark", "PREROUTING", "before", "nfqws_pre", 10,
			benchRuleComment(spec.SessionID, "in-mark"),
			append(append([]string{}, inTuple...), "-m", "comment", "--comment", benchRuleComment(spec.SessionID, "in-mark"), "-j", "MARK", "--set-xmark", mark),
		),
		rule(
			"in-queue", "PREROUTING", "before", "nfqws_pre", 20,
			benchRuleComment(spec.SessionID, "in-queue"),
			append(append([]string{}, inTuple...), "-m", "mark", "--mark", mark, "-m", "comment", "--comment", benchRuleComment(spec.SessionID, "in-queue"), "-j", "NFQUEUE", "--queue-num", queue, "--queue-bypass"),
		),
		rule(
			"in-clear", "PREROUTING", "after", "nfqws_pre", 30,
			benchRuleComment(spec.SessionID, "in-clear"),
			append(append([]string{}, inTuple...), "-m", "mark", "--mark", mark, "-m", "comment", "--comment", benchRuleComment(spec.SessionID, "in-clear"), "-j", "MARK", "--set-xmark", clearMark),
		),
	}, nil
}

func runBenchTransaction(ctx context.Context, ops benchTransactionOps, spec benchTransactionSpec) (result benchTransactionResult) {
	result = benchTransactionResult{
		State:         benchLifecycleStateNew,
		AppliedRules:  []string{},
		CleanupErrors: []string{},
	}
	if ctx == nil {
		result.State = benchLifecycleStateFailed
		result.Error = "transaction context is nil"
		return result
	}
	if ops == nil {
		result.State = benchLifecycleStateFailed
		result.Error = "transaction operations are nil"
		return result
	}

	rules, err := buildBenchRulePlan(spec)
	if err != nil {
		result.State = benchLifecycleStateFailed
		result.Error = err.Error()
		return result
	}

	candidateStarted := false
	applied := []benchRuleSpec{}

	defer func() {
		if !candidateStarted && len(applied) == 0 {
			return
		}

		result.CleanupAttempted = true
		result.State = benchLifecycleStateCleaning

		for i := len(applied) - 1; i >= 0; i-- {
			if err := ops.DeleteRule(context.Background(), applied[i]); err != nil {
				result.CleanupErrors = append(result.CleanupErrors, "delete "+applied[i].Name+": "+err.Error())
			}
		}
		if candidateStarted {
			if err := ops.StopCandidate(context.Background(), spec); err != nil {
				result.CleanupErrors = append(result.CleanupErrors, "stop candidate: "+err.Error())
			}
		}
		if err := ops.VerifyCleanup(context.Background(), spec); err != nil {
			result.CleanupErrors = append(result.CleanupErrors, "verify cleanup: "+err.Error())
		}

		result.CleanupProven = len(result.CleanupErrors) == 0
		if result.CleanupProven {
			result.State = benchLifecycleStateClean
		} else {
			result.State = benchLifecycleStateCleanupUnproven
		}
	}()

	if err := ops.StartCandidate(ctx, spec); err != nil {
		result.State = benchLifecycleStateFailed
		result.Error = "start candidate: " + err.Error()
		return result
	}
	candidateStarted = true
	result.State = benchLifecycleStateCandidateStarted

	for _, rule := range rules {
		if err := ops.InstallRule(ctx, rule); err != nil {
			result.State = benchLifecycleStateFailed
			result.Error = "install " + rule.Name + ": " + err.Error()
			return result
		}
		applied = append(applied, rule)
		result.AppliedRules = append(result.AppliedRules, rule.Name)
	}
	result.State = benchLifecycleStateRuleInstalled

	if err := ops.VerifyInstalled(ctx, spec, rules); err != nil {
		result.State = benchLifecycleStateFailed
		result.Error = "verify installed: " + err.Error()
		return result
	}

	result.State = benchLifecycleStateProbeRunning
	if err := ops.Probe(ctx, spec); err != nil {
		result.State = benchLifecycleStateFailed
		result.Error = "probe: " + err.Error()
		return result
	}

	return result
}
