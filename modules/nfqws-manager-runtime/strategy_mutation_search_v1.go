package main

import (
	"context"
	"fmt"
	"sync"
)

const v2MutationSearchVersion = 1

type v2MutationRoundTrace struct {
	Version    int                `json:"version"`
	Budget     int                `json:"budget"`
	Generated  int                `json:"generated"`
	Admitted   int                `json:"admitted"`
	Duplicates int                `json:"duplicates"`
	Executed   bool               `json:"executed"`
	Completed  int                `json:"completed"`
	SeedPlan   v2MutationSeedPlan `json:"seed_plan"`
}

type v2MutationRoundPlan struct {
	Trace      v2MutationRoundTrace
	Candidates []v2CandidatePoolItem
}

func v2BuildMutationRound(target string, transport benchTransportProfile, mode benchAutoTuneMode, results []v2CandidateResult) v2MutationRoundPlan {
	seedPlan := v2MutationSelectSeeds(mode, transport, results)
	totalBudget := v2MutationBudget(mode, mode.MaxCandidates)
	plan := v2MutationRoundPlan{
		Trace: v2MutationRoundTrace{
			Version:  v2MutationSearchVersion,
			Budget:   totalBudget,
			SeedPlan: seedPlan,
		},
		Candidates: []v2CandidatePoolItem{},
	}
	if totalBudget <= 0 || seedPlan.Selected == 0 {
		return plan
	}

	seen := map[string]bool{}
	for _, result := range results {
		fp := v2CandidateTechniqueFingerprint(result.Args)
		if fp != "" {
			seen[fp] = true
		}
	}

	type seedNeighborhood struct {
		seedIndex int
		items     []v2CandidatePoolItem
		cursor    int
	}
	neighborhoods := make([]seedNeighborhood, 0, len(seedPlan.Seeds))
	for seedIndex, seed := range seedPlan.Seeds {
		neighbors, meta := v2MutateCandidateNeighbors(target, transport, mode, seed.Seed, totalBudget)
		plan.Trace.Generated += meta.Generated
		neighborhoods = append(neighborhoods, seedNeighborhood{
			seedIndex: seedIndex,
			items:     neighbors,
		})
	}

	remaining := totalBudget
	admit := func(neighborhood *seedNeighborhood, quota int) int {
		admitted := 0
		for neighborhood.cursor < len(neighborhood.items) && remaining > 0 && admitted < quota {
			neighborIndex := neighborhood.cursor
			item := neighborhood.items[neighborIndex]
			neighborhood.cursor++
			fp := v2CandidateTechniqueFingerprint(item.Args)
			if fp == "" || seen[fp] {
				plan.Trace.Duplicates++
				continue
			}
			seen[fp] = true
			item.ID = fmt.Sprintf(
				"mut-v%d-%s-s%02d-%02d",
				v2StrategyMutationVersion, transport.ID, neighborhood.seedIndex+1, neighborIndex+1,
			)
			label := item.Name
			if len(label) >= len("Mutation · ") && label[:len("Mutation · ")] == "Mutation · " {
				label = label[len("Mutation · "):]
			}
			item.Name = fmt.Sprintf("Mutation · seed %d · %s", neighborhood.seedIndex+1, label)
			item.Source = "mutated"
			item.Protocol = transport.ID
			item.Fingerprint = fp
			plan.Candidates = append(plan.Candidates, item)
			plan.Trace.Admitted++
			remaining--
			admitted++
		}
		return admitted
	}

	for seedIndex := range neighborhoods {
		if remaining <= 0 {
			break
		}
		seedsLeft := len(neighborhoods) - seedIndex
		share := (remaining + seedsLeft - 1) / seedsLeft
		admit(&neighborhoods[seedIndex], share)
	}

	for remaining > 0 {
		progress := false
		for seedIndex := range neighborhoods {
			if remaining <= 0 {
				break
			}
			if admit(&neighborhoods[seedIndex], 1) > 0 {
				progress = true
			}
		}
		if !progress {
			break
		}
	}

	return plan
}

type v2MutationRuntimeCandidate struct {
	Item    v2CandidatePoolItem
	Profile benchStrategyProfile
}

func v2CompileMutationRound(target string, transport benchTransportProfile, items []v2CandidatePoolItem) ([]v2MutationRuntimeCandidate, error) {
	out := make([]v2MutationRuntimeCandidate, 0, len(items))
	for _, item := range items {
		profile, err := v2CustomProfileForTransport(item.Args, target, transport)
		if err != nil {
			return nil, fmt.Errorf("compile mutation %s: %w", item.ID, err)
		}
		profile.Index = -1
		out = append(out, v2MutationRuntimeCandidate{Item: item, Profile: profile})
	}
	return out, nil
}

func v2RunMutationRound(
	ctx context.Context,
	capabilities benchCapabilities,
	configSHA, target, ip string,
	inventory benchStrategyInventory,
	transport benchTransportProfile,
	mode benchAutoTuneMode,
	items []v2CandidatePoolItem,
	queues []int,
) ([]v2CandidateResult, error) {
	compiled, err := v2CompileMutationRound(target, transport, items)
	if err != nil {
		return nil, err
	}
	if len(compiled) == 0 || len(queues) == 0 {
		return []v2CandidateResult{}, nil
	}

	type job struct {
		index int
		item  v2MutationRuntimeCandidate
	}
	type jobResult struct {
		index  int
		result v2CandidateResult
	}

	workers := len(queues)
	if workers > len(compiled) {
		workers = len(compiled)
	}
	jobs := make(chan job)
	results := make(chan jobResult, len(compiled))
	var wg sync.WaitGroup

	for worker := 0; worker < workers; worker++ {
		queue := queues[worker]
		wg.Add(1)
		go func(q int) {
			defer wg.Done()
			for current := range jobs {
				candidate := v2CandidateResult{
					CandidateID:        current.item.Item.ID,
					CandidateName:      current.item.Item.Name,
					CandidateSource:    "mutated",
					SourceProfileIndex: -1,
					StrategyTags:       append([]int{}, current.item.Profile.StrategyTags...),
					Args:               append([]string{}, current.item.Profile.Args...),
					CleanupProven:      true,
				}
				for attemptIndex := 0; attemptIndex < mode.Attempts; attemptIndex++ {
					attempt := v2RunTransportAttempt(
						ctx, capabilities, configSHA, target, ip,
						inventory, &current.item.Profile, q, transport,
					)
					candidate.Attempts = append(candidate.Attempts, attempt)
					if !attempt.CleanupProven || !attempt.InfrastructureOK {
						break
					}
				}
				v2FinalizeCandidate(&candidate)
				results <- jobResult{index: current.index, result: candidate}
			}
		}(queue)
	}

	go func() {
		for i, item := range compiled {
			jobs <- job{index: i, item: item}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	out := make([]v2CandidateResult, len(compiled))
	for result := range results {
		out[result.index] = result.result
	}
	return out, nil
}
