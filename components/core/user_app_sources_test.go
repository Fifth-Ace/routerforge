package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withTempAppSources(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	oldConfig := appSourcesConfigPath
	oldCache := appSourcesCacheDir
	appSourcesConfigPath = filepath.Join(dir, "etc", "app-sources.json")
	appSourcesCacheDir = filepath.Join(dir, "cache")
	t.Cleanup(func() {
		appSourcesConfigPath = oldConfig
		appSourcesCacheDir = oldCache
	})
	return dir
}

func TestAppSourceURLPolicyRejectsUnsafeTargets(t *testing.T) {
	cases := []string{
		"http://example.com/index.json",
		"https://localhost/index.json",
		"https://127.0.0.1/index.json",
		"https://10.0.0.1/index.json",
		"https://169.254.169.254/latest/meta-data",
		"https://user:pass@example.com/index.json",
	}
	for _, raw := range cases {
		if _, err := validateAppSourceURL(raw); err == nil {
			t.Fatalf("validateAppSourceURL(%q) succeeded, want rejection", raw)
		}
	}
	if _, err := validateAppSourceURL("https://example.com/routerforge/index.json"); err != nil {
		t.Fatalf("public HTTPS source rejected: %v", err)
	}
}

func TestParseThirdPartyRepository(t *testing.T) {
	raw := []byte(`{
	  "schema_version":1,
	  "registry_id":"example-apps",
	  "name":"Example Apps",
	  "revision":"1",
	  "entries":[{
	    "id":"demo-app",
	    "kind":"integration",
	    "name":"Demo App",
	    "category":"Utilities",
	    "description":"Demo",
	    "publisher":{"id":"example","name":"Example"},
	    "detection":{"packages":["demo-app"]},
	    "install":{"method":"opkg","packages":["demo-app"]},
	    "update":{"method":"opkg","packages":["demo-app"]},
	    "remove":{"method":"opkg","packages":["demo-app"]}
	  }]
	}`)
	cache, err := parseThirdPartySourceDocument(raw, "https://example.com/index.json", "repository")
	if err != nil {
		t.Fatalf("parse repository: %v", err)
	}
	if cache.Kind != "repository" || cache.RegistryID != "example-apps" || len(cache.Entries) != 1 {
		t.Fatalf("unexpected cache: %#v", cache)
	}
	if cache.Entries[0].Trust.Status != "unverified" {
		t.Fatalf("trust=%q, want unverified", cache.Entries[0].Trust.Status)
	}
	if len(cache.ManifestSHA256) != 64 {
		t.Fatalf("fingerprint length=%d, want 64", len(cache.ManifestSHA256))
	}
}

func TestThirdPartyRegistryCannotClaimRouterForgeNamespace(t *testing.T) {
	raw := []byte(`{
	  "schema_version":1,
	  "registry_id":"routerforge-community",
	  "entries":[{
	    "id":"evil",
	    "kind":"integration",
	    "name":"Evil",
	    "publisher":{"name":"Other"}
	  }]
	}`)
	if _, err := parseThirdPartySourceDocument(raw, "https://example.com/index.json", "repository"); err == nil {
		t.Fatal("reserved registry id accepted")
	}
}

func TestThirdPartySourceBlocksStructuredLifecycle(t *testing.T) {
	raw := []byte(`{
	  "schema_version":1,
	  "app":{
	    "id":"demo",
	    "kind":"integration",
	    "name":"Demo",
	    "publisher":{"name":"Example"},
	    "install":{"method":"structured","steps":[{"type":"opkg-install","packages":["demo"]}]}
	  }
	}`)
	if _, err := parseThirdPartySourceDocument(raw, "https://example.com/routerforge.json", "app"); err == nil {
		t.Fatal("structured lifecycle from unverified source accepted")
	}
}

func TestAppSourceConfigAndUnsafeGate(t *testing.T) {
	withTempAppSources(t)
	cfg, err := loadAppSourcesConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AllowUnverified {
		t.Fatal("unverified installs enabled by default")
	}

	if _, err := setAppSourceSecurity(appSourceSecurityRequest{
		AllowUnverified:  true,
		Accepted:         false,
		AgreementVersion: appSourcesAgreementVersion,
	}); err == nil {
		t.Fatal("unsafe mode enabled without acceptance")
	}

	cfg, err = setAppSourceSecurity(appSourceSecurityRequest{
		AllowUnverified:  true,
		Accepted:         true,
		AgreementVersion: appSourcesAgreementVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AllowUnverified || cfg.AgreementAccepted == "" {
		t.Fatalf("unsafe gate not persisted: %#v", cfg)
	}

	item := catalogItem{
		ID:             "src-demo:app",
		RegistrySource: "src-demo",
		Trust:          catalogTrust{Status: "unverified"},
	}
	if reason := appSourceActionBlockReason(item, "install", ""); !strings.Contains(reason, "confirmation") {
		t.Fatalf("missing confirmation reason=%q", reason)
	}
	if reason := appSourceActionBlockReason(item, "install", appSourceRiskConfirm); reason != "" {
		t.Fatalf("confirmed unsafe action blocked: %q", reason)
	}
	if reason := appSourceActionBlockReason(item, "remove", ""); reason != "" {
		t.Fatalf("remove should remain possible with source provenance: %q", reason)
	}
}

func TestUserSourceCacheAndNamespace(t *testing.T) {
	withTempAppSources(t)
	cache := appSourceCache{
		SchemaVersion:  appSourcesSchemaVersion,
		SourceID:       "src-123456789abc",
		Kind:           "app",
		RegistryID:     "single",
		Name:           "Demo",
		ResolvedURL:    "https://example.com/routerforge.json",
		ManifestSHA256: strings.Repeat("a", 64),
		Entries: []catalogItem{{
			ID:        "demo",
			Kind:      "integration",
			Name:      "Demo",
			Publisher: catalogPublisher{Name: "Example"},
			Detection: catalogDetection{Packages: []string{"demo"}},
			Trust:     catalogTrust{Status: "unverified"},
		}},
	}
	if err := saveAppSourceCache(cache); err != nil {
		t.Fatal(err)
	}
	appSourcesMu.Lock()
	cfg := defaultAppSourcesConfig()
	cfg.Sources = []appSourceRecord{{
		ID:          cache.SourceID,
		Kind:        "app",
		Name:        "Demo",
		URL:         "https://example.com/routerforge.json",
		ResolvedURL: cache.ResolvedURL,
		Trust:       "unsigned",
		Enabled:     true,
	}}
	if err := saveAppSourcesConfigUnlocked(cfg); err != nil {
		appSourcesMu.Unlock()
		t.Fatal(err)
	}
	appSourcesMu.Unlock()

	snapshot := catalogSnapshot{}
	applyUserAppSources(&snapshot, map[string]string{}, map[string]bool{}, func(string) bool { return false })
	if len(snapshot.Integrations) != 1 {
		t.Fatalf("integrations=%d, want 1", len(snapshot.Integrations))
	}
	item := snapshot.Integrations[0]
	if item.ID != cache.SourceID+":demo" || item.ManifestID != "demo" {
		t.Fatalf("namespace not applied: id=%q manifest=%q", item.ID, item.ManifestID)
	}
	if item.RegistrySource != cache.SourceID || item.Source != "user-source" {
		t.Fatalf("source provenance missing: %#v", item)
	}
}

func TestRemoveSourceKeepsConfigConsistent(t *testing.T) {
	withTempAppSources(t)
	id := "src-123456789abc"
	if err := os.MkdirAll(appSourcesCacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appSourceCachePath(id), []byte(`{"schema_version":1,"source_id":"src-123456789abc"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	appSourcesMu.Lock()
	cfg := defaultAppSourcesConfig()
	cfg.Sources = []appSourceRecord{{ID: id, URL: "https://example.com/index.json", Enabled: true}}
	if err := saveAppSourcesConfigUnlocked(cfg); err != nil {
		appSourcesMu.Unlock()
		t.Fatal(err)
	}
	appSourcesMu.Unlock()

	if err := removeAppSource(id); err != nil {
		t.Fatal(err)
	}
	next, err := loadAppSourcesConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Sources) != 0 {
		t.Fatalf("sources remain after removal: %#v", next.Sources)
	}
	if _, err := os.Stat(appSourceCachePath(id)); !os.IsNotExist(err) {
		t.Fatalf("cache still exists: %v", err)
	}
}

func TestPreviewResolverCanBeStubbed(t *testing.T) {
	old := appSourceResolve
	defer func() { appSourceResolve = old }()
	appSourceResolve = func(ctx context.Context, rawURL, kind string) (appSourceCache, error) {
		return appSourceCache{
			SchemaVersion:  appSourcesSchemaVersion,
			Kind:           "app",
			RegistryID:     "single",
			Name:           "Stub",
			ResolvedURL:    rawURL,
			ManifestSHA256: strings.Repeat("b", 64),
			Entries: []catalogItem{{
				ID:        "stub",
				Kind:      "integration",
				Name:      "Stub",
				Publisher: catalogPublisher{Name: "Test"},
			}},
		}, nil
	}
	cache, err := appSourceResolve(context.Background(), "https://example.com/routerforge.json", "app")
	if err != nil || cache.Name != "Stub" {
		t.Fatalf("stub resolver failed: cache=%#v err=%v", cache, err)
	}
}
