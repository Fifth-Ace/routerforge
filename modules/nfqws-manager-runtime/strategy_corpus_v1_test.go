package main

import (
	"strings"
	"testing"
	"time"
)

func TestStaticStrategyCorpusIsSubstantialCuratedAndCompilable(t *testing.T) {
	items := v2StaticStrategyCorpus()
	if len(items) < 70 {
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
	for _, source := range []string{"curated", "omn1z", "z2k"} {
		if !sources[source] {
			t.Fatalf("expected source %s in corpus, got %v", source, sources)
		}
	}
}

func TestStaticStrategyCorpusHasNoExternalResourceDependency(t *testing.T) {
	allowedInline := []string{
		"blob=0x", "blob=tls_clienthello", "blob=http_req", "blob=fake_default_quic",
		"blob=syn_packet", "blob=empty", "seqovl_pattern=tls_clienthello",
	}
	for _, item := range v2StaticStrategyCorpus() {
		for _, arg := range item.Args {
			lower := strings.ToLower(arg)
			if strings.Contains(lower, "z2k_real_") ||
				strings.Contains(lower, "tls_clienthello_www_") ||
				strings.Contains(lower, "tls_clienthello_4pda") ||
				strings.Contains(lower, "tls_clienthello_gosuslugi") ||
				strings.Contains(lower, "tls_clienthello_vk") {
				t.Fatalf("%s leaked external z2k blob/pattern dependency: %s", item.ID, arg)
			}
			if strings.Contains(lower, "blob=") || strings.Contains(lower, "seqovl_pattern=") {
				ok := false
				for _, allowed := range allowedInline {
					if strings.Contains(lower, allowed) {
						ok = true
						break
					}
				}
				if !ok {
					t.Fatalf("%s has unapproved external dependency token: %s", item.ID, arg)
				}
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
func TestP25AUpstreamCorpusSourcesReachRegistry(t *testing.T) {
	registry := v2BuildStrategyRegistry(
		v2StrategyLibraryDocument{Version: 1, Strategies: []v2StoredStrategy{}},
		v2TargetMemoryDocument{Version: v2TargetMemoryVersion, Entries: []v2TargetMemoryEntry{}},
		time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
	)
	sourceSet := map[string]bool{}
	repositorySet := map[string]bool{}
	for _, entry := range registry.Entries {
		for _, source := range entry.Sources {
			sourceSet[source] = true
		}
		for _, provenance := range entry.Provenance {
			if provenance.Repository != "" {
				repositorySet[provenance.Repository] = true
			}
		}
	}
	for _, source := range []string{"z2k", "omn1z"} {
		if !sourceSet[source] {
			t.Fatalf("registry is missing upstream source %s; sources=%v", source, registry.Sources)
		}
	}
	for _, repository := range []string{"necronicle/z2k", "Omn1z/nfqws2-keenetic-strategy-selector"} {
		if !repositorySet[repository] {
			t.Fatalf("registry is missing upstream provenance repository %s", repository)
		}
	}
}

func TestP25AUpstreamFamiliesAreRepresented(t *testing.T) {
	families := map[string]bool{}
	for _, item := range v2StaticStrategyCorpus() {
		families[item.Family] = true
	}
	for _, family := range []string{"hostfakesplit", "syndata", "send", "fake+split", "fake+disorder"} {
		if !families[family] {
			t.Fatalf("upstream family %s is missing from static corpus", family)
		}
	}
}
