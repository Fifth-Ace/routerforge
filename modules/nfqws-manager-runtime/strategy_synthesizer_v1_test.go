package main

import (
	"strings"
	"testing"
)

func TestV2SynthesisBudgetReservesHalfOfSelectorSpace(t *testing.T) {
	cases := []struct {
		mode string
		want int
	}{
		{mode: "fast", want: 4},
		{mode: "normal", want: 8},
		{mode: "thorough", want: 16},
	}
	for _, tc := range cases {
		mode, err := v2SelectorMode(tc.mode)
		if err != nil {
			t.Fatal(err)
		}
		if got := v2SynthesisBudget(mode, mode.MaxCandidates); got != tc.want {
			t.Fatalf("mode=%s budget=%d want=%d", tc.mode, got, tc.want)
		}
		if got := v2SynthesisBudget(mode, 2); got != 2 {
			t.Fatalf("mode=%s remaining clamp=%d want=2", tc.mode, got)
		}
	}
}

func TestV2SynthesizerGeneratesUniqueCompilableCandidates(t *testing.T) {
	mode, err := v2SelectorMode("thorough")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{benchTransportHTTPS, benchTransportHTTP, benchTransportQUIC, benchTransportSTUN} {
		transport, err := normalizeBenchTransport(id)
		if err != nil {
			t.Fatal(err)
		}
		items, meta := v2SynthesizeCandidates("example.com", transport, mode, v2PlannerHint{}, 16)
		if len(items) == 0 {
			t.Fatalf("transport=%s generated no candidates", id)
		}
		if meta.Version != v2StrategySynthesizerVersion || meta.Generated < len(items) || meta.Compilable != len(items) {
			t.Fatalf("transport=%s bad meta: %+v items=%d", id, meta, len(items))
		}
		seen := map[string]bool{}
		for _, item := range items {
			if item.Source != "synthesized" {
				t.Fatalf("transport=%s source=%q", id, item.Source)
			}
			profile, err := v2CustomProfileForTransport(item.Args, "example.com", transport)
			if err != nil {
				t.Fatalf("transport=%s candidate=%s compile: %v", id, item.ID, err)
			}
			if !profile.CandidateEligible {
				t.Fatalf("transport=%s candidate=%s not eligible: %v", id, item.ID, profile.Reasons)
			}
			fp := v2CandidateTechniqueFingerprint(item.Args)
			if fp == "" || seen[fp] {
				t.Fatalf("transport=%s duplicate/empty fingerprint candidate=%s", id, item.ID)
			}
			seen[fp] = true
		}
	}
}

func TestV2HTTPSSynthesizerIsNotAStaticTinyPool(t *testing.T) {
	mode, _ := v2SelectorMode("thorough")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	items, meta := v2SynthesizeCandidates("example.com", transport, mode, v2PlannerHint{}, 24)
	if len(items) < 20 {
		t.Fatalf("HTTPS synthesized pool too small: %d", len(items))
	}
	families := map[string]bool{}
	for _, item := range items {
		families[item.Family] = true
	}
	if len(families) < 5 || len(meta.Families) < 5 {
		t.Fatalf("HTTPS synthesis lacks family diversity: items=%v meta=%v", families, meta.Families)
	}
	joined := ""
	for _, item := range items {
		joined += strings.Join(item.Args, " ") + "\n"
	}
	for _, want := range []string{"seqovl=336", "fakedsplit", "fake:blob="} {
		if !strings.Contains(joined, want) {
			t.Fatalf("HTTPS synthesis missing generated axis %q", want)
		}
	}
}

func TestV2SynthesisDiagnosticCanChangeSearchOrder(t *testing.T) {
	mode, _ := v2SelectorMode("normal")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	items, meta := v2SynthesizeCandidates("example.com", transport, mode, v2PlannerHint{
		DiagnosticCode: "TCP_CONNECT_RESET", StrategyRelevant: true,
	}, 8)
	if len(items) == 0 {
		t.Fatal("no synthesized candidates")
	}
	if items[0].Family != "disorder" {
		t.Fatalf("first family=%q want disorder", items[0].Family)
	}
	if !strings.Contains(meta.Basis, "TCP_CONNECT_RESET") {
		t.Fatalf("basis=%q", meta.Basis)
	}
}

func TestV2QUICSynthesizerExpandsBeyondLegacyRepeatPool(t *testing.T) {
	mode, _ := v2SelectorMode("thorough")
	transport, _ := normalizeBenchTransport(benchTransportQUIC)
	items, _ := v2SynthesizeCandidates("example.com", transport, mode, v2PlannerHint{}, 20)
	joined := ""
	for _, item := range items {
		joined += strings.Join(item.Args, " ") + "\n"
	}
	if !strings.Contains(joined, "repeats=11") {
		t.Fatalf("QUIC synthesis did not expand repeat axis: %s", joined)
	}
	if !strings.Contains(joined, "ip_ttl=3") {
		t.Fatalf("QUIC synthesis did not expand TTL axis: %s", joined)
	}
}

func TestV2StrategySourcePreservesSynthesizedOrigin(t *testing.T) {
	if got := v2StrategySource("synthesized"); got != "synthesized" {
		t.Fatalf("source=%q want synthesized", got)
	}
}
