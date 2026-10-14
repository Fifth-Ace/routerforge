package main

import "sort"

const v2ConfirmMaxCandidates = 2

// v2ConfirmationShortlist selects only single-pass working candidates for a
// bounded second live probe. This function never creates or mutates a strategy.
func v2ConfirmationShortlist(candidates []v2CandidateResult, limit int) []int {
	if limit < 1 {
		return nil
	}
	var indices []int
	for i, c := range candidates {
		if c.ResultClass == "WORKING" && c.InfrastructureOK && c.CleanupProven &&
			len(c.Attempts) == 1 && c.Successes == 1 {
			indices = append(indices, i)
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		return v2CandidateBetter(candidates[indices[i]], candidates[indices[j]])
	})
	if len(indices) > limit {
		indices = indices[:limit]
	}
	return indices
}

// A direct selector recommendation needs at least two independent successful
// attempts in the same run. Historical memory does not count as a live attempt.
func v2ChooseConfirmedRecommendation(baseline v2CandidateResult, candidates []v2CandidateResult) (bool, *v2CandidateResult, bool, string) {
	verified := make([]v2CandidateResult, 0, len(candidates))
	for _, c := range candidates {
		if c.ResultClass == "WORKING" && c.InfrastructureOK && c.CleanupProven &&
			len(c.Attempts) >= 2 && c.Successes == len(c.Attempts) {
			verified = append(verified, c)
		}
	}
	ok, best, needed, reason := v2ChooseRecommendation(baseline, verified)
	if !ok && baseline.ResultClass != "WORKING" && len(verified) == 0 {
		return false, nil, false, "no candidate passed two independent live NFQUEUE checks; no Apply recommendation"
	}
	return ok, best, needed, reason
}

// Do not promote one-off successes back into trusted target memory. Failed and
// inconclusive tests can still be recorded as negative observations.
func v2ConfirmedMemoryEvidence(candidates []v2CandidateResult) []v2CandidateResult {
	out := make([]v2CandidateResult, 0, len(candidates))
	for _, c := range candidates {
		if c.ResultClass == "WORKING" && len(c.Attempts) < 2 {
			continue
		}
		out = append(out, c)
	}
	return out
}
