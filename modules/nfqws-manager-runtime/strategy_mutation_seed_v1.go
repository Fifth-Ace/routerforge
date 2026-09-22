package main

import (
	"fmt"
	"sort"
	"strings"
)

const v2MutationSeedSelectorVersion = 1

type v2MutationSeed struct {
	Rank        int                 `json:"rank"`
	Outcome     string              `json:"outcome"`
	Reason      string              `json:"reason"`
	Fingerprint string              `json:"fingerprint"`
	Seed        v2CandidatePoolItem `json:"seed"`
	Result      v2CandidateResult   `json:"result"`
}

type v2MutationSeedPlan struct {
	Version    int              `json:"version"`
	Mode       string           `json:"mode"`
	Budget     int              `json:"budget"`
	Eligible   int              `json:"eligible"`
	Selected   int              `json:"selected"`
	Rejected   int              `json:"rejected"`
	Duplicates int              `json:"duplicates"`
	Seeds      []v2MutationSeed `json:"seeds"`
}

func v2MutationSeedBudget(mode benchAutoTuneMode) int {
	switch mode.Name {
	case "fast":
		return 1
	case "thorough":
		return 4
	default:
		return 2
	}
}

func v2MutationOutcomePriority(outcome string) int {
	switch outcome {
	case v2MutationOutcomeVerified:
		return 500
	case v2MutationOutcomePromising:
		return 400
	case v2MutationOutcomePartial:
		return 350
	case v2MutationOutcomeUnstable:
		return 300
	default:
		return 0
	}
}

func v2MutationSeedReason(result v2CandidateResult, outcome string) string {
	switch outcome {
	case v2MutationOutcomeVerified:
		return "verified live candidate; explore nearby variants for robustness or a simpler equivalent"
	case v2MutationOutcomePromising:
		return "candidate produced useful live progress; explore neighboring technique values"
	case v2MutationOutcomePartial:
		return "candidate produced partial transport progress; mutate around the partial breakthrough"
	case v2MutationOutcomeUnstable:
		return "candidate worked inconsistently; explore nearby values for a stable variant"
	default:
		return "candidate is not eligible for adaptive mutation"
	}
}

func v2MutationSeedScore(result v2CandidateResult) int {
	outcome := v2MutationOutcome(result)
	score := v2MutationOutcomePriority(outcome)
	if score == 0 {
		return 0
	}
	score += int(result.SuccessRate * 100)
	score += int(result.CompleteRate * 50)
	if result.Successes > 0 {
		bonus := result.Successes * 5
		if bonus > 25 {
			bonus = 25
		}
		score += bonus
	}
	if result.MedianTTFBMS > 0 {
		switch {
		case result.MedianTTFBMS <= 250:
			score += 20
		case result.MedianTTFBMS <= 1000:
			score += 10
		case result.MedianTTFBMS >= 5000:
			score -= 10
		}
	}
	if result.MedianThroughput > 0 {
		switch {
		case result.MedianThroughput >= 1024*1024:
			score += 20
		case result.MedianThroughput >= 128*1024:
			score += 10
		}
	}
	return score
}

func v2MutationSeedFromResult(result v2CandidateResult, transport benchTransportProfile) (v2CandidatePoolItem, string, bool) {
	if result.Baseline || len(result.Args) == 0 || !v2MutationSeedEligible(result) {
		return v2CandidatePoolItem{}, "", false
	}
	fp := v2CandidateTechniqueFingerprint(result.Args)
	if fp == "" {
		return v2CandidatePoolItem{}, "", false
	}
	id := strings.TrimSpace(result.CandidateID)
	if id == "" {
		id = "seed-" + fp[:16]
	}
	name := strings.TrimSpace(result.CandidateName)
	if name == "" {
		name = "Adaptive seed " + fp[:8]
	}
	source := strings.TrimSpace(result.CandidateSource)
	if source == "" {
		source = "custom"
	}
	item := v2CandidatePoolItem{
		ID:          id,
		Name:        name,
		Source:      source,
		Family:      v2MutationFamily(result.Args, ""),
		Protocol:    transport.ID,
		Args:        append([]string{}, result.Args...),
		Fingerprint: fp,
		Stage:       v2ProgressiveStageFull,
	}
	return item, fp, true
}

func v2MutationSelectSeeds(mode benchAutoTuneMode, transport benchTransportProfile, results []v2CandidateResult) v2MutationSeedPlan {
	plan := v2MutationSeedPlan{
		Version: v2MutationSeedSelectorVersion,
		Mode:    mode.Name,
		Budget:  v2MutationSeedBudget(mode),
		Seeds:   []v2MutationSeed{},
	}
	type ranked struct {
		seed  v2MutationSeed
		score int
		order int
	}
	unique := map[string]ranked{}
	for i, result := range results {
		item, fp, ok := v2MutationSeedFromResult(result, transport)
		if !ok {
			plan.Rejected++
			continue
		}
		outcome := v2MutationOutcome(result)
		entry := ranked{
			seed: v2MutationSeed{
				Outcome:     outcome,
				Reason:      v2MutationSeedReason(result, outcome),
				Fingerprint: fp,
				Seed:        item,
				Result:      result,
			},
			score: v2MutationSeedScore(result),
			order: i,
		}
		if existing, found := unique[fp]; found {
			plan.Duplicates++
			if entry.score > existing.score ||
				(entry.score == existing.score && v2CandidateBetter(entry.seed.Result, existing.seed.Result)) {
				unique[fp] = entry
			}
			continue
		}
		unique[fp] = entry
	}
	rankedSeeds := make([]ranked, 0, len(unique))
	for _, entry := range unique {
		rankedSeeds = append(rankedSeeds, entry)
	}
	plan.Eligible = len(rankedSeeds)
	sort.SliceStable(rankedSeeds, func(i, j int) bool {
		if rankedSeeds[i].score != rankedSeeds[j].score {
			return rankedSeeds[i].score > rankedSeeds[j].score
		}
		if v2CandidateBetter(rankedSeeds[i].seed.Result, rankedSeeds[j].seed.Result) {
			return true
		}
		if v2CandidateBetter(rankedSeeds[j].seed.Result, rankedSeeds[i].seed.Result) {
			return false
		}
		if rankedSeeds[i].seed.Fingerprint != rankedSeeds[j].seed.Fingerprint {
			return rankedSeeds[i].seed.Fingerprint < rankedSeeds[j].seed.Fingerprint
		}
		return rankedSeeds[i].order < rankedSeeds[j].order
	})
	limit := plan.Budget
	if limit > len(rankedSeeds) {
		limit = len(rankedSeeds)
	}
	for i := 0; i < limit; i++ {
		seed := rankedSeeds[i].seed
		seed.Rank = i + 1
		seed.Reason = fmt.Sprintf("%s; adaptive score=%d", seed.Reason, rankedSeeds[i].score)
		plan.Seeds = append(plan.Seeds, seed)
	}
	plan.Selected = len(plan.Seeds)
	return plan
}
