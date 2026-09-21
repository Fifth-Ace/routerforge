package main

import (
	"strings"
	"testing"
	"time"
)

func TestStaticStrategyCorpusIsSubstantialCuratedAndCompilable(t *testing.T) {
	items := v2StaticStrategyCorpus()
	if len(items) < 24 {
		t.Fatalf("strategy corpus unexpectedly small: %d", len(items))
	}
	ids := map[string]bool{}
	sources := map[string]bool{}
	for _, item := range items {
		if item.ID == "" || ids[item.ID] {
			t.Fatalf("duplicate/empty corpus id: %q", item.ID)
		}
		ids[item.ID] = true
		sources[item.Source] = true
		if strings.Contains(strings.Join(item.Args, " "), "--hostlist") ||
			strings.Contains(strings.Join(item.Args, " "), "--ipset") {
			t.Fatalf("selection-only args leaked into static corpus candidate %s", item.ID)
		}
		transport, err := normalizeBenchTransport(item.Protocol)
		if err != nil {
			t.Fatalf("%s transport: %v", item.ID, err)
		}
		if _, err := v2CustomProfileForTransport(item.Args, "example.com", transport); err != nil {
			t.Fatalf("%s does not compile through RouterForge candidate compiler: %v", item.ID, err)
		}
	}
	if !sources["curated"] {
		t.Fatalf("expected RouterForge curated source, got %v", sources)
	}
}

func TestStaticStrategyCorpusHasNoExternalResourceDependency(t *testing.T) {
	for _, item := range v2StaticStrategyCorpus() {
		for _, arg := range item.Args {
			if strings.Contains(arg, "blob=") || strings.Contains(arg, "seqovl_pattern=") {
				t.Fatalf("%s unexpectedly requires an external blob/pattern: %s", item.ID, arg)
			}
		}
	}
}

func TestStrategyRegistryMergesCuratedCorpusWithBuiltinTechnique(t *testing.T) {
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	registry := v2BuildStrategyRegistry(
		v2StrategyLibraryDocument{Version: 1, Strategies: []v2StoredStrategy{}},
		v2TargetMemoryDocument{Version: v2TargetMemoryVersion, Entries: []v2TargetMemoryEntry{}},
		now,
	)
	fp := v2CandidateTechniqueFingerprint([]string{
		"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	})
	var found *v2StrategyRegistryEntry
	for i := range registry.Entries {
		if registry.Entries[i].Fingerprint == fp {
			found = &registry.Entries[i]
			break
		}
	}
	if found == nil {
		t.Fatal("merged builtin/corpus technique not found")
	}
	hasBuiltin, hasCurated := false, false
	for _, source := range found.Sources {
		hasBuiltin = hasBuiltin || source == "builtin"
		hasCurated = hasCurated || source == "curated"
	}
	if !hasBuiltin || !hasCurated {
		t.Fatalf("merged sources incomplete: %v", found.Sources)
	}
}

func TestCorpusSourcePreferencePreservesCuratedSource(t *testing.T) {
	entry := v2StrategyRegistryEntry{Sources: []string{"memory", "builtin", "curated"}}
	if got := v2RecommendationSource(entry); got != "curated" {
		t.Fatalf("source=%q want curated", got)
	}
}
