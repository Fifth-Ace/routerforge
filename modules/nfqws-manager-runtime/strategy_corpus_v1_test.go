package main

import (
	"strings"
	"testing"
	"time"
)

func TestStaticStrategyCorpusIsSubstantialPinnedAndCompilable(t *testing.T) {
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
		if item.Repository == "" || item.Ref == "" || item.UpstreamName == "" {
			t.Fatalf("missing provenance for %s: %+v", item.ID, item)
		}
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
	if !sources["omn1z"] || !sources["z2k"] {
		t.Fatalf("expected both upstream sources, got %v", sources)
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

func TestStrategyRegistryMergesCorpusProvenanceWithBuiltinTechnique(t *testing.T) {
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
	hasBuiltin, hasOmn1z := false, false
	hasPinnedOmn1z := false
	for _, source := range found.Sources {
		hasBuiltin = hasBuiltin || source == "builtin"
		hasOmn1z = hasOmn1z || source == "omn1z"
	}
	for _, p := range found.Provenance {
		if p.Source == "omn1z" && p.Repository == v2CorpusOmn1zRepository && p.Ref == v2CorpusOmn1zRef {
			hasPinnedOmn1z = true
		}
	}
	if !hasBuiltin || !hasOmn1z || !hasPinnedOmn1z {
		t.Fatalf("merged provenance incomplete: sources=%v provenance=%+v", found.Sources, found.Provenance)
	}
}

func TestCorpusSourcePreferencePreservesConcreteUpstream(t *testing.T) {
	entry := v2StrategyRegistryEntry{Sources: []string{"memory", "builtin", "omn1z"}}
	if got := v2RecommendationSource(entry); got != "omn1z" {
		t.Fatalf("source=%q want omn1z", got)
	}
}
