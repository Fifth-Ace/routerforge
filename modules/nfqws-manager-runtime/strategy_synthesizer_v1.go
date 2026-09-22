package main

import (
	"fmt"
	"sort"
	"strings"
)

const v2StrategySynthesizerVersion = 1

type v2SynthesisMeta struct {
	Version    int
	Generated  int
	Compilable int
	Families   []string
	Basis      string
}

type v2SynthesisBuilder struct {
	target    string
	transport benchTransportProfile
	mode      benchAutoTuneMode
	limit     int
	hint      v2PlannerHint
	items     []v2CandidatePoolItem
	seen      map[string]bool
	families  map[string]bool
	generated int
}

func v2SynthesisBudget(mode benchAutoTuneMode, remaining int) int {
	if remaining <= 0 {
		return 0
	}
	budget := 8
	switch mode.Name {
	case "fast":
		budget = 4
	case "thorough":
		budget = 16
	}
	if budget > remaining {
		budget = remaining
	}
	return budget
}

func v2SynthesisBaseArgs(transport benchTransportProfile) []string {
	switch transport.ID {
	case benchTransportHTTPS:
		return []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello"}
	case benchTransportHTTP:
		return []string{"--filter-tcp=80", "--filter-l7=http", "--payload=http_req"}
	case benchTransportQUIC:
		return []string{"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial"}
	case benchTransportSTUN:
		return []string{"--filter-udp=3478", "--filter-l7=stun", "--payload=stun"}
	default:
		return nil
	}
}

func (b *v2SynthesisBuilder) add(name, family string, actions ...string) {
	b.generated++
	if len(b.items) >= b.limit || len(actions) == 0 {
		return
	}
	args := append([]string{}, v2SynthesisBaseArgs(b.transport)...)
	args = append(args, actions...)
	if len(args) == len(actions) {
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
	b.families[family] = true
	stage := v2ProgressiveStageFull
	if len(b.items) < v2ProgressiveQuickLimit(b.mode) {
		stage = v2ProgressiveStageQuick
	}
	b.items = append(b.items, v2CandidatePoolItem{
		ID:   fmt.Sprintf("synth-v%d-%s-%02d", v2StrategySynthesizerVersion, b.transport.ID, len(b.items)+1),
		Name: "Synth · " + name, Source: "synthesized", Family: family,
		Protocol: b.transport.ID, Args: args, Stage: stage,
	})
}

func v2SynthesisDiagnosticBasis(hint v2PlannerHint) string {
	code := strings.TrimSpace(hint.DiagnosticCode)
	if code == "" {
		return "transport grammar + live verification"
	}
	if hint.StrategyRelevant {
		return "diagnostic " + code + " + transport grammar + live verification"
	}
	return "transport grammar + live verification; diagnostic " + code + " is observe-only"
}

func v2SynthesisHTTPS(b *v2SynthesisBuilder) {
	code := strings.ToUpper(strings.TrimSpace(b.hint.DiagnosticCode))
	disorderFirst := code == "TCP_CONNECT_RESET"

	positions := []struct {
		label string
		value string
	}{
		{"midsld", "1,midsld"},
		{"sniext+1", "1,sniext+1"},
		{"host+1", "1,host+1"},
		{"midsld-2", "2,midsld-2"},
		{"sld+1", "sld+1"},
		{"multi-position", "1,sniext+1,host+1,midsld"},
	}
	methods := []struct {
		method string
		family string
	}{
		{"multisplit", "split"},
		{"multidisorder", "disorder"},
	}
	if disorderFirst {
		methods[0], methods[1] = methods[1], methods[0]
	}
	for _, pos := range positions {
		for _, method := range methods {
			b.add(method.method+" "+pos.label, method.family,
				"--lua-desync="+method.method+":pos="+pos.value)
		}
	}

	for _, method := range []struct {
		method string
		family string
	}{
		{"fakedsplit", "fake-split"},
		{"fakeddisorder", "fake-disorder"},
	} {
		for _, pos := range []string{"midsld", "sniext+1"} {
			b.add(method.method+" "+pos, method.family,
				"--lua-desync="+method.method+":pos="+pos+":seqovl=1:seqovl_pattern=tls_clienthello")
		}
	}

	for _, ovl := range []int{1, 336, 681} {
		b.add(fmt.Sprintf("multisplit overlap %d", ovl), "split",
			fmt.Sprintf("--lua-desync=multisplit:pos=1,midsld:seqovl=%d:seqovl_pattern=tls_clienthello", ovl))
		b.add(fmt.Sprintf("multidisorder overlap %d", ovl), "disorder",
			fmt.Sprintf("--lua-desync=multidisorder:pos=1,midsld:seqovl=%d:seqovl_pattern=tls_clienthello", ovl))
	}

	fakes := []struct {
		label string
		arm   string
	}{
		{"badsum", "--lua-desync=fake:blob=tls_clienthello:badsum"},
		{"tcp_md5", "--lua-desync=fake:blob=tls_clienthello:tcp_md5"},
		{"ack/repeats", "--lua-desync=fake:blob=0x00000000:tcp_ack=-66000:repeats=2"},
		{"tcp_ts", "--lua-desync=fake:blob=tls_clienthello:tcp_ts_up"},
		{"ttl4", "--lua-desync=fake:blob=tls_clienthello:ip_ttl=4"},
		{"autottl", "--lua-desync=fake:blob=tls_clienthello:ip_autottl=0,3-20"},
	}
	for _, fake := range fakes {
		b.add("fake "+fake.label+" + multisplit", "fake+split", fake.arm,
			"--lua-desync=multisplit:pos=1,midsld:seqovl=1:seqovl_pattern=tls_clienthello")
		b.add("fake "+fake.label+" + multidisorder", "fake+disorder", fake.arm,
			"--lua-desync=multidisorder:pos=1,midsld")
	}
}

func v2SynthesisHTTP(b *v2SynthesisBuilder) {
	positions := []string{"method+2", "host+1", "method+2,host+1"}
	for _, pos := range positions {
		b.add("HTTP multisplit "+pos, "split", "--lua-desync=multisplit:pos="+pos)
		b.add("HTTP multidisorder "+pos, "disorder", "--lua-desync=multidisorder:pos="+pos)
	}
	b.add("HTTP methodeol", "http-method", "--lua-desync=http_methodeol")
	b.add("HTTP methodeol badsum", "http-method", "--lua-desync=http_methodeol:badsum")
	for _, method := range []string{"multisplit", "multidisorder"} {
		family := "fake+split"
		if method == "multidisorder" {
			family = "fake+disorder"
		}
		b.add("HTTP fake + "+method, family,
			"--lua-desync=fake:blob=http_req",
			"--lua-desync="+method+":pos=method+2,host+1")
	}
}

func v2SynthesisQUIC(b *v2SynthesisBuilder) {
	for _, blob := range []string{"fake_default_quic", "0x00000000000000000000000000000000"} {
		for _, repeats := range []int{1, 2, 3, 5, 6, 11} {
			b.add(fmt.Sprintf("QUIC fake %s x%d", blob, repeats), "fake",
				fmt.Sprintf("--lua-desync=fake:blob=%s:repeats=%d", blob, repeats))
		}
	}
	for _, ttl := range []int{3, 5, 8, 12} {
		b.add(fmt.Sprintf("QUIC fake TTL %d", ttl), "fake",
			fmt.Sprintf("--lua-desync=fake:blob=fake_default_quic:repeats=2:ip_ttl=%d", ttl))
	}
	for _, inc := range []int{4, 8, 25, 100} {
		b.add(fmt.Sprintf("QUIC udplen +%d", inc), "udplen",
			fmt.Sprintf("--lua-desync=udplen:payload=quic_initial:dir=out:increment=%d", inc))
	}
}

func v2SynthesisSTUN(b *v2SynthesisBuilder) {
	const blob = "0x00000000000000000000000000000000"
	for _, repeats := range []int{1, 2, 3, 5, 6, 11} {
		b.add(fmt.Sprintf("STUN fake x%d", repeats), "fake",
			fmt.Sprintf("--lua-desync=fake:blob=%s:repeats=%d", blob, repeats))
	}
	for _, ttl := range []int{3, 5, 8, 12} {
		b.add(fmt.Sprintf("STUN fake TTL %d", ttl), "fake",
			fmt.Sprintf("--lua-desync=fake:blob=%s:repeats=2:ip_ttl=%d", blob, ttl))
	}
}

func v2SynthesizeCandidates(target string, transport benchTransportProfile, mode benchAutoTuneMode, hint v2PlannerHint, limit int) ([]v2CandidatePoolItem, v2SynthesisMeta) {
	meta := v2SynthesisMeta{Version: v2StrategySynthesizerVersion, Basis: v2SynthesisDiagnosticBasis(hint)}
	if limit <= 0 {
		return []v2CandidatePoolItem{}, meta
	}
	b := &v2SynthesisBuilder{
		target: target, transport: transport, mode: mode, limit: limit, hint: hint,
		items: []v2CandidatePoolItem{}, seen: map[string]bool{}, families: map[string]bool{},
	}
	switch transport.ID {
	case benchTransportHTTPS:
		v2SynthesisHTTPS(b)
	case benchTransportHTTP:
		v2SynthesisHTTP(b)
	case benchTransportQUIC:
		v2SynthesisQUIC(b)
	case benchTransportSTUN:
		v2SynthesisSTUN(b)
	}
	meta.Generated = b.generated
	meta.Compilable = len(b.items)
	for family := range b.families {
		meta.Families = append(meta.Families, family)
	}
	sort.Strings(meta.Families)
	return append([]v2CandidatePoolItem{}, b.items...), meta
}
