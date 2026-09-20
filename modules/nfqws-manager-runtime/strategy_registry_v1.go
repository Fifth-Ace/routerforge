package main

import (
	"net/http"
	"sort"
	"strings"
	"time"
)

const v2StrategyRegistryVersion = 1

type v2StrategyRegistryProvenance struct {
	Kind       string `json:"kind"`
	Source     string `json:"source"`
	Repository string `json:"repository,omitempty"`
	Ref        string `json:"ref,omitempty"`
	Note       string `json:"note,omitempty"`
}

type v2StrategyRegistryEvidence struct {
	Targets        int     `json:"targets"`
	VerifiedCount  int     `json:"verified_count"`
	WorkingCount   int     `json:"working_count"`
	UnstableCount  int     `json:"unstable_count"`
	FailureCount   int     `json:"failure_count"`
	SuccessRate    float64 `json:"success_rate,omitempty"`
	Confidence     string  `json:"confidence,omitempty"`
	LastVerified  string  `json:"last_verified,omitempty"`
	ReuseEligible int     `json:"reuse_eligible_targets"`
}

type v2StrategyRegistryEntry struct {
	ID          string                       `json:"id"`
	Name        string                       `json:"name"`
	Aliases     []string                     `json:"aliases,omitempty"`
	Sources     []string                     `json:"sources"`
	Family      string                       `json:"family"`
	Protocol    string                       `json:"protocol"`
	Args        []string                     `json:"args"`
	Fingerprint string                       `json:"fingerprint"`
	Provenance  []v2StrategyRegistryProvenance `json:"provenance"`
	Evidence    v2StrategyRegistryEvidence   `json:"evidence"`
}

type v2StrategyRegistryResponse struct {
	OK        bool                      `json:"ok"`
	Version   int                       `json:"version"`
	ReadOnly  bool                      `json:"read_only"`
	Count     int                       `json:"count"`
	Sources   []string                  `json:"sources"`
	Families  []string                  `json:"families"`
	Protocols []string                  `json:"protocols"`
	Entries   []v2StrategyRegistryEntry `json:"entries"`
}

type v2StrategyRegistryAccumulator struct {
	Entry           v2StrategyRegistryEntry
	Targets         map[string]bool
	WeightedSuccess float64
	SuccessWeight   int
	ConfidenceRank  int
}

func registerStrategyRegistryV1Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/strategy-registry", getOnly(handleV2StrategyRegistry))
}

func v2RegistryAddUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func v2RegistryAddProvenance(values []v2StrategyRegistryProvenance, item v2StrategyRegistryProvenance) []v2StrategyRegistryProvenance {
	for _, existing := range values {
		if existing.Kind == item.Kind && existing.Source == item.Source &&
			existing.Repository == item.Repository && existing.Ref == item.Ref {
			return values
		}
	}
	return append(values, item)
}

func v2RegistryInferProtocolFamily(args []string) (string, string) {
	joined := strings.ToLower(strings.Join(args, " "))
	protocol := "tcp"
	switch {
	case strings.Contains(joined, "--filter-l7=quic") || strings.Contains(joined, "--payload=quic_initial"):
		protocol = "quic"
	case strings.Contains(joined, "--filter-l7=stun") || strings.Contains(joined, "--payload=stun"):
		protocol = "stun"
	case strings.Contains(joined, "--filter-l7=http") || strings.Contains(joined, "--payload=http_req"):
		protocol = "http"
	case strings.Contains(joined, "--filter-l7=tls") || strings.Contains(joined, "--payload=tls_client_hello"):
		protocol = "https"
	case strings.Contains(joined, "--filter-l7=wireguard") || strings.Contains(joined, "wireguard_"):
		protocol = "wireguard"
	case strings.Contains(joined, "--filter-udp="):
		protocol = "udp"
	}

	family := "custom"
	switch {
	case strings.Contains(joined, "fakeddisorder"):
		family = "fake-disorder"
	case strings.Contains(joined, "fakedsplit"):
		family = "fake-split"
	case strings.Contains(joined, "multidisorder"):
		family = "disorder"
	case strings.Contains(joined, "multisplit"):
		family = "split"
	case strings.Contains(joined, "http_hostcase") || strings.Contains(joined, "http_domcase"):
		family = "http-case"
	case strings.Contains(joined, "http_methodeol"):
		family = "http-method"
	case strings.Contains(joined, "--lua-desync=fake"):
		family = "fake"
	}
	return protocol, family
}

func v2RegistryProvenanceForSource(source string) v2StrategyRegistryProvenance {
	source = strings.ToLower(strings.TrimSpace(source))
	switch source {
	case "builtin":
		return v2StrategyRegistryProvenance{
			Kind: "routerforge-native", Source: "builtin",
			Repository: "Fifth-Ace/routerforge",
			Note: "RouterForge built-in candidate family",
		}
	case "memory":
		return v2StrategyRegistryProvenance{
			Kind: "runtime-evidence", Source: "memory",
			Repository: "Fifth-Ace/routerforge",
			Note: "verified RouterForge target-memory evidence",
		}
	case "catalog":
		return v2StrategyRegistryProvenance{
			Kind: "local-library", Source: "catalog",
			Note: "catalog label preserved; exact upstream ref is not stored by the legacy library schema",
		}
	case "zapret":
		return v2StrategyRegistryProvenance{
			Kind: "local-library", Source: "zapret",
			Note: "Zapret import label preserved; exact upstream ref is not stored by the legacy library schema",
		}
	case "import":
		return v2StrategyRegistryProvenance{
			Kind: "local-library", Source: "import",
			Note: "imported candidate; exact original upstream ref is not stored by the legacy library schema",
		}
	default:
		return v2StrategyRegistryProvenance{
			Kind: "local-library", Source: source,
			Note: "local Candidate Library entry",
		}
	}
}

func v2RegistryEnsure(
	acc map[string]*v2StrategyRegistryAccumulator,
	name, source, protocol, family string,
	args []string,
) *v2StrategyRegistryAccumulator {
	portable := v2PortableCandidateArgs(args)
	fingerprint := v2CandidateTechniqueFingerprint(portable)
	if fingerprint == "" {
		return nil
	}
	item := acc[fingerprint]
	if item == nil {
		if protocol == "" || family == "" {
			inferredProtocol, inferredFamily := v2RegistryInferProtocolFamily(portable)
			if protocol == "" {
				protocol = inferredProtocol
			}
			if family == "" {
				family = inferredFamily
			}
		}
		id := "reg-" + fingerprint
		if len(fingerprint) >= 16 {
			id = "reg-" + fingerprint[:16]
		}
		item = &v2StrategyRegistryAccumulator{
			Entry: v2StrategyRegistryEntry{
				ID: id, Name: strings.TrimSpace(name),
				Sources: []string{}, Family: family, Protocol: protocol,
				Args: append([]string{}, portable...), Fingerprint: fingerprint,
				Provenance: []v2StrategyRegistryProvenance{},
			},
			Targets: map[string]bool{},
		}
		acc[fingerprint] = item
	}
	if item.Entry.Name == "" {
		item.Entry.Name = strings.TrimSpace(name)
	} else if strings.TrimSpace(name) != "" && item.Entry.Name != strings.TrimSpace(name) {
		item.Entry.Aliases = v2RegistryAddUnique(item.Entry.Aliases, strings.TrimSpace(name))
	}
	item.Entry.Sources = v2RegistryAddUnique(item.Entry.Sources, strings.ToLower(strings.TrimSpace(source)))
	item.Entry.Provenance = v2RegistryAddProvenance(item.Entry.Provenance, v2RegistryProvenanceForSource(source))
	if item.Entry.Protocol == "" || item.Entry.Family == "" {
		inferredProtocol, inferredFamily := v2RegistryInferProtocolFamily(item.Entry.Args)
		if item.Entry.Protocol == "" {
			item.Entry.Protocol = inferredProtocol
		}
		if item.Entry.Family == "" {
			item.Entry.Family = inferredFamily
		}
	}
	return item
}

func v2BuildStrategyRegistry(
	library v2StrategyLibraryDocument,
	memory v2TargetMemoryDocument,
	now time.Time,
) v2StrategyRegistryResponse {
	acc := map[string]*v2StrategyRegistryAccumulator{}

	builtins := make([]v2BuiltinCandidate, 0,
		len(v2BuiltinHTTPSCandidates)+len(v2BuiltinHTTPCandidates)+
			len(v2BuiltinQUICCandidates)+len(v2BuiltinSTUNCandidates))
	builtins = append(builtins, v2BuiltinHTTPSCandidates...)
	builtins = append(builtins, v2BuiltinHTTPCandidates...)
	builtins = append(builtins, v2BuiltinQUICCandidates...)
	builtins = append(builtins, v2BuiltinSTUNCandidates...)
	for _, builtin := range builtins {
		v2RegistryEnsure(acc, builtin.Name, "builtin", builtin.Protocol, builtin.Family, builtin.Args)
	}

	for _, stored := range library.Strategies {
		v2RegistryEnsure(acc, stored.Name, stored.Source, "", "", stored.Args)
	}

	for _, evidence := range memory.Entries {
		item := v2RegistryEnsure(acc, evidence.CandidateName, "memory", evidence.Protocol, "", evidence.Args)
		if item == nil {
			continue
		}
		if evidence.CandidateSource != "" {
			item.Entry.Sources = v2RegistryAddUnique(item.Entry.Sources, strings.ToLower(evidence.CandidateSource))
			item.Entry.Provenance = v2RegistryAddProvenance(item.Entry.Provenance, v2RegistryProvenanceForSource(evidence.CandidateSource))
		}
		if evidence.Target != "" {
			item.Targets[evidence.Target] = true
		}
		item.Entry.Evidence.VerifiedCount += evidence.VerifiedCount
		switch evidence.ResultClass {
		case "WORKING":
			item.Entry.Evidence.WorkingCount++
		case "UNSTABLE":
			item.Entry.Evidence.UnstableCount++
		default:
			item.Entry.Evidence.FailureCount++
		}
		weight := evidence.VerifiedCount
		if weight <= 0 {
			weight = 1
		}
		item.WeightedSuccess += evidence.SuccessRate * float64(weight)
		item.SuccessWeight += weight
		if evidence.LastVerified > item.Entry.Evidence.LastVerified {
			item.Entry.Evidence.LastVerified = evidence.LastVerified
		}
		decision := v2TargetMemoryReuseDecisionForEntry(evidence, now)
		if decision.ReuseEligible {
			item.Entry.Evidence.ReuseEligible++
		}
		rank := v2MemoryConfidenceRank(decision.Confidence)
		if rank > item.ConfidenceRank {
			item.ConfidenceRank = rank
			item.Entry.Evidence.Confidence = decision.Confidence
		}
	}

	entries := make([]v2StrategyRegistryEntry, 0, len(acc))
	sourceSet, familySet, protocolSet := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, item := range acc {
		item.Entry.Evidence.Targets = len(item.Targets)
		if item.SuccessWeight > 0 {
			item.Entry.Evidence.SuccessRate = item.WeightedSuccess / float64(item.SuccessWeight)
		}
		sort.Strings(item.Entry.Sources)
		sort.Strings(item.Entry.Aliases)
		for _, source := range item.Entry.Sources {
			sourceSet[source] = true
		}
		if item.Entry.Family != "" {
			familySet[item.Entry.Family] = true
		}
		if item.Entry.Protocol != "" {
			protocolSet[item.Entry.Protocol] = true
		}
		entries = append(entries, item.Entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Protocol != entries[j].Protocol {
			return entries[i].Protocol < entries[j].Protocol
		}
		if entries[i].Family != entries[j].Family {
			return entries[i].Family < entries[j].Family
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	toSorted := func(set map[string]bool) []string {
		out := make([]string, 0, len(set))
		for value := range set {
			out = append(out, value)
		}
		sort.Strings(out)
		return out
	}

	return v2StrategyRegistryResponse{
		OK: true, Version: v2StrategyRegistryVersion, ReadOnly: true,
		Count: len(entries), Sources: toSorted(sourceSet),
		Families: toSorted(familySet), Protocols: toSorted(protocolSet),
		Entries: entries,
	}
}

func handleV2StrategyRegistry(w http.ResponseWriter, _ *http.Request) {
	library, err := readV2StrategyLibrary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read strategy library: " + err.Error()})
		return
	}
	memory, err := readV2TargetMemory()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read target memory: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, v2BuildStrategyRegistry(library, memory, v2TargetMemoryNow().UTC()))
}
