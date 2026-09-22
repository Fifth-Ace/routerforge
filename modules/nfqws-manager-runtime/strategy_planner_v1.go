package main

import (
	"sort"
	"strings"
)

const v2RecommendationPlannerVersion = 1

type v2PlannerHint struct {
	DiagnosticCode   string
	FaultDomain      string
	StrategyRelevant bool
	Properties       *v2DPIPropertyVector
}

type v2PlannerRankedCandidate struct {
	Item          v2CandidatePoolItem
	OriginalOrder int
	Score         int
}

func v2MergeHistoricalRecommendation(items []v2CandidatePoolItem, hint v2CandidatePoolItem) bool {
	fp := strings.TrimSpace(hint.Fingerprint)
	if fp == "" {
		fp = v2CandidateTechniqueFingerprint(hint.Args)
	}
	if fp == "" {
		return false
	}
	for i := range items {
		if items[i].Fingerprint != fp {
			continue
		}
		items[i].HistoricalRank = hint.HistoricalRank
		items[i].HistoricalScore = hint.HistoricalScore
		items[i].HistoricalInsight = hint.HistoricalInsight
		items[i].HistoricalPrimary = hint.HistoricalPrimary
		items[i].HistoricalPromoted = hint.HistoricalPromoted
		items[i].HistoricalReason = hint.HistoricalReason
		items[i].PlannerAlreadyInPool = true
		return true
	}
	return false
}

func v2PlannerHistoricalScore(item v2CandidatePoolItem) int {
	score := 0
	switch strings.ToUpper(strings.TrimSpace(item.HistoricalInsight)) {
	case "PROVEN":
		score += 50
	case "PROMISING":
		score += 35
	case "MIXED":
		score += 5
	case "WEAK":
		score -= 10
	case "DEGRADED":
		score -= 25
	}
	if item.HistoricalPrimary {
		score += 15
	}
	if item.HistoricalScore > 0 {
		bonus := item.HistoricalScore / 10
		if bonus > 10 {
			bonus = 10
		}
		score += bonus
	}
	if item.MemoryVerifiedCount > 0 {
		bonus := item.MemoryVerifiedCount
		if bonus > 10 {
			bonus = 10
		}
		score += bonus
	}
	return score
}

func v2PlannerDiagnosticScore(item v2CandidatePoolItem, hint v2PlannerHint) (int, string) {
	if !hint.StrategyRelevant {
		return 0, ""
	}
	code := strings.ToUpper(strings.TrimSpace(hint.DiagnosticCode))
	family := strings.ToLower(strings.TrimSpace(item.Family))
	switch {
	case strings.HasPrefix(code, "TLS_HANDSHAKE_"):
		switch family {
		case "split", "disorder", "fake-split", "fake-disorder", "fake+split":
			return 45, "TLS handshake fault: prioritize TCP/TLS split/disorder families for live verification"
		case "fake":
			return 20, "TLS handshake fault: fake family remains a secondary live candidate"
		}
	case code == "HTTP_STREAM_CUTOFF_12_20K_SUSPECTED":
		switch family {
		case "split", "disorder", "fake-split", "fake-disorder", "fake+split":
			return 50, "12-20 KiB cutoff suspicion: prioritize segmentation/disorder families; live proof remains mandatory"
		case "fake":
			return 15, "12-20 KiB cutoff suspicion: fake family is a secondary hint only"
		}
	case strings.HasPrefix(code, "HTTP_TRANSPORT_"),
		code == "HTTP_FIRST_BYTE_FAILURE",
		code == "HTTP_PROGRESS_NOT_PROVEN":
		switch family {
		case "split", "disorder", "fake-split", "fake-disorder", "fake+split":
			return 35, "HTTP transport fault after TLS: prioritize transport-shaping families for live verification"
		case "fake":
			return 15, "HTTP transport fault: fake family remains a secondary candidate"
		}
	case code == "TCP_CONNECT_RESET":
		switch family {
		case "disorder", "fake-disorder", "fake-split":
			return 20, "TCP reset may be path/DPI related: modestly prioritize reset-sensitive families"
		}
	}
	return 0, ""
}

func v2PlannerCandidateReason(item v2CandidatePoolItem, diagnosticReason string) string {
	parts := []string{}
	if strings.TrimSpace(item.HistoricalReason) != "" {
		parts = append(parts, "historical: "+strings.TrimSpace(item.HistoricalReason))
	}
	if diagnosticReason != "" {
		parts = append(parts, "diagnostic: "+diagnosticReason)
	}
	if item.PlannerAlreadyInPool {
		parts = append(parts, "existing pool candidate promoted in place; original source identity preserved")
	}
	if len(parts) == 0 {
		parts = append(parts, "normal deterministic candidate order; live verification decides outcome")
	}
	return strings.Join(parts, "; ")
}

func v2PlannerDiversify(items []v2PlannerRankedCandidate) {
	for i := 1; i < len(items); i++ {
		prev := items[i-1].Item
		cur := items[i].Item
		if prev.Family != cur.Family || prev.Source != cur.Source {
			continue
		}
		for j := i + 1; j < len(items) && j <= i+3; j++ {
			alt := items[j]
			if items[i].Score-alt.Score > 10 {
				break
			}
			if alt.Item.Family == prev.Family && alt.Item.Source == prev.Source {
				continue
			}
			items[i], items[j] = items[j], items[i]
			break
		}
	}
}

func v2ApplyRecommendationPlanner(resp *v2CandidatePoolResponse, hint v2PlannerHint) {
	if resp == nil {
		return
	}
	resp.PlannerVersion = v2RecommendationPlannerVersion
	resp.PlannerReadOnly = true
	resp.PlannerDiagnosticCode = strings.TrimSpace(hint.DiagnosticCode)
	resp.PlannerFaultDomain = strings.TrimSpace(hint.FaultDomain)
	resp.PlannerStrategyRelevant = hint.StrategyRelevant
	resp.PlannerCompatibleCount = resp.RecommendationHints
	resp.PlannerAdmittedRegistryCount = resp.RecommendationAdded

	ranked := make([]v2PlannerRankedCandidate, 0, len(resp.Candidates))
	for i, item := range resp.Candidates {
		item.PlannerOriginalOrder = i + 1
		item.PlannerOriginalStage = item.Stage
		item.PlannerEffectiveStage = item.Stage
		historicalScore := v2PlannerHistoricalScore(item)
		diagnosticScore, diagnosticReason := v2PlannerDiagnosticScore(item, hint)
		item.PlannerScore = historicalScore + diagnosticScore
		item.PlannerHistoricalHint = item.HistoricalPromoted || item.HistoricalRank > 0 || item.HistoricalInsight != ""
		item.PlannerDiagnosticHint = diagnosticScore > 0
		item.PlannerReason = v2PlannerCandidateReason(item, diagnosticReason)
		ranked = append(ranked, v2PlannerRankedCandidate{
			Item: item, OriginalOrder: i + 1, Score: item.PlannerScore,
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].OriginalOrder < ranked[j].OriginalOrder
	})
	v2PlannerDiversify(ranked)

	promoted := resp.HistoricalExistingPromoted
	for i := range ranked {
		ranked[i].Item.PlannerEffectiveOrder = i + 1
		if i+1 < ranked[i].OriginalOrder {
			promoted++
		}
		resp.Candidates[i] = ranked[i].Item
	}
	resp.PlannerPromotedCount = promoted
	resp.Count = len(resp.Candidates)
	resp.Sources = v2CandidateSourceList(resp.Candidates)
}

func v2PlanCandidatePoolForTransport(target, mode, transportID string, hint v2PlannerHint) (v2CandidatePoolResponse, error) {
	resp, err := v2BuildCandidatePoolForTransportWithHint(target, mode, transportID, hint)
	if err != nil {
		return v2CandidatePoolResponse{}, err
	}
	v2ApplyRecommendationPlanner(&resp, hint)
	return resp, nil
}
