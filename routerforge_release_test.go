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
