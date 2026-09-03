package main

import (
	"encoding/json"
	"testing"
)

func TestParseRouterForgeReleaseIndex(t *testing.T) {
	old := releaseChannel
	releaseChannel = "beta"
	defer func() { releaseChannel = old }()

	doc := routerForgeReleaseIndex{
		SchemaVersion: 1,
		Channel:       "beta",
		Components: []catalogRelease{
			{
				Package: "routerforge-network",
				Version: "0.5.1-beta",
				Asset:   "routerforge-network_0.5.1-beta_aarch64-3.10.ipk",
				SHA256:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				URL:     "https://github.com/Fifth-Ace/dns-monitor/releases/download/routerforge-beta/routerforge-network_0.5.1-beta_aarch64-3.10.ipk",
			},
		},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseRouterForgeReleaseIndex(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Components[0].Version != "0.5.1-beta" {
		t.Fatalf("unexpected release: %#v", parsed.Components[0])
	}
}

func TestParseRouterForgeReleaseIndexAcceptsRenamedRepository(t *testing.T) {
	old := releaseChannel
	releaseChannel = "beta"
	defer func() { releaseChannel = old }()

	doc := routerForgeReleaseIndex{
		SchemaVersion: 1,
		Channel:       "beta",
		Components: []catalogRelease{
			{
				Package:      "routerforge-core",
				Version:      "0.3.2-beta",
				Asset:        "routerforge-core_0.3.2-beta_aarch64-3.10.ipk",
				SHA256:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				URL:          "https://github.com/Fifth-Ace/dns-monitor/releases/download/routerforge-beta/routerforge-core_0.3.2-beta_aarch64-3.10.ipk",
				CanonicalURL: "https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-beta/routerforge-core_0.3.2-beta_aarch64-3.10.ipk",
			},
		},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseRouterForgeReleaseIndex(data); err != nil {
		t.Fatal(err)
	}

	urls := routerForgeReleaseDownloadURLs(doc.Components[0])
	if len(urls) != 2 {
		t.Fatalf("unexpected download URL candidates: %#v", urls)
	}
	if urls[0] != doc.Components[0].CanonicalURL || urls[1] != doc.Components[0].URL {
		t.Fatalf("canonical repository is not preferred: %#v", urls)
	}
}

func TestParseRouterForgeReleaseIndexRejectsForeignRepository(t *testing.T) {
	old := releaseChannel
	releaseChannel = "beta"
	defer func() { releaseChannel = old }()

	doc := routerForgeReleaseIndex{
		SchemaVersion: 1,
		Channel:       "beta",
		Components: []catalogRelease{
			{
				Package: "routerforge-core",
				Version: "0.3.2-beta",
				Asset:   "routerforge-core_0.3.2-beta_aarch64-3.10.ipk",
				SHA256:  "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				URL:     "https://github.com/example/routerforge/releases/download/routerforge-beta/routerforge-core_0.3.2-beta_aarch64-3.10.ipk",
			},
		},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseRouterForgeReleaseIndex(data); err == nil {
		t.Fatal("foreign release repository was accepted")
	}
}

func TestCoreCanExposeIndependentUpdate(t *testing.T) {
	item := catalogItem{
		ID:              "routerforge-core",
		Builtin:         true,
		Installed:       true,
		Version:         "0.3.0-beta",
		UpdateAvailable: true,
		Update: catalogInstallPlan{
			Method:   "routerforge-release",
			Packages: []string{"routerforge-core"},
		},
	}
	actions := deriveCatalogActions(item)
	if !actions.Update || actions.Install || actions.Remove {
		t.Fatalf("unexpected core actions: %#v", actions)
	}
}
