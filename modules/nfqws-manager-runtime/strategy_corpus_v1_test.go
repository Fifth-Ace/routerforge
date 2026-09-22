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
	for _, source := range []string{"curated", "other"} {
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
			if strings.Contains(lower, "tls_clienthello_www_") ||
				strings.Contains(lower, "tls_clienthello_4pda") ||
				strings.Contains(lower, "tls_clienthello_gosuslugi") ||
				strings.Contains(lower, "tls_clienthello_vk") {
				t.Fatalf("%s leaked external resource dependency: %s", item.ID, arg)
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
func TestOtherStrategyCorpusSourceReachesRegistry(t *testing.T) {
	registry := v2BuildStrategyRegistry(
		v2StrategyLibraryDocument{Version: 1, Strategies: []v2StoredStrategy{}},
		v2TargetMemoryDocument{Version: v2TargetMemoryVersion, Entries: []v2TargetMemoryEntry{}},
		time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
	)
	sourceSet := map[string]bool{}
	hasOtherProvenance := false
	for _, entry := range registry.Entries {
		for _, source := range entry.Sources {
			sourceSet[source] = true
		}
		for _, provenance := range entry.Provenance {
			if provenance.Source == "other" && provenance.Kind == "other-strategy-corpus" {
				hasOtherProvenance = true
			}
			if provenance.Repository != "" && provenance.Source == "other" {
				t.Fatalf("other strategy provenance must stay neutral, got repository %q", provenance.Repository)
			}
		}
	}
	if !sourceSet["other"] {
		t.Fatalf("registry is missing other strategy source; sources=%v", registry.Sources)
	}
	if !hasOtherProvenance {
		t.Fatalf("registry is missing neutral other-strategy provenance")
	}
}

func TestP25AUpstreamFamiliesAreRepresented(t *testing.T) {
	families := map[string]bool{}
	for _, item := range v2StaticStrategyCorpus() {
		families[item.Family] = true
	}
	for _, family := range []string{"hostfakesplit", "syndata", "send", "fake+split", "fake+disorder"} {
		if !families[family] {
			t.Fatalf("external family %s is missing from static corpus", family)
		}
	}
}

func TestP25BFinalCorpusIsModuleReadyForDomainIPv4Selection(t *testing.T) {
	https, err := normalizeBenchTransport(benchTransportHTTPS)
	if err != nil {
		t.Fatal(err)
	}
	items := v2CorpusCandidatesForTransport(https)
	if len(items) < 100 {
		t.Fatalf("https corpus too small for final domain/IPv4 selector pool: %d", len(items))
	}

	sources := map[string]bool{}
	families := map[string]bool{}
	for _, item := range items {
		if item.Protocol != benchTransportHTTPS {
			t.Fatalf("unexpected non-HTTPS item in HTTPS corpus: %s protocol=%s", item.ID, item.Protocol)
		}
		sources[item.Source] = true
		families[item.Family] = true
	}
	for _, source := range []string{"curated", "other"} {
		if !sources[source] {
			t.Fatalf("module corpus missing source %s; sources=%v", source, sources)
		}
	}
	for _, family := range []string{
		"hostfakesplit", "fake+split", "fake+disorder", "fake-split", "fake-disorder",
		"split", "disorder", "send", "syndata",
	} {
		if !families[family] {
			t.Fatalf("module corpus missing family %s; families=%v", family, families)
		}
	}

	first := 32
	if len(items) < first {
		first = len(items)
	}
	earlySources := map[string]bool{}
	earlyFamilies := map[string]bool{}
	for _, item := range items[:first] {
		earlySources[item.Source] = true
		earlyFamilies[item.Family] = true
	}
	if !earlySources["other"] {
		t.Fatalf("early module corpus window missing other strategies; sources=%v", earlySources)
	}
	for _, family := range []string{"hostfakesplit", "fake+split", "fake+disorder", "fake-split"} {
		if !earlyFamilies[family] {
			t.Fatalf("early module corpus window missing family %s; families=%v", family, earlyFamilies)
		}
	}
}
