package main

import (
	"os"
	"strings"
	"testing"
)

func TestUserSourceWebProbeIsAlwaysExplicitlyBlocked(t *testing.T) {
	item := catalogItem{
		ID:             "src-123456789abc:demo",
		Kind:           "integration",
		Name:           "Demo",
		Installed:      true,
		RegistrySource: "src-123456789abc",
		Trust:          catalogTrust{Status: "verified"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   8080,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
	}

	if catalogWebProbeAllowed(item) {
		t.Fatal("user-added source unexpectedly gained active web-probe authority")
	}
}

func TestUserSourceDetectionIsReadOnlyAgainstSourceState(t *testing.T) {
	withTempAppSources(t)

	sourceID := "src-123456789abc"
	cache := appSourceCache{
		SchemaVersion:  appSourcesSchemaVersion,
		SourceID:       sourceID,
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
			Detection: catalogDetection{
				Packages: []string{"demo-pkg"},
				Services: []string{"/opt/etc/init.d/S99demo"},
				Paths:    []string{"/opt/bin/demo"},
			},
		}},
	}
	if err := saveAppSourceCache(cache); err != nil {
		t.Fatal(err)
	}

	appSourcesMu.Lock()
	cfg := defaultAppSourcesConfig()
	cfg.Sources = []appSourceRecord{{
		ID:          sourceID,
		Kind:        "app",
		Name:        "Demo",
		URL:         "https://example.com/routerforge.json",
		ResolvedURL: cache.ResolvedURL,
		Trust:       "unsigned",
		Enabled:     true,
		Cached:      true,
		EntryCount:  1,
	}}
	if err := saveAppSourcesConfigUnlocked(cfg); err != nil {
		appSourcesMu.Unlock()
		t.Fatal(err)
	}
	appSourcesMu.Unlock()

	configBefore, err := os.ReadFile(appSourcesConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	cacheBefore, err := os.ReadFile(appSourceCachePath(sourceID))
	if err != nil {
		t.Fatal(err)
	}

	snapshot := catalogSnapshot{}
	applyUserAppSources(
		&snapshot,
		map[string]string{"demo-pkg": "1.2.3"},
		map[string]bool{},
		func(path string) bool { return path == "/opt/etc/init.d/S99demo" || path == "/opt/bin/demo" },
	)

	if len(snapshot.Integrations) != 1 {
		t.Fatalf("integrations=%d, want 1", len(snapshot.Integrations))
	}
	if !snapshot.Integrations[0].Installed {
		t.Fatal("passive detection did not observe installed state")
	}

	configAfter, err := os.ReadFile(appSourcesConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	cacheAfter, err := os.ReadFile(appSourceCachePath(sourceID))
	if err != nil {
		t.Fatal(err)
	}
	if string(configAfter) != string(configBefore) {
		t.Fatal("passive detection mutated app source config")
	}
	if string(cacheAfter) != string(cacheBefore) {
		t.Fatal("passive detection mutated app source cache")
	}
}

func TestUserSourceWebMetadataDoesNotGrantLifecycleAuthority(t *testing.T) {
	item := catalogItem{
		ID:             "src-123456789abc:demo",
		Kind:           "integration",
		RegistrySource: "src-123456789abc",
		Trust:          catalogTrust{Status: "unverified"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   8080,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
	}
	actions := deriveCatalogActions(item)
	if actions.Install || actions.Update || actions.Remove {
		t.Fatalf("web metadata unexpectedly granted lifecycle authority: %#v", actions)
	}
}
