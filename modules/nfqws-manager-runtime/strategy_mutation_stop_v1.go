package main

const v2MutationStopPolicyVersion = 1

const (
	v2MutationStopContinue            = "CONTINUE_ADAPTIVE_SEARCH"
	v2MutationStopBaselineWorking     = "STOP_BASELINE_WORKING"
	v2MutationStopNoBudget            = "STOP_NO_MUTATION_BUDGET"
	v2MutationStopNoEligibleSeeds     = "STOP_NO_ELIGIBLE_SEEDS"
	v2MutationStopFastVerified        = "STOP_FAST_VERIFIED"
	v2MutationStopVerifiedSetComplete = "STOP_VERIFIED_SET_COMPLETE"
	v2MutationStopMultipleVerified    = "STOP_MULTIPLE_VERIFIED"
)

type v2MutationStopDecision struct {
	Version             int                `json:"version"`
	Continue            bool               `json:"continue"`
	Code                string             `json:"code"`
	Reason              string             `json:"reason"`
	Budget              int                `json:"budget"`
	EligibleSeeds       int                `json:"eligible_seeds"`
	SelectedSeeds       int                `json:"selected_seeds"`
	VerifiedSeeds       int                `json:"verified_seeds"`
	NonVerifiedEligible int                `json:"non_verified_eligible"`
	SeedPlan            v2MutationSeedPlan `json:"seed_plan"`
}

func v2MutationVerifiedCandidate(result v2CandidateResult) bool {
	return result.CleanupProven &&
		result.InfrastructureOK &&
		len(result.Attempts) > 0 &&
		result.ResultClass == "WORKING" &&
		result.SuccessRate == 1
}

func v2MutationEvaluateStopPolicy(mode benchAutoTuneMode, transport benchTransportProfile, baseline v2CandidateResult, results []v2CandidateResult) v2MutationStopDecision {
	budget := v2MutationBudget(mode, mode.MaxCandidates)
	seedPlan := v2MutationSelectSeeds(mode, transport, results)
	decision := v2MutationStopDecision{
		Version:       v2MutationStopPolicyVersion,
		Continue:      true,
		Code:          v2MutationStopContinue,
		Reason:        "adaptive mutation may improve or stabilize the first live batch",
		Budget:        budget,
		EligibleSeeds: seedPlan.Eligible,
		SelectedSeeds: seedPlan.Selected,
		SeedPlan:      seedPlan,
	}

	for _, result := range results {
		if !v2MutationSeedEligible(result) {
			continue
		}
		if v2MutationVerifiedCandidate(result) {
			decision.VerifiedSeeds++
		} else {
			decision.NonVerifiedEligible++
		}
	}

	if v2MutationVerifiedCandidate(baseline) {
		decision.Continue = false
		decision.Code = v2MutationStopBaselineWorking
		decision.Reason = "baseline is already fully working; adaptive bypass mutation is unnecessary"
		return decision
	}
	if budget <= 0 {
		decision.Continue = false
		decision.Code = v2MutationStopNoBudget
		decision.Reason = "mutation budget is zero for the current selector mode"
		return decision
	}
	if seedPlan.Selected == 0 {
		decision.Continue = false
		decision.Code = v2MutationStopNoEligibleSeeds
		decision.Reason = "first live batch produced no safe eligible mutation seed"
		return decision
	}

	switch mode.Name {
	case "fast":
		if decision.VerifiedSeeds > 0 {
			decision.Continue = false
			decision.Code = v2MutationStopFastVerified
			decision.Reason = "fast mode already has a verified live candidate; skip the adaptive round"
		}
	case "thorough":
		if decision.VerifiedSeeds >= 2 && decision.NonVerifiedEligible == 0 {
			decision.Continue = false
			decision.Code = v2MutationStopMultipleVerified
			decision.Reason = "thorough mode already has multiple verified candidates and no unresolved eligible seed"
		}
	default:
		if decision.VerifiedSeeds > 0 && decision.NonVerifiedEligible == 0 {
			decision.Continue = false
			decision.Code = v2MutationStopVerifiedSetComplete
			decision.Reason = "normal mode eligible seed set is already fully verified"
		}
	}
	return decision
}
