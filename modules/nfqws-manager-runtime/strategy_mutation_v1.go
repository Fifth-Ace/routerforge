package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const v2StrategyMutationVersion = 1

const (
	v2MutationOutcomeVerified     = "VERIFIED"
	v2MutationOutcomePromising    = "PROMISING"
	v2MutationOutcomePartial      = "PARTIAL"
	v2MutationOutcomeUnstable     = "UNSTABLE"
	v2MutationOutcomeDead         = "DEAD"
	v2MutationOutcomeInconclusive = "INCONCLUSIVE"
)

type v2MutationMeta struct {
	Version       int      `json:"version"`
	SeedID        string   `json:"seed_id"`
	SeedClass     string   `json:"seed_class"`
	Generated     int      `json:"generated"`
	Compilable    int      `json:"compilable"`
	Admitted      int      `json:"admitted"`
	Axes          []string `json:"axes"`
	Transport     string   `json:"transport"`
	MutationLimit int      `json:"mutation_limit"`
}

type v2MutationBuilder struct {
	target    string
	transport benchTransportProfile
	mode      benchAutoTuneMode
	seed      v2CandidatePoolItem
	limit     int
	items     []v2CandidatePoolItem
	seen      map[string]bool
	axes      map[string]bool
	generated int
}

var (
	v2MutationRepeatsRE = regexp.MustCompile(`repeats=[0-9]+`)
	v2MutationTTLRE     = regexp.MustCompile(`ip_ttl=[0-9]+`)
	v2MutationSeqOvlRE  = regexp.MustCompile(`seqovl=[0-9]+`)
	v2MutationPosRE     = regexp.MustCompile(`pos=[^:]+`)
)

func v2MutationBudget(mode benchAutoTuneMode, remaining int) int {
	if remaining <= 0 {
		return 0
	}
	budget := 4
	switch mode.Name {
	case "fast":
		budget = 2
	case "thorough":
		budget = 8
	}
	if budget > remaining {
		budget = remaining
	}
	return budget
}

func v2MutationOutcome(result v2CandidateResult) string {
	switch {
	case !result.CleanupProven || !result.InfrastructureOK || len(result.Attempts) == 0:
		return v2MutationOutcomeInconclusive
	case result.ResultClass == "WORKING" && result.SuccessRate == 1:
		return v2MutationOutcomeVerified
	case result.ResultClass == "PARTIAL":
		return v2MutationOutcomePartial
	case result.ResultClass == "UNSTABLE" || (result.Successes > 0 && result.SuccessRate < 1):
		return v2MutationOutcomeUnstable
	case result.Successes > 0:
		return v2MutationOutcomePromising
	default:
		return v2MutationOutcomeDead
	}
}

func v2MutationSeedEligible(result v2CandidateResult) bool {
	switch v2MutationOutcome(result) {
	case v2MutationOutcomeVerified, v2MutationOutcomePromising, v2MutationOutcomePartial, v2MutationOutcomeUnstable:
		return true
	default:
		return false
	}
}

func v2MutationFamily(args []string, fallback string) string {
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "fakedsplit"):
		return "fake-split"
	case strings.Contains(joined, "fakeddisorder"):
		return "fake-disorder"
	case strings.Contains(joined, "multidisorder"):
		if strings.Contains(joined, "--lua-desync=fake:") {
			return "fake+disorder"
		}
		return "disorder"
	case strings.Contains(joined, "multisplit"):
		if strings.Contains(joined, "--lua-desync=fake:") {
			return "fake+split"
		}
		return "split"
	case strings.Contains(joined, "udplen"):
		return "udplen"
	case strings.Contains(joined, "http_methodeol"):
		return "http-method"
	case strings.Contains(joined, "--lua-desync=fake:"):
		return "fake"
	default:
		return fallback
	}
}

func v2MutationReplaceArg(args []string, index int, value string) []string {
	out := append([]string{}, args...)
	out[index] = value
	return out
}

func v2MutationAppendToken(arg, token string) string {
	if strings.Contains(arg, ":"+token) || strings.HasSuffix(arg, token) {
		return arg
	}
	return arg + ":" + token
}

func (b *v2MutationBuilder) add(axis, label string, args []string) {
	b.generated++
	if len(b.items) >= b.limit {
		return
	}
	if _, err := v2CustomProfileForTransport(args, b.target, b.transport); err != nil {
		return
	}
	fp := v2CandidateTechniqueFingerprint(args)
	if fp == "" || b.seen[fp] {
		return
	}
	b.seen[fp] = true
	b.axes[axis] = true
	family := v2MutationFamily(args, b.seed.Family)
	b.items = append(b.items, v2CandidatePoolItem{
		ID:       fmt.Sprintf("mut-v%d-%s-%02d", v2StrategyMutationVersion, b.transport.ID, len(b.items)+1),
		Name:     "Mutation · " + label,
		Source:   "mutated",
		Family:   family,
		Protocol: b.transport.ID,
		Args:     append([]string{}, args...),
		Stage:    v2ProgressiveStageFull,
	})
}

func v2MutationFamilySwap(b *v2MutationBuilder) {
	for i, arg := range b.seed.Args {
		switch {
		case strings.Contains(arg, "multisplit"):
			b.add("family", "multisplit → multidisorder", v2MutationReplaceArg(b.seed.Args, i, strings.Replace(arg, "multisplit", "multidisorder", 1)))
		case strings.Contains(arg, "multidisorder"):
			b.add("family", "multidisorder → multisplit", v2MutationReplaceArg(b.seed.Args, i, strings.Replace(arg, "multidisorder", "multisplit", 1)))
		case strings.Contains(arg, "fakedsplit"):
			b.add("family", "fakedsplit → fakeddisorder", v2MutationReplaceArg(b.seed.Args, i, strings.Replace(arg, "fakedsplit", "fakeddisorder", 1)))
		case strings.Contains(arg, "fakeddisorder"):
			b.add("family", "fakeddisorder → fakedsplit", v2MutationReplaceArg(b.seed.Args, i, strings.Replace(arg, "fakeddisorder", "fakedsplit", 1)))
		}
	}
}

func v2MutationPositions(b *v2MutationBuilder) {
	var positions []string
	switch b.transport.ID {
	case benchTransportHTTPS:
		positions = []string{"1,midsld", "1,sniext+1", "1,host+1", "2,midsld-2", "sld+1", "1,sniext+1,host+1,midsld"}
	case benchTransportHTTP:
		positions = []string{"method+2", "host+1", "method+2,host+1"}
	default:
		return
	}
	for i, arg := range b.seed.Args {
		if !strings.Contains(arg, "--lua-desync=") || !strings.Contains(arg, "pos=") {
			continue
		}
		for _, pos := range positions {
			next := v2MutationPosRE.ReplaceAllString(arg, "pos="+pos)
			if next != arg {
				b.add("position", "position "+pos, v2MutationReplaceArg(b.seed.Args, i, next))
			}
		}
	}
}

func v2MutationOverlap(b *v2MutationBuilder) {
	if b.transport.ID != benchTransportHTTPS {
		return
	}
	for i, arg := range b.seed.Args {
		if !strings.Contains(arg, "--lua-desync=") ||
			(!strings.Contains(arg, "multisplit") && !strings.Contains(arg, "multidisorder") &&
				!strings.Contains(arg, "fakedsplit") && !strings.Contains(arg, "fakeddisorder")) {
			continue
		}
		for _, overlap := range []int{1, 336, 681} {
			token := "seqovl=" + strconv.Itoa(overlap)
			next := arg
			if v2MutationSeqOvlRE.MatchString(next) {
				next = v2MutationSeqOvlRE.ReplaceAllString(next, token)
			} else {
				next = v2MutationAppendToken(next, token)
			}
			if !strings.Contains(next, "seqovl_pattern=") {
				next = v2MutationAppendToken(next, "seqovl_pattern=tls_clienthello")
			}
			if next != arg {
				b.add("overlap", "overlap "+strconv.Itoa(overlap), v2MutationReplaceArg(b.seed.Args, i, next))
			}
		}
	}
}

func v2MutationRepeats(b *v2MutationBuilder) {
	for i, arg := range b.seed.Args {
		if !strings.Contains(arg, "--lua-desync=fake:") {
			continue
		}
		for _, repeats := range []int{1, 2, 3, 5, 6, 11} {
			token := "repeats=" + strconv.Itoa(repeats)
			next := arg
			if v2MutationRepeatsRE.MatchString(next) {
				next = v2MutationRepeatsRE.ReplaceAllString(next, token)
			} else {
				next = v2MutationAppendToken(next, token)
			}
			if next != arg {
				b.add("repeats", "fake repeats "+strconv.Itoa(repeats), v2MutationReplaceArg(b.seed.Args, i, next))
			}
		}
	}
}

func v2MutationTTL(b *v2MutationBuilder) {
	for i, arg := range b.seed.Args {
		if !strings.Contains(arg, "--lua-desync=fake:") {
			continue
		}
		for _, ttl := range []int{3, 4, 5, 8, 12} {
			token := "ip_ttl=" + strconv.Itoa(ttl)
			next := arg
			if v2MutationTTLRE.MatchString(next) {
				next = v2MutationTTLRE.ReplaceAllString(next, token)
			} else {
				next = v2MutationAppendToken(next, token)
			}
			if next != arg {
				b.add("ttl", "fake TTL "+strconv.Itoa(ttl), v2MutationReplaceArg(b.seed.Args, i, next))
			}
		}
	}
}

func v2MutationFooling(b *v2MutationBuilder) {
	if b.transport.ID != benchTransportHTTPS {
		return
	}
	for i, arg := range b.seed.Args {
		if !strings.Contains(arg, "--lua-desync=fake:") {
			continue
		}
		for _, fooling := range []string{"badsum", "tcp_md5", "tcp_ts_up"} {
			if strings.Contains(arg, fooling) {
				continue
			}
			b.add("fooling", "fake "+fooling, v2MutationReplaceArg(b.seed.Args, i, v2MutationAppendToken(arg, fooling)))
		}
	}
}

func v2MutateCandidateNeighbors(target string, transport benchTransportProfile, mode benchAutoTuneMode, seed v2CandidatePoolItem, limit int) ([]v2CandidatePoolItem, v2MutationMeta) {
	meta := v2MutationMeta{
		Version: v2StrategyMutationVersion, SeedID: seed.ID, Transport: transport.ID,
		MutationLimit: limit, Axes: []string{},
	}
	if limit <= 0 || len(seed.Args) == 0 {
		return []v2CandidatePoolItem{}, meta
	}
	if _, err := v2CustomProfileForTransport(seed.Args, target, transport); err != nil {
		return []v2CandidatePoolItem{}, meta
	}
	b := &v2MutationBuilder{
		target: target, transport: transport, mode: mode, seed: seed, limit: limit,
		items: []v2CandidatePoolItem{}, seen: map[string]bool{}, axes: map[string]bool{},
	}
	b.seen[v2CandidateTechniqueFingerprint(seed.Args)] = true

	v2MutationFamilySwap(b)
	v2MutationPositions(b)
	v2MutationOverlap(b)
	v2MutationRepeats(b)
	v2MutationTTL(b)
	v2MutationFooling(b)

	quickLimit := v2ProgressiveQuickLimit(mode)
	for i := range b.items {
		b.items[i].Stage = v2ProgressiveStageFull
		if i < quickLimit {
			b.items[i].Stage = v2ProgressiveStageQuick
		}
	}
	for _, axis := range []string{"family", "position", "overlap", "repeats", "ttl", "fooling"} {
		if b.axes[axis] {
			meta.Axes = append(meta.Axes, axis)
		}
	}
	meta.Generated = b.generated
	meta.Compilable = len(b.items)
	meta.Admitted = len(b.items)
	return append([]v2CandidatePoolItem{}, b.items...), meta
}
