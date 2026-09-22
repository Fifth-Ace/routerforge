package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const (
	v2DPIPropertyProbeVersion = 1
	v2DPIPropertyProbeConfirm = "ROUTERFORGE_DPI_PROPERTY_PROBE_V1"

	v2DPIPropertyHelps      = "HELPS"
	v2DPIPropertyMixed      = "MIXED"
	v2DPIPropertyNoEffect   = "NO_EFFECT"
	v2DPIPropertyUnmeasured = "UNMEASURED"
)

type v2DPIPropertyProbeRequest struct {
	Mode                 string `json:"mode"`
	ServerName           string `json:"server_name"`
	Transport            string `json:"transport,omitempty"`
	ExpectedConfigSHA256 string `json:"expected_config_sha256"`
	Confirm              string `json:"confirm"`
}

type v2DPIPropertyProbeSpec struct {
	ID       string
	Name     string
	Families []string
	Args     []string
}

type v2DPIPropertyObservation struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Families         []string          `json:"families"`
	State            string            `json:"state"`
	Attempts         int               `json:"attempts"`
	Successes        int               `json:"successes"`
	ResultClass      string            `json:"result_class"`
	CleanupProven    bool              `json:"cleanup_proven"`
	InfrastructureOK bool              `json:"infrastructure_ok"`
	Args             []string          `json:"args"`
	Result           v2CandidateResult `json:"result"`
}

type v2DPIPropertyVector struct {
	Version               int                        `json:"version"`
	Transport             string                     `json:"transport"`
	BaselineWorking       bool                       `json:"baseline_working"`
	EvidenceComplete      bool                       `json:"evidence_complete"`
	Probes                []v2DPIPropertyObservation `json:"probes"`
	FamilyScores          map[string]int             `json:"family_scores"`
	PreferredFamilies     []string                   `json:"preferred_families"`
	DeprioritizedFamilies []string                   `json:"deprioritized_families"`
	Reason                string                     `json:"reason"`
}

type v2DPIPropertyProbeResponse struct {
	OK                   bool                `json:"ok"`
	Target               string              `json:"target"`
	DestinationIPv4      string              `json:"destination_ipv4"`
	Transport            string              `json:"transport"`
	Mode                 string              `json:"mode"`
	ConfigSHA256         string              `json:"config_sha256"`
	Baseline             v2CandidateResult   `json:"baseline"`
	Vector               v2DPIPropertyVector `json:"vector"`
	CleanupBaselineAfter bool                `json:"cleanup_baseline_after"`
	Warnings             []string            `json:"warnings"`
}

func registerDPIPropertyProbeV1Route(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/property-probe", mutationOnly(handleV2DPIPropertyProbe))
}

func v2DPIPropertyProbeLimit(mode benchAutoTuneMode) int {
	switch mode.Name {
	case "fast":
		return 4
	case "thorough":
		return 8
	default:
		return 6
	}
}

func v2DPIPropertyProbeAttempts(mode benchAutoTuneMode) int {
	if mode.Name == "thorough" {
		return 2
	}
	return 1
}

func v2DPIPropertyProbeCatalog(transport benchTransportProfile) []v2DPIPropertyProbeSpec {
	base := v2SynthesisBaseArgs(transport)
	with := func(actions ...string) []string {
		out := append([]string{}, base...)
		return append(out, actions...)
	}
	switch transport.ID {
	case benchTransportHTTPS:
		return []v2DPIPropertyProbeSpec{
			{ID: "tls-split", Name: "TLS split", Families: []string{"split"}, Args: with("--lua-desync=multisplit:pos=1,midsld")},
			{ID: "tls-disorder", Name: "TLS disorder", Families: []string{"disorder"}, Args: with("--lua-desync=multidisorder:pos=1,midsld")},
			{ID: "tls-fake-split", Name: "TLS fake + split", Families: []string{"fake+split"}, Args: with("--lua-desync=fake:blob=tls_clienthello:badsum", "--lua-desync=multisplit:pos=1,midsld")},
			{ID: "tls-fake-disorder", Name: "TLS fake + disorder", Families: []string{"fake+disorder"}, Args: with("--lua-desync=fake:blob=tls_clienthello:badsum", "--lua-desync=multidisorder:pos=1,midsld")},
			{ID: "tls-overlap", Name: "TLS overlap", Families: []string{"split", "fake-split"}, Args: with("--lua-desync=multisplit:pos=1,midsld:seqovl=336:seqovl_pattern=tls_clienthello")},
			{ID: "tls-fakedsplit", Name: "TLS fake split", Families: []string{"fake-split"}, Args: with("--lua-desync=fakedsplit:pos=midsld:seqovl=1:seqovl_pattern=tls_clienthello")},
			{ID: "tls-fakeddisorder", Name: "TLS fake disorder", Families: []string{"fake-disorder"}, Args: with("--lua-desync=fakeddisorder:pos=midsld:seqovl=1:seqovl_pattern=tls_clienthello")},
			{ID: "tls-ttl-fake", Name: "TLS TTL-limited fake", Families: []string{"fake+split", "fake+disorder"}, Args: with("--lua-desync=fake:blob=tls_clienthello:ip_ttl=4", "--lua-desync=multisplit:pos=1,midsld")},
		}
	case benchTransportHTTP:
		return []v2DPIPropertyProbeSpec{
			{ID: "http-split", Name: "HTTP split", Families: []string{"split"}, Args: with("--lua-desync=multisplit:pos=method+2,host+1")},
			{ID: "http-disorder", Name: "HTTP disorder", Families: []string{"disorder"}, Args: with("--lua-desync=multidisorder:pos=method+2,host+1")},
			{ID: "http-methodeol", Name: "HTTP method EOL", Families: []string{"http-method"}, Args: with("--lua-desync=http_methodeol")},
			{ID: "http-fake-split", Name: "HTTP fake + split", Families: []string{"fake+split"}, Args: with("--lua-desync=fake:blob=http_req", "--lua-desync=multisplit:pos=method+2,host+1")},
			{ID: "http-fake-disorder", Name: "HTTP fake + disorder", Families: []string{"fake+disorder"}, Args: with("--lua-desync=fake:blob=http_req", "--lua-desync=multidisorder:pos=method+2,host+1")},
			{ID: "http-methodeol-badsum", Name: "HTTP method EOL + badsum", Families: []string{"http-method"}, Args: with("--lua-desync=http_methodeol:badsum")},
		}
	case benchTransportQUIC:
		return []v2DPIPropertyProbeSpec{
			{ID: "quic-fake-2", Name: "QUIC fake x2", Families: []string{"fake"}, Args: with("--lua-desync=fake:blob=fake_default_quic:repeats=2")},
			{ID: "quic-fake-6", Name: "QUIC fake x6", Families: []string{"fake"}, Args: with("--lua-desync=fake:blob=fake_default_quic:repeats=6")},
			{ID: "quic-fake-11", Name: "QUIC fake x11", Families: []string{"fake"}, Args: with("--lua-desync=fake:blob=fake_default_quic:repeats=11")},
			{ID: "quic-ttl-5", Name: "QUIC fake TTL 5", Families: []string{"fake"}, Args: with("--lua-desync=fake:blob=fake_default_quic:repeats=2:ip_ttl=5")},
			{ID: "quic-udplen-8", Name: "QUIC UDP length +8", Families: []string{"udplen"}, Args: with("--lua-desync=udplen:payload=quic_initial:dir=out:increment=8")},
			{ID: "quic-udplen-25", Name: "QUIC UDP length +25", Families: []string{"udplen"}, Args: with("--lua-desync=udplen:payload=quic_initial:dir=out:increment=25")},
		}
	case benchTransportSTUN:
		return []v2DPIPropertyProbeSpec{
			{ID: "stun-fake-2", Name: "STUN fake x2", Families: []string{"fake"}, Args: with("--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=2")},
			{ID: "stun-fake-6", Name: "STUN fake x6", Families: []string{"fake"}, Args: with("--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=6")},
			{ID: "stun-fake-11", Name: "STUN fake x11", Families: []string{"fake"}, Args: with("--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=11")},
			{ID: "stun-ttl-5", Name: "STUN fake TTL 5", Families: []string{"fake"}, Args: with("--lua-desync=fake:blob=0x00000000000000000000000000000000:repeats=2:ip_ttl=5")},
		}
	default:
		return nil
	}
}

func v2DPIPropertyObservationState(result v2CandidateResult) string {
	if !result.CleanupProven || !result.InfrastructureOK || len(result.Attempts) == 0 {
		return v2DPIPropertyUnmeasured
	}
	if result.SuccessRate == 1 {
		return v2DPIPropertyHelps
	}
	if result.Successes > 0 {
		return v2DPIPropertyMixed
	}
	return v2DPIPropertyNoEffect
}

func v2FinalizeDPIPropertyVector(vector *v2DPIPropertyVector) {
	vector.FamilyScores = map[string]int{}
	vector.PreferredFamilies = []string{}
	vector.DeprioritizedFamilies = []string{}
	vector.EvidenceComplete = true
	for _, probe := range vector.Probes {
		delta := 0
		switch probe.State {
		case v2DPIPropertyHelps:
			delta = 100
		case v2DPIPropertyMixed:
			delta = 40
		case v2DPIPropertyNoEffect:
			delta = -15
		default:
			vector.EvidenceComplete = false
		}
		for _, family := range probe.Families {
			family = strings.TrimSpace(family)
			if family != "" {
				vector.FamilyScores[family] += delta
			}
		}
	}
	for family, score := range vector.FamilyScores {
		switch {
		case score > 0:
			vector.PreferredFamilies = append(vector.PreferredFamilies, family)
		case score < 0:
			vector.DeprioritizedFamilies = append(vector.DeprioritizedFamilies, family)
		}
	}
	sort.Slice(vector.PreferredFamilies, func(i, j int) bool {
		a, b := vector.PreferredFamilies[i], vector.PreferredFamilies[j]
		if vector.FamilyScores[a] != vector.FamilyScores[b] {
			return vector.FamilyScores[a] > vector.FamilyScores[b]
		}
		return a < b
	})
	sort.Slice(vector.DeprioritizedFamilies, func(i, j int) bool {
		a, b := vector.DeprioritizedFamilies[i], vector.DeprioritizedFamilies[j]
		if vector.FamilyScores[a] != vector.FamilyScores[b] {
			return vector.FamilyScores[a] < vector.FamilyScores[b]
		}
		return a < b
	})
	if vector.BaselineWorking {
		vector.Reason = "baseline already works; no DPI technique probing is required"
	} else if len(vector.PreferredFamilies) > 0 {
		vector.Reason = "live property probes prioritize: " + strings.Join(vector.PreferredFamilies, ", ")
	} else if vector.EvidenceComplete {
		vector.Reason = "live property probes found no directly successful family; synthesis remains broad"
	} else {
		vector.Reason = "property evidence is incomplete; synthesis remains broad and live verification stays authoritative"
	}
}

func v2PropertyFamilyScore(vector *v2DPIPropertyVector, family string) int {
	if vector == nil || vector.Version != v2DPIPropertyProbeVersion {
		return 0
	}
	return vector.FamilyScores[strings.TrimSpace(family)]
}

func v2NormalizeDPIPropertyVector(input *v2DPIPropertyVector, transportID string) (*v2DPIPropertyVector, error) {
	if input == nil {
		return nil, nil
	}
	transport, err := normalizeBenchTransport(transportID)
	if err != nil {
		return nil, err
	}
	if input.Version != v2DPIPropertyProbeVersion {
		return nil, errors.New("unsupported DPI property vector version")
	}
	if input.Transport != transport.ID {
		return nil, errors.New("DPI property vector transport mismatch")
	}
	if len(input.Probes) > 16 {
		return nil, errors.New("DPI property vector exceeds probe limit")
	}
	out := &v2DPIPropertyVector{
		Version: v2DPIPropertyProbeVersion, Transport: transport.ID,
		BaselineWorking: input.BaselineWorking, Probes: make([]v2DPIPropertyObservation, 0, len(input.Probes)),
	}
	for _, probe := range input.Probes {
		if len(probe.ID) == 0 || len(probe.ID) > 64 || len(probe.Families) == 0 || len(probe.Families) > 4 {
			return nil, errors.New("invalid DPI property observation")
		}
		switch probe.State {
		case v2DPIPropertyHelps, v2DPIPropertyMixed, v2DPIPropertyNoEffect, v2DPIPropertyUnmeasured:
		default:
			return nil, errors.New("invalid DPI property state")
		}
		cp := probe
		cp.Families = append([]string{}, probe.Families...)
		cp.Args = append([]string{}, probe.Args...)
		out.Probes = append(out.Probes, cp)
	}
	v2FinalizeDPIPropertyVector(out)
	return out, nil
}

func validateV2DPIPropertyProbeRequest(req v2DPIPropertyProbeRequest) (benchAutoTuneMode, benchTransportProfile, string, error) {
	mode, err := v2SelectorMode(req.Mode)
	if err != nil {
		return benchAutoTuneMode{}, benchTransportProfile{}, "", err
	}
	target, err := v2NormalizeTarget(req.ServerName)
	if err != nil {
		return benchAutoTuneMode{}, benchTransportProfile{}, "", err
	}
	transport, err := normalizeBenchTransport(req.Transport)
	if err != nil {
		return benchAutoTuneMode{}, benchTransportProfile{}, "", err
	}
	if req.Confirm != v2DPIPropertyProbeConfirm {
		return benchAutoTuneMode{}, benchTransportProfile{}, "", errors.New("confirm must equal " + v2DPIPropertyProbeConfirm)
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(req.ExpectedConfigSHA256)) {
		return benchAutoTuneMode{}, benchTransportProfile{}, "", errors.New("expected_config_sha256 must be SHA256")
	}
	return mode, transport, target, nil
}

func handleV2DPIPropertyProbe(w http.ResponseWriter, r *http.Request) {
	var req v2DPIPropertyProbeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid DPI property probe request"})
		return
	}
	mode, transport, target, err := validateV2DPIPropertyProbeRequest(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !atomic.CompareAndSwapInt32(&benchSmokeActive, 0, 1) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "another bench session is active"})
		return
	}
	defer atomic.StoreInt32(&benchSmokeActive, 0)

	status := readStatus()
	if !strings.EqualFold(status.ConfigSHA256, strings.TrimSpace(req.ExpectedConfigSHA256)) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "production config changed before DPI property probe", "current_sha256": status.ConfigSHA256})
		return
	}
	capabilities := readBenchCapabilities()
	if !benchExecutionReady(capabilities) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "DPI property probe capability gates are not proven"})
		return
	}
	inventory := readBenchStrategyInventory()
	if !inventory.BaseDependenciesProven {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "strategy base dependencies are not proven"})
		return
	}
	queues := v2FreeBenchQueues(capabilities.OccupiedQueues, 1)
	if len(queues) == 0 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "no free reserved NFQUEUE for DPI property probe"})
		return
	}

	resolveCtx, cancelResolve := context.WithTimeout(r.Context(), 4*time.Second)
	ip, err := resolveBenchServerIPv4(resolveCtx, target)
	cancelResolve()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "resolve target: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(mode.TimeoutSec)*time.Second)
	defer cancel()
	baseline := v2CandidateResult{
		Baseline: true, CandidateID: "property-baseline", CandidateName: "Property baseline",
		CandidateSource: "property-probe", SourceProfileIndex: -1, CleanupProven: true,
	}
	baseline.Attempts = append(baseline.Attempts, v2RunTransportAttempt(ctx, capabilities, status.ConfigSHA256, target, ip, inventory, nil, queues[0], transport))
	v2FinalizeCandidate(&baseline)
	vector := v2DPIPropertyVector{
		Version: v2DPIPropertyProbeVersion, Transport: transport.ID,
		BaselineWorking: baseline.ResultClass == "WORKING" && baseline.SuccessRate == 1,
		Probes:          []v2DPIPropertyObservation{},
	}
	response := v2DPIPropertyProbeResponse{
		Target: target, DestinationIPv4: ip, Transport: transport.ID, Mode: mode.Name,
		ConfigSHA256: status.ConfigSHA256, Baseline: baseline, Vector: vector, Warnings: []string{},
	}
	if !baseline.CleanupProven || !baseline.InfrastructureOK {
		v2FinalizeDPIPropertyVector(&response.Vector)
		after := readBenchCapabilities()
		response.CleanupBaselineAfter = after.CleanupBaselineProven
		writeJSON(w, http.StatusBadGateway, response)
		return
	}
	if vector.BaselineWorking {
		v2FinalizeDPIPropertyVector(&response.Vector)
		after := readBenchCapabilities()
		response.CleanupBaselineAfter = after.CleanupBaselineProven
		response.OK = response.CleanupBaselineAfter && strings.EqualFold(readStatus().ConfigSHA256, status.ConfigSHA256)
		if !response.OK {
			writeJSON(w, http.StatusBadGateway, response)
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}

	catalog := v2DPIPropertyProbeCatalog(transport)
	limit := v2DPIPropertyProbeLimit(mode)
	if limit > len(catalog) {
		limit = len(catalog)
	}
	attempts := v2DPIPropertyProbeAttempts(mode)
	for _, spec := range catalog[:limit] {
		profile, compileErr := v2CustomProfileForTransport(spec.Args, target, transport)
		if compileErr != nil {
			response.Warnings = append(response.Warnings, spec.ID+": "+compileErr.Error())
			response.Vector.Probes = append(response.Vector.Probes, v2DPIPropertyObservation{
				ID: spec.ID, Name: spec.Name, Families: append([]string{}, spec.Families...), State: v2DPIPropertyUnmeasured,
				Args: append([]string{}, spec.Args...), Result: v2CandidateResult{CandidateID: spec.ID, CandidateName: spec.Name, CandidateSource: "property-probe"},
			})
			continue
		}
		profile.Index = -1
		result := v2CandidateResult{
			CandidateID: spec.ID, CandidateName: spec.Name, CandidateSource: "property-probe",
			SourceProfileIndex: -1, Args: append([]string{}, profile.Args...), CleanupProven: true,
		}
		for i := 0; i < attempts; i++ {
			attempt := v2RunTransportAttempt(ctx, capabilities, status.ConfigSHA256, target, ip, inventory, &profile, queues[0], transport)
			result.Attempts = append(result.Attempts, attempt)
			if !attempt.CleanupProven || !attempt.InfrastructureOK {
				break
			}
		}
		v2FinalizeCandidate(&result)
		response.Vector.Probes = append(response.Vector.Probes, v2DPIPropertyObservation{
			ID: spec.ID, Name: spec.Name, Families: append([]string{}, spec.Families...),
			State: v2DPIPropertyObservationState(result), Attempts: len(result.Attempts), Successes: result.Successes,
			ResultClass: result.ResultClass, CleanupProven: result.CleanupProven, InfrastructureOK: result.InfrastructureOK,
			Args: append([]string{}, spec.Args...), Result: result,
		})
		if !result.CleanupProven || !result.InfrastructureOK {
			break
		}
	}
	v2FinalizeDPIPropertyVector(&response.Vector)
	after := readBenchCapabilities()
	response.CleanupBaselineAfter = after.CleanupBaselineProven
	response.OK = response.CleanupBaselineAfter && strings.EqualFold(readStatus().ConfigSHA256, status.ConfigSHA256)
	if !response.OK {
		writeJSON(w, http.StatusBadGateway, response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func v2DPIPropertyVectorSummary(vector *v2DPIPropertyVector) string {
	if vector == nil {
		return "property probe not available"
	}
	if vector.BaselineWorking {
		return "baseline already works"
	}
	measured := 0
	for _, probe := range vector.Probes {
		if probe.State != v2DPIPropertyUnmeasured {
			measured++
		}
	}
	return fmt.Sprintf("property probe v%d: %d/%d measured; preferred=%s", vector.Version, measured, len(vector.Probes), strings.Join(vector.PreferredFamilies, ","))
}
