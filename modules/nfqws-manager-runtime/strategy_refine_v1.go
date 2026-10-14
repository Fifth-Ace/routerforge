package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// NF3B3: a deliberately small, independently implemented subset of the
// parameter-axis refinement in zemidala/nfqws2-ui (MIT), src/www/api.php,
// testRefine() at upstream commit ea5304505c85e4da003a1f6b703f014a572d81c3.
// Only existing fake Lua spellings proven in RouterForge's TLS corpus are used.
// SNI rewriting and ClientHello cloning are NOT attempted without device proof.
const (
	v2RefineMaxVariants = 4
	v2RefineTimeBudget  = 300 * time.Second
	v2RefineAttemptTime = 35 * time.Second
)

type v2RefinePlan struct {
	ParentID         string
	CandidateID      string
	CandidateName    string
	ChangedParameter string
	Profile          benchStrategyProfile
}

type v2RefineAxis struct {
	Kind  string
	Value string
}

// One-axis-at-a-time experiments, following upstream's order: fake corruption,
// then fake repeat count. Neither cross-products nor arbitrary Lua mutation.
var v2RefineAxes = []v2RefineAxis{
	{Kind: "fool", Value: "badsum"},
	{Kind: "fool", Value: "tcp_md5"},
	{Kind: "repeats", Value: "2"},
	{Kind: "repeats", Value: "4"},
}

// These keys are mutually exclusive alternatives on the selected fake step,
// mirroring upstream's withFool() behavior. Other parameters are preserved.
func v2RefineFoolKey(key string) bool {
	switch key {
	case "tcp_ts", "tcp_md5", "badsum", "tcp_seq", "tcp_ack", "ip_ttl", "ip_autottl":
		return true
	default:
		return false
	}
}

// Returns no variant unless the step is a real fake with a blob and well-formed
// colon-separated parameters. Never edits another Lua function or a selector.
func v2RefineFakeArg(raw string, axis v2RefineAxis) (string, bool) {
	const prefix = "--lua-desync=fake:"
	if !strings.HasPrefix(raw, prefix) {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(raw, prefix), ":")
	if len(parts) == 0 || len(parts) > 32 {
		return "", false
	}
	seen := make(map[string]bool, len(parts))
	blob := false
	for _, part := range parts {
		if part == "" {
			return "", false
		}
		key := strings.SplitN(part, "=", 2)[0]
		if seen[key] {
			return "", false
		}
		seen[key] = true
		if strings.HasPrefix(part, "blob=") && len(part) > len("blob=") {
			blob = true
		}
	}
	if !blob {
		return "", false
	}
	kept := make([]string, 0, len(parts)+1)
	for _, part := range parts {
		key := strings.SplitN(part, "=", 2)[0]
		switch axis.Kind {
		case "fool":
			if v2RefineFoolKey(key) {
				continue
			}
		case "repeats":
			if key == "repeats" {
				continue
			}
		default:
			return "", false
		}
		kept = append(kept, part)
	}
	switch axis.Kind {
	case "fool":
		if axis.Value != "badsum" && axis.Value != "tcp_md5" {
			return "", false
		}
		kept = append(kept, axis.Value)
	case "repeats":
		if axis.Value != "2" && axis.Value != "4" {
			return "", false
		}
		kept = append(kept, "repeats="+axis.Value)
	}
	updated := prefix + strings.Join(kept, ":")
	return updated, updated != raw
}

// A failed probe with no observed TLS/HTTP progress is not an excuse to invent
// candidates. A PARTIAL or UNSTABLE result must also carry real live evidence.
func v2RefineProgressScore(c v2CandidateResult) int64 {
	if !c.CleanupProven || !c.InfrastructureOK || len(c.Attempts) == 0 {
		return 0
	}
	switch c.ResultClass {
	case "PARTIAL", "UNSTABLE", "FAILED":
	default:
		return 0
	}
	var score int64
	for _, a := range c.Attempts {
		if !a.InfrastructureOK || !a.CleanupProven || !a.StrategyPathExercised {
			return 0
		}
		if a.OK {
			score += 100000000
		}
		if a.Metrics.Cutoff16KSuspected {
			score += 10000000
		}
		if a.Metrics.TLSComplete {
			score += 1000000
		}
		if a.Metrics.HTTPStatus > 0 {
			score += 100000
		}
		if a.Metrics.Bytes > 0 {
			bytes := a.Metrics.Bytes
			if bytes > 65535 {
				bytes = 65535
			}
			score += bytes
		}
	}
	return score
}

func v2RefinePlans(candidates []v2CandidateResult, target string) []v2RefinePlan {
	limit := v2DirectSelectorMaxCandidates - len(candidates)
	if limit <= 0 {
		return nil
	}
	if limit > v2RefineMaxVariants {
		limit = v2RefineMaxVariants
	}
	// NF3B2 is authoritative: a twice-confirmed winner needs no refinement.
	for _, c := range candidates {
		if c.ResultClass == "WORKING" && c.InfrastructureOK && c.CleanupProven &&
			len(c.Attempts) >= 2 && c.Successes == len(c.Attempts) {
			return nil
		}
	}
	seen := make(map[string]bool, len(candidates)+limit)
	for _, c := range candidates {
		if fp := v2CandidateTechniqueFingerprint(c.Args); fp != "" {
			seen[fp] = true
		}
	}
	// Upstream ranks by successful attempts and progress. Keep one best fake
	// parent, not the first partial hit in the candidate queue.
	parentIndex := -1
	var bestScore int64
	var fakeIndex int
	for i, c := range candidates {
		score := v2RefineProgressScore(c)
		if score == 0 || score <= bestScore {
			continue
		}
		for j, arg := range c.Args {
			// At least one known axis must yield a valid, distinct fake.
			// This also accepts a parent that already uses repeats=2.
			valid := false
			for _, axis := range v2RefineAxes {
				if _, ok := v2RefineFakeArg(arg, axis); ok {
					valid = true
					break
				}
			}
			if valid {
				parentIndex, fakeIndex, bestScore = i, j, score
				break
			}
		}
	}
	if parentIndex < 0 {
		return nil
	}
	parent := candidates[parentIndex]
	plans := make([]v2RefinePlan, 0, limit)
	for _, axis := range v2RefineAxes {
		changed, ok := v2RefineFakeArg(parent.Args[fakeIndex], axis)
		if !ok {
			continue
		}
		args := append([]string(nil), parent.Args...)
		args[fakeIndex] = changed
		profile, err := v2CustomProfile(args, target)
		if err != nil {
			continue // Existing installed-profile eligibility remains authoritative.
		}
		fp := v2CandidateTechniqueFingerprint(profile.Args)
		if fp == "" || seen[fp] {
			continue
		}
		seen[fp] = true
		profile.Index = -1
		plans = append(plans, v2RefinePlan{
			ParentID:         parent.CandidateID,
			CandidateID:      fmt.Sprintf("refine-%d-%d", parentIndex+1, len(plans)+1),
			CandidateName:    "Refined " + parent.CandidateName + " (fake " + axis.Kind + "=" + axis.Value + ")",
			ChangedParameter: "fake." + axis.Kind + "=" + axis.Value,
			Profile:          profile,
		})
		if len(plans) >= limit {
			break
		}
	}
	return plans
}

type v2RefineAttemptRunner func(context.Context, benchStrategyProfile) (v2BenchAttempt, error)

// NF3B2's two-independent-live-success rule applies unmodified to refinements.
func v2RunRefinePlans(ctx context.Context, plans []v2RefinePlan, runner v2RefineAttemptRunner) ([]v2CandidateResult, error) {
	if len(plans) > v2RefineMaxVariants {
		return nil, errors.New("refinement cardinality exceeded")
	}
	results := make([]v2CandidateResult, 0, len(plans))
	for _, plan := range plans {
		if err := ctx.Err(); err != nil {
			return results, fmt.Errorf("refinement stopped: %w", err)
		}
		c := v2CandidateResult{
			CandidateID: plan.CandidateID, CandidateName: plan.CandidateName,
			CandidateSource: "refine", SourceProfileIndex: -1,
			DerivedFrom: plan.ParentID, ChangedParameter: plan.ChangedParameter,
			Args:         append([]string(nil), plan.Profile.Args...),
			StrategyTags: append([]int(nil), plan.Profile.StrategyTags...),
		}
		for attemptNo := 0; attemptNo < 2; attemptNo++ {
			if err := ctx.Err(); err != nil {
				v2FinalizeCandidate(&c)
				c.LiveResult = c.ResultClass
				results = append(results, c)
				return results, fmt.Errorf("refinement stopped: %w", err)
			}
			attemptCtx, cancel := context.WithTimeout(ctx, v2RefineAttemptTime)
			a, err := runner(attemptCtx, plan.Profile)
			cancel()
			c.Attempts = append(c.Attempts, a)
			v2FinalizeCandidate(&c)
			c.LiveResult = c.ResultClass
			if err != nil || !a.CleanupProven || !a.InfrastructureOK {
				results = append(results, c)
				if err != nil {
					return results, err
				}
				return results, errors.New("refinement worker infrastructure/cleanup failed; stopped fail-closed")
			}
			if !a.OK {
				break
			}
		}
		results = append(results, c)
	}
	return results, nil
}

// No new executor: isolated worker, NFQUEUE, memory and exact production SHA.
func v2DirectRefineRunner(target, ip, configSHA string, inventory benchStrategyInventory) v2RefineAttemptRunner {
	return func(ctx context.Context, profile benchStrategyProfile) (v2BenchAttempt, error) {
		if err := ctx.Err(); err != nil {
			return v2BenchAttempt{}, err
		}
		if !strings.EqualFold(readStatus().ConfigSHA256, configSHA) {
			return v2BenchAttempt{}, errors.New("production config changed before refinement")
		}
		if v2ReadSelectorWorkerBudget().Limit < 1 {
			return v2BenchAttempt{}, errors.New("refinement memory budget exhausted")
		}
		cap := readBenchCapabilities()
		if !cap.CleanupBaselineProven || !benchExecutionReady(cap) {
			return v2BenchAttempt{}, errors.New("refinement capability or baseline cleanup gate failed")
		}
		queues := v2FreeBenchQueues(cap.OccupiedQueues, 1)
		if len(queues) != 1 {
			return v2BenchAttempt{}, errors.New("no free NFQUEUE for refinement")
		}
		sandbox := newV2DirectSandbox(cap, inventory, 0, queues[0], target, ip)
		if err := sandbox.RulesUp(true); err != nil {
			sandbox.RulesDown()
			return v2BenchAttempt{}, fmt.Errorf("refinement rules up: %w", err)
		}
		a := sandbox.RunCandidate(ctx, profile)
		sandbox.RulesDown()
		if !readBenchCapabilities().CleanupBaselineProven {
			a.CleanupProven = false
			return a, errors.New("refinement NFQUEUE rules cleanup is not proven")
		}
		if !strings.EqualFold(readStatus().ConfigSHA256, configSHA) {
			return a, errors.New("production config changed during refinement")
		}
		if err := ctx.Err(); err != nil {
			return a, fmt.Errorf("refinement attempt timed out or cancelled: %w", err)
		}
		return a, nil
	}
}
