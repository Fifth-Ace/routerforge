package main

import "sort"

// v2BuildPriorityCandidatePool orders existing strategy sources for live testing.
// This is not a planner or a substitute for the isolated NFQUEUE test: evidence
// determines only the order in which the direct selector executes candidates.
func v2BuildPriorityCandidatePool(
	manual []v2SelectorCandidateInput,
	memory []v2CandidatePoolItem,
	saved []v2StoredStrategy,
	corpus []v2StaticCorpusEntry,
	corpusLimit int,
) ([]v2SelectorCandidateInput, v2SelectorAutoPoolMeta) {
	meta := v2SelectorAutoPoolMeta{Enabled: true, Sources: []string{}, Warnings: []string{}}
	result := make([]v2SelectorCandidateInput, 0, len(manual)+len(memory)+len(saved)+len(corpus))
	seen := make(map[string]bool)
	sourceSet := make(map[string]bool)

	add := func(candidate v2SelectorCandidateInput, origin string) bool {
		if len(result) >= v2DirectSelectorMaxCandidates {
			return false
		}
		fingerprint := v2CandidateTechniqueFingerprint(candidate.Args)
		if fingerprint == "" || seen[fingerprint] {
			return false
		}
		seen[fingerprint] = true
		candidate.Args = append([]string(nil), candidate.Args...)
		result = append(result, candidate)
		if candidate.Source != "" {
			sourceSet[candidate.Source] = true
		}
		switch origin {
		case "memory":
			meta.MemoryCandidates++
			meta.Added++
		case "saved":
			meta.LibraryCandidates++
			meta.Added++
		case "community":
			meta.CommunityCandidates++
			meta.Added++
		case "corpus":
			meta.BuiltinCandidates++
			meta.Added++
		}
		return true
	}

	// A user-chosen candidate must not get buried behind hundreds of corpus
	// entries, especially in fast mode. Keep the caller's explicit order.
	for _, item := range manual {
		add(item, "manual")
	}
	// Memory is already ranked and filtered by target, current config,
	// environment, freshness, and independently verified live evidence.
	for _, item := range memory {
		if item.MemoryClass != "WORKING" || item.Fingerprint == "" || item.Fingerprint != v2CandidateTechniqueFingerprint(item.Args) {
			continue
		}
		add(v2SelectorCandidateInput{ID: item.ID, Name: item.Name, Source: "memory", Args: item.Args}, "memory")
	}
	for _, item := range saved {
		origin := "saved"
		if item.Source == "community" {
			origin = "community"
		}
		add(v2SelectorCandidateInput{ID: item.ID, Name: item.Name, Source: item.Source, Args: item.Args}, origin)
	}
	corpusAdded := 0
	for _, item := range corpus {
		if corpusAdded >= corpusLimit || len(result) >= v2DirectSelectorMaxCandidates {
			break
		}
		if add(v2SelectorCandidateInput{ID: item.ID, Name: item.Name, Source: item.Source, Args: item.Args}, "corpus") {
			corpusAdded++
		}
	}

	for source := range sourceSet {
		meta.Sources = append(meta.Sources, source)
	}
	sort.Strings(meta.Sources)
	meta.Warnings = append(meta.Warnings,
		"NF3A live test order: explicit, current-site memory, saved library, portable corpus; historical results never replace live testing")
	return result, meta
}
