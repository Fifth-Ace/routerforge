package main

import (
	"sort"
	"strings"
)

type v2CandidatePoolItem struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Source              string   `json:"source"`
	Family              string   `json:"family"`
	Protocol            string   `json:"protocol"`
	Args                []string `json:"args"`
	Fingerprint         string   `json:"fingerprint"`
	MemoryClass         string   `json:"memory_class,omitempty"`
	MemoryConfidence    string   `json:"memory_confidence,omitempty"`
	MemoryAgeSeconds    int64    `json:"memory_age_seconds,omitempty"`
	MemoryVerifiedCount int      `json:"memory_verified_count,omitempty"`
	MemorySuccessStreak int      `json:"memory_success_streak,omitempty"`
	Stage               string   `json:"stage,omitempty"`
	HistoricalRank      int      `json:"historical_rank,omitempty"`
	HistoricalScore     int      `json:"historical_score,omitempty"`
	HistoricalInsight   string   `json:"historical_insight,omitempty"`
	HistoricalPrimary   bool     `json:"historical_primary,omitempty"`
	HistoricalPromoted  bool     `json:"historical_promoted,omitempty"`
	HistoricalReason    string   `json:"historical_reason,omitempty"`

	PlannerOriginalOrder  int    `json:"planner_original_order,omitempty"`
	PlannerEffectiveOrder int    `json:"planner_effective_order,omitempty"`
	PlannerOriginalStage  string `json:"planner_original_stage,omitempty"`
	PlannerEffectiveStage string `json:"planner_effective_stage,omitempty"`
	PlannerScore          int    `json:"planner_score,omitempty"`
	PlannerReason         string `json:"planner_reason,omitempty"`
	PlannerHistoricalHint bool   `json:"planner_historical_hint,omitempty"`
	PlannerDiagnosticHint bool   `json:"planner_diagnostic_hint,omitempty"`
	PlannerAlreadyInPool  bool   `json:"planner_already_in_pool,omitempty"`
}

type v2CandidatePoolResponse struct {
	OK                         bool                  `json:"ok"`
	Target                     string                `json:"target"`
	Mode                       string                `json:"mode"`
	Transport                  string                `json:"transport"`
	Network                    string                `json:"network"`
	RemotePort                 int                   `json:"remote_port"`
	MetricScope                string                `json:"metric_scope"`
	ConfigSHA                  string                `json:"config_sha256"`
	Candidates                 []v2CandidatePoolItem `json:"candidates"`
	Count                      int                   `json:"count"`
	Sources                    []string              `json:"sources"`
	Warnings                   []string              `json:"warnings"`
	RecommendationAware        bool                  `json:"recommendation_aware"`
	RecommendationHints        int                   `json:"recommendation_hints"`
	RecommendationAdded        int                   `json:"recommendation_added"`
	HistoricalExistingPromoted int                   `json:"historical_existing_promoted"`
	SynthesisVersion           int                   `json:"synthesis_version"`
	SynthesisGenerated         int                   `json:"synthesis_generated"`
	SynthesisAdmitted          int                   `json:"synthesis_admitted"`
	SynthesisFamilies          []string              `json:"synthesis_families,omitempty"`
	SynthesisBasis             string                `json:"synthesis_basis,omitempty"`

	PlannerVersion               int    `json:"planner_version"`
	PlannerReadOnly              bool   `json:"planner_read_only"`
	PlannerDiagnosticCode        string `json:"planner_diagnostic_code,omitempty"`
	PlannerFaultDomain           string `json:"planner_fault_domain,omitempty"`
	PlannerStrategyRelevant      bool   `json:"planner_strategy_relevant"`
	PlannerCompatibleCount       int    `json:"planner_compatible_count"`
	PlannerPromotedCount         int    `json:"planner_promoted_count"`
	PlannerAdmittedRegistryCount int    `json:"planner_admitted_registry_count"`
}

type v2SelectorAutoPoolMeta struct {
	Enabled               bool
	Added                 int
	MemoryCandidates      int
	LibraryCandidates     int
	BuiltinCandidates     int
	SynthesizedCandidates int
	Sources               []string
	Warnings              []string
	RecommendationAware   bool
	RecommendationHints   int
	RecommendationAdded   int

	PlannerVersion               int
	PlannerDiagnosticCode        string
	PlannerFaultDomain           string
	PlannerStrategyRelevant      bool
	PlannerCompatibleCount       int
	PlannerPromotedCount         int
	PlannerAdmittedRegistryCount int
	PlannerPlan                  []v2CandidatePoolItem
}

type v2BuiltinCandidate struct {
	ID       string
	Name     string
	Family   string
	Protocol string
	Args     []string
}

var v2BuiltinHTTPSCandidates = []v2BuiltinCandidate{
	{
		ID: "builtin-tls-multisplit-midsld", Name: "TLS multisplit midsld",
		Family: "split", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,midsld"},
	},
	{
		ID: "builtin-tls-multisplit-sniext", Name: "TLS multisplit sniext",
		Family: "split", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,sniext+1"},
	},
	{
		ID: "builtin-tls-multisplit-host", Name: "TLS multisplit host",
		Family: "split", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multisplit:pos=1,host+1"},
	},
	{
		ID: "builtin-tls-multidisorder-midsld", Name: "TLS multidisorder midsld",
		Family: "disorder", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,midsld"},
	},
	{
		ID: "builtin-tls-multidisorder-sniext", Name: "TLS multidisorder sniext",
		Family: "disorder", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=multidisorder:pos=1,sniext+1"},
	},
	{
		ID: "builtin-tls-fakedsplit-midsld", Name: "TLS fakedsplit midsld",
		Family: "fake-split", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakedsplit:pos=midsld"},
	},
	{
		ID: "builtin-tls-fakeddisorder-midsld", Name: "TLS fakeddisorder midsld",
		Family: "fake-disorder", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fakeddisorder:pos=midsld"},
	},
	{
		ID: "builtin-tls-inline-fake-multisplit", Name: "TLS inline fake + multisplit",
		Family: "fake+split", Protocol: "https",
		Args: []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=0x00000000:tcp_ack=-66000:repeats=2", "--lua-desync=multisplit:pos=1,midsld"},
	},
}

var v2BuiltinHTTPCandidates = []v2BuiltinCandidate{
	{
		ID: "builtin-http-hostcase", Name: "HTTP hostcase",
		Family: "http-case", Protocol: "http",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=http_hostcase"},
	},
	{
		ID: "builtin-http-domcase", Name: "HTTP domcase",
		Family: "http-case", Protocol: "http",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=http_domcase"},
	},
	{
		ID: "builtin-http-methodeol", Name: "HTTP method EOL",
		Family: "http-method", Protocol: "http",
		Args: []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req", "--lua-desync=http_methodeol"},
	},
}

var v2BuiltinQUICCandidates = []v2BuiltinCandidate{
	{ID: "builtin-quic-fake-1", Name: "QUIC fake x1", Family: "fake", Protocol: "quic", Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=1"}},
	{ID: "builtin-quic-fake-2", Name: "QUIC fake x2", Family: "fake", Protocol: "quic", Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=2"}},
	{ID: "builtin-quic-fake-3", Name: "QUIC fake x3", Family: "fake", Protocol: "quic", Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=3"}},
	{ID: "builtin-quic-fake-5", Name: "QUIC fake x5", Family: "fake", Protocol: "quic", Args: []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=5"}},
}

var v2BuiltinSTUNCandidates = []v2BuiltinCandidate{
	{ID: "builtin-stun-fake-1", Name: "STUN fake x1", Family: "fake", Protocol: "stun", Args: []string{"--filter-udp=3478", "--filter-l7=stun", "--payload=stun", "--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=1"}},
	{ID: "builtin-stun-fake-2", Name: "STUN fake x2", Family: "fake", Protocol: "stun", Args: []string{"--filter-udp=3478", "--filter-l7=stun", "--payload=stun", "--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=2"}},
	{ID: "builtin-stun-fake-3", Name: "STUN fake x3", Family: "fake", Protocol: "stun", Args: []string{"--filter-udp=3478", "--filter-l7=stun", "--payload=stun", "--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=3"}},
	{ID: "builtin-stun-fake-5", Name: "STUN fake x5", Family: "fake", Protocol: "stun", Args: []string{"--filter-udp=3478", "--filter-l7=stun", "--payload=stun", "--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=5"}},
}

func v2BuiltinCandidatesForTransport(transport benchTransportProfile) []v2BuiltinCandidate {
	var items []v2BuiltinCandidate
	switch transport.ID {
	case benchTransportHTTPS:
		items = v2BuiltinHTTPSCandidates
	case benchTransportHTTP:
		items = v2BuiltinHTTPCandidates
	case benchTransportQUIC:
		items = v2BuiltinQUICCandidates
	case benchTransportSTUN:
		items = v2BuiltinSTUNCandidates
	default:
		return nil
	}
	return append([]v2BuiltinCandidate{}, items...)
}

func v2PortableCandidateArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--hostlist-domains="),
			strings.HasPrefix(arg, "--hostlist="),
			strings.HasPrefix(arg, "--hostlist-auto="),
			strings.HasPrefix(arg, "--hostlist-exclude="),
			strings.HasPrefix(arg, "--ipset="),
			strings.HasPrefix(arg, "--ipset-exclude="):
			continue
		}
		out = append(out, arg)
	}
	return out
}

func v2CandidateTechniqueFingerprint(args []string) string {
	return v2StrategyFingerprint(v2PortableCandidateArgs(args))
}

func v2CandidateSourceList(items []v2CandidatePoolItem) []string {
	set := map[string]bool{}
	for _, item := range items {
		if item.Source != "" {
			set[item.Source] = true
		}
	}
	out := make([]string, 0, len(set))
	for source := range set {
		out = append(out, source)
	}
	sort.Strings(out)
	return out
}

func v2AppendPoolItem(out []v2CandidatePoolItem, seen map[string]bool, item v2CandidatePoolItem) []v2CandidatePoolItem {
	item.Args = v2PortableCandidateArgs(item.Args)
	if len(item.Args) == 0 {
		return out
	}
	item.Fingerprint = v2CandidateTechniqueFingerprint(item.Args)
	if item.Fingerprint == "" || seen[item.Fingerprint] {
		return out
	}
	seen[item.Fingerprint] = true
	return append(out, item)
}

func v2RecommendationSource(entry v2StrategyRegistryEntry) string {
	for _, preferred := range []string{"catalog", "import", "custom", "curated", "other", "builtin", "memory"} {
		for _, source := range entry.Sources {
			if strings.EqualFold(strings.TrimSpace(source), preferred) {
				return preferred
			}
		}
	}
	if len(entry.Sources) > 0 {
		return strings.ToLower(strings.TrimSpace(entry.Sources[0]))
	}
	return "registry"
}

func v2RecommendationProtocolCompatible(protocol string, transport benchTransportProfile) bool {
	return strings.EqualFold(strings.TrimSpace(protocol), strings.TrimSpace(transport.ID))
}

func v2BuildRecommendationPoolItems(target string, transport benchTransportProfile, limit int) ([]v2CandidatePoolItem, int, error) {
	if limit <= 0 {
		return []v2CandidatePoolItem{}, 0, nil
	}
	library, err := readV2StrategyLibrary()
	if err != nil {
		return nil, 0, err
	}
	memory, err := readV2TargetMemory()
	if err != nil {
		return nil, 0, err
	}
	registry := v2BuildStrategyRegistry(library, memory, v2TargetMemoryNow().UTC())
	scores := v2BuildStrategyScores(registry)
	insights := v2BuildStrategyInsights(scores)
	gate := v2BuildPolicyAutomationGate(scores, insights)
	recommendations := v2BuildStrategyRecommendations(scores, insights, gate)

	entryByFingerprint := map[string]v2StrategyRegistryEntry{}
	for _, entry := range registry.Entries {
		entryByFingerprint[entry.Fingerprint] = entry
	}
	out := make([]v2CandidatePoolItem, 0, limit)
	compatible := 0
	for _, recommendation := range recommendations.Items {
		if !v2RecommendationProtocolCompatible(recommendation.Protocol, transport) {
			continue
		}
		compatible++
		if len(out) >= limit {
			continue
		}
		entry, ok := entryByFingerprint[recommendation.Fingerprint]
		if !ok || !entry.Capabilities.CandidateReady {
			continue
		}
		transportReady := false
		for _, candidateTransport := range entry.Capabilities.BenchTransports {
			if strings.EqualFold(candidateTransport, transport.ID) {
				transportReady = true
				break
			}
		}
		if !transportReady {
			continue
		}
		if _, compileErr := v2CustomProfileForTransport(entry.Args, target, transport); compileErr != nil {
			continue
		}
		source := v2RecommendationSource(entry)
		stage := v2ProgressiveStageQuick
		if source == "memory" {
			stage = v2ProgressiveStageMemory
		}
		out = append(out, v2CandidatePoolItem{
			ID: entry.ID, Name: entry.Name, Source: source, Family: entry.Family,
			Protocol: transport.ID, Args: append([]string{}, entry.Args...), Stage: stage,
			HistoricalRank: recommendation.Rank, HistoricalScore: recommendation.Score,
			HistoricalInsight: recommendation.Insight, HistoricalPrimary: recommendation.Primary,
			HistoricalPromoted: true, HistoricalReason: recommendation.Reason,
		})
	}
	return out, compatible, nil
}

func v2BuildCandidatePoolForTransportWithHint(target, mode, transportID string, hint v2PlannerHint) (v2CandidatePoolResponse, error) {
	target, err := v2NormalizeTarget(target)
	if err != nil {
		return v2CandidatePoolResponse{}, err
	}
	m, err := v2SelectorMode(mode)
	if err != nil {
		return v2CandidatePoolResponse{}, err
	}
	transport, err := normalizeBenchTransport(transportID)
	if err != nil {
		return v2CandidatePoolResponse{}, err
	}
	status := readStatus()
	resp := v2CandidatePoolResponse{
		OK: true, Target: target, Mode: m.Name,
		Transport: transport.ID, Network: transport.Network, RemotePort: transport.RemotePort, MetricScope: transport.MetricScope,
		ConfigSHA: status.ConfigSHA256, Candidates: []v2CandidatePoolItem{}, Warnings: []string{},
	}
	seen := map[string]bool{}

	// Reserve half of every selector mode for newly composed candidates. Historical
	// evidence stays valuable, but it must never consume the entire search budget.
	synthesisReserve := v2SynthesisBudget(m, m.MaxCandidates)
	knownLimit := m.MaxCandidates - synthesisReserve
	if knownLimit < 0 {
		knownLimit = 0
	}

	memory, memoryErr := v2TargetMemoryCandidatesForTransport(target, status.ConfigSHA256, transport.ID, knownLimit)
	if memoryErr != nil {
		resp.Warnings = append(resp.Warnings, "memory: "+memoryErr.Error())
	} else {
		for _, item := range memory {
			item.Stage = v2ProgressiveStageMemory
			resp.Candidates = v2AppendPoolItem(resp.Candidates, seen, item)
			if len(resp.Candidates) >= knownLimit {
				break
			}
		}
	}

	if len(resp.Candidates) < knownLimit {
		recommended, compatible, recommendationErr := v2BuildRecommendationPoolItems(target, transport, knownLimit-len(resp.Candidates))
		if recommendationErr != nil {
			resp.Warnings = append(resp.Warnings, "recommendations: "+recommendationErr.Error())
		} else {
			resp.RecommendationAware = true
			resp.RecommendationHints = compatible
			for _, item := range recommended {
				item.Args = v2PortableCandidateArgs(item.Args)
				item.Fingerprint = v2CandidateTechniqueFingerprint(item.Args)
				if item.Fingerprint != "" && seen[item.Fingerprint] {
					if v2MergeHistoricalRecommendation(resp.Candidates, item) {
						resp.HistoricalExistingPromoted++
					}
					continue
				}
				before := len(resp.Candidates)
				resp.Candidates = v2AppendPoolItem(resp.Candidates, seen, item)
				if len(resp.Candidates) > before {
					resp.RecommendationAdded++
				}
				if len(resp.Candidates) >= knownLimit {
					break
				}
			}
		}
	}

	synthesisBudget := v2SynthesisBudget(m, m.MaxCandidates-len(resp.Candidates))
	synthesized, synthesisMeta := v2SynthesizeCandidates(target, transport, m, hint, synthesisBudget)
	resp.SynthesisVersion = synthesisMeta.Version
	resp.SynthesisGenerated = synthesisMeta.Generated
	resp.SynthesisFamilies = append([]string{}, synthesisMeta.Families...)
	resp.SynthesisBasis = synthesisMeta.Basis
	for _, item := range synthesized {
		before := len(resp.Candidates)
		resp.Candidates = v2AppendPoolItem(resp.Candidates, seen, item)
		if len(resp.Candidates) > before {
			resp.SynthesisAdmitted++
		}
		if len(resp.Candidates) >= m.MaxCandidates {
			break
		}
	}

	// Unverified saved Library entries are deliberately not injected into Auto Pool.
	// The UI sends them separately when the user enables Candidate Library; proven
	// library entries may still return through historical recommendations above.

	if len(resp.Candidates) < m.MaxCandidates {
		corpus := v2CorpusCandidatesForTransport(transport)
		quickLimit := v2ProgressiveQuickLimit(m)
		for i, item := range corpus {
			if _, compileErr := v2CustomProfileForTransport(item.Args, target, transport); compileErr != nil {
				resp.Warnings = append(resp.Warnings, item.ID+": "+compileErr.Error())
				continue
			}
			stage := v2ProgressiveStageFull
			if i < quickLimit {
				stage = v2ProgressiveStageQuick
			}
			poolItem := v2CandidatePoolItem{
				ID: item.ID, Name: item.Name, Source: item.Source, Family: item.Family,
				Protocol: item.Protocol, Args: append([]string{}, item.Args...), Stage: stage,
			}
			resp.Candidates = v2AppendPoolItem(resp.Candidates, seen, poolItem)
			if len(resp.Candidates) >= m.MaxCandidates {
				break
			}
		}
	}

	if len(resp.Candidates) < m.MaxCandidates {
		builtins := v2BuiltinCandidatesForTransport(transport)
		quickLimit := v2ProgressiveQuickLimit(m)
		for i, item := range builtins {
			if _, compileErr := v2CustomProfileForTransport(item.Args, target, transport); compileErr != nil {
				resp.Warnings = append(resp.Warnings, item.ID+": "+compileErr.Error())
				continue
			}
			stage := v2ProgressiveStageFull
			if i < quickLimit {
				stage = v2ProgressiveStageQuick
			}
			poolItem := v2CandidatePoolItem{
				ID: item.ID, Name: item.Name, Source: "builtin", Family: item.Family,
				Protocol: item.Protocol, Args: append([]string{}, item.Args...), Stage: stage,
			}
			resp.Candidates = v2AppendPoolItem(resp.Candidates, seen, poolItem)
			if len(resp.Candidates) >= m.MaxCandidates {
				break
			}
		}
	}

	resp.Count = len(resp.Candidates)
	resp.Sources = v2CandidateSourceList(resp.Candidates)
	return resp, nil
}

func populateV2SelectorCandidates(req *v2SelectorRequest) (v2SelectorAutoPoolMeta, error) {
	meta := v2SelectorAutoPoolMeta{Sources: []string{}, Warnings: []string{}}
	enabled := true
	if req.AutoPool != nil {
		enabled = *req.AutoPool
	}
	meta.Enabled = enabled
	if !enabled {
		return meta, nil
	}

	if _, err := v2NormalizeTarget(req.ServerName); err != nil {
		return meta, err
	}
	mode, err := v2SelectorMode(req.Mode)
	if err != nil {
		return meta, err
	}
	transport, err := normalizeBenchTransport(benchTransportHTTPS)
	if err != nil {
		return meta, err
	}

	// P26A direct execution model:
	// Auto Pool is the portable corpus itself. No planner, synthesis, historical
	// promotion or adaptive mutation is allowed to replace real candidate execution.
	external := append([]v2SelectorCandidateInput{}, req.Candidates...)
	req.Candidates = []v2SelectorCandidateInput{}
	seen := map[string]bool{}
	sourceSet := map[string]bool{}

	corpus := v2CorpusCandidatesForTransport(transport)
	autoLimit := v2DirectAutoPoolLimit(mode.Name, len(corpus))
	for _, item := range corpus {
		if len(req.Candidates) >= autoLimit {
			break
		}
		fp := v2CandidateTechniqueFingerprint(item.Args)
		if fp == "" || seen[fp] {
			continue
		}
		seen[fp] = true
		req.Candidates = append(req.Candidates, v2SelectorCandidateInput{
			ID: item.ID, Name: item.Name, Source: item.Source, Args: append([]string{}, item.Args...),
		})
		meta.Added++
		meta.LibraryCandidates++
		if item.Source != "" {
			sourceSet[item.Source] = true
		}
	}

	for _, existing := range external {
		if len(req.Candidates) >= v2DirectSelectorMaxCandidates {
			break
		}
		fp := v2CandidateTechniqueFingerprint(existing.Args)
		if fp == "" || seen[fp] {
			continue
		}
		seen[fp] = true
		req.Candidates = append(req.Candidates, existing)
		if existing.Source != "" {
			sourceSet[v2StrategySource(existing.Source)] = true
		}
	}

	for source := range sourceSet {
		meta.Sources = append(meta.Sources, source)
	}
	sort.Strings(meta.Sources)
	meta.Warnings = append(meta.Warnings, "P26 direct catalog execution: planner, synthesis and adaptive mutation are bypassed")
	return meta, nil
}
