package main

import (
	"strings"
	"testing"
)

func TestAppSourceRecordIDValidation(t *testing.T) {
	valid := appSourceID("https://example.com/routerforge.json")
	if !validAppSourceRecordID(valid) {
		t.Fatalf("generated source id %q rejected", valid)
	}
	for _, id := range []string{
		"",
		"src-",
		"src-123",
		"src-123456789abc0",
		"src-123456789abz",
		"routerforge-official",
		"src-123456789ABC",
		"src-../../escape",
	} {
		if validAppSourceRecordID(id) {
			t.Fatalf("invalid source id accepted: %q", id)
		}
	}
}

func TestUserSourceCannotOverridePublicManifestID(t *testing.T) {
	source := appSourceRecord{
		ID:    "src-123456789abc",
		Local: false,
	}
	cache := appSourceCache{
		ManifestSHA256: strings.Repeat("a", 64),
		ResolvedURL:    "https://example.com/routerforge.json",
	}
	item := normalizeUserSourceItem(source, cache, catalogItem{
		ID:   "dns",
		Kind: "integration",
		Name: "Collision Attempt",
	})

	if item.ID != "src-123456789abc:dns" {
		t.Fatalf("user source was not namespaced: %q", item.ID)
	}
	if item.ManifestID != "dns" {
		t.Fatalf("original manifest id not preserved: %q", item.ManifestID)
	}
	if item.ID == item.ManifestID {
		t.Fatal("user source silently retained public manifest id")
	}
	if item.RegistrySource != source.ID || item.Source != "user-source" {
		t.Fatalf("source provenance missing: %#v", item)
	}
	if item.Trust.Status != "unverified" {
		t.Fatalf("trust=%q, want unverified", item.Trust.Status)
	}
}

func TestLocalPrivateSourceGetsExplicitLowerTrustNote(t *testing.T) {
	source := appSourceRecord{
		ID:    "src-123456789abc",
		Local: true,
	}
	cache := appSourceCache{
		ManifestSHA256: strings.Repeat("b", 64),
		ResolvedURL:    "https://192.168.1.10/index.json",
	}
	item := normalizeUserSourceItem(source, cache, catalogItem{
		ID:   "demo",
		Kind: "integration",
		Name: "Demo",
	})

	if item.Trust.Status != "unverified" {
		t.Fatalf("local trust=%q, want unverified", item.Trust.Status)
	}
	note := strings.ToLower(item.Trust.Note)
	if !strings.Contains(note, "local/private") {
		t.Fatalf("local/private trust note missing: %q", item.Trust.Note)
	}
	if !strings.Contains(note, "explicit") {
		t.Fatalf("explicit permission warning missing: %q", item.Trust.Note)
	}
}

func TestMalformedConfiguredSourceCannotEnterCatalog(t *testing.T) {
	withTempAppSources(t)

	cache := appSourceCache{
		SchemaVersion:  appSourcesSchemaVersion,
		SourceID:       "src-123456789abc",
		Kind:           "app",
		RegistryID:     "single",
		Name:           "Demo",
		ResolvedURL:    "https://example.com/routerforge.json",
		ManifestSHA256: strings.Repeat("c", 64),
		Entries: []catalogItem{{
			ID:        "demo",
			Kind:      "integration",
			Name:      "Demo",
			Publisher: catalogPublisher{Name: "Example"},
		}},
	}
	if err := saveAppSourceCache(cache); err != nil {
		t.Fatal(err)
	}

	appSourcesMu.Lock()
	cfg := defaultAppSourcesConfig()
	cfg.Sources = []appSourceRecord{{
		ID:      "src-../../escape",
		Kind:    "app",
		Name:    "Malformed",
		URL:     "https://example.com/routerforge.json",
		Enabled: true,
	}}
	if err := saveAppSourcesConfigUnlocked(cfg); err != nil {
		appSourcesMu.Unlock()
		t.Fatal(err)
	}
	appSourcesMu.Unlock()

	snapshot := catalogSnapshot{}
	applyUserAppSources(&snapshot, nil, nil, func(string) bool { return false })
	if len(snapshot.Integrations) != 0 {
		t.Fatalf("malformed configured source entered catalog: %#v", snapshot.Integrations)
	}
}
