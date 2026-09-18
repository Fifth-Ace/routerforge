package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestUnmanagedGitHubAssetMatchARM64(t *testing.T) {
	release := githubReleaseAPI{
		ID:         42,
		TagName:    "v4.2.4",
		Prerelease: false,
		Assets: []githubReleaseAssetAPI{
			{ID: 1, Name: "dpi_detector_v4.2.4_win10_arm64.exe", BrowserDownloadURL: "https://github.com/example/app/releases/download/v4.2.4/win.exe", Size: 1},
			{ID: 2, Name: "dpi_detector_v4.2.4_linux_armv7", BrowserDownloadURL: "https://github.com/example/app/releases/download/v4.2.4/armv7", Size: 2},
			{ID: 3, Name: "dpi_detector_v4.2.4_linux_arm64", BrowserDownloadURL: "https://github.com/example/app/releases/download/v4.2.4/arm64", Size: 3},
			{ID: 4, Name: "dpi_detector_v4.2.4_linux_arm64.tar.gz", BrowserDownloadURL: "https://github.com/example/app/releases/download/v4.2.4/archive", Size: 4},
		},
	}
	asset, ok := unmanagedReleaseAssetForTarget(release, "aarch64-3.10", "release")
	if !ok {
		t.Fatal("ARM64 release asset was not detected")
	}
	if asset.AssetID != 3 || asset.Asset != "dpi_detector_v4.2.4_linux_arm64" {
		t.Fatalf("wrong ARM64 asset selected: %#v", asset)
	}
	if asset.Version != "4.2.4" || asset.Channel != "release" || asset.Architecture != "aarch64-3.10" {
		t.Fatalf("wrong release metadata: %#v", asset)
	}
}

func TestUnmanagedGitHubAssetRejectsWrongArchitectureAndArchives(t *testing.T) {
	for _, name := range []string{
		"tool_linux_x86_64",
		"tool_linux_armv7",
		"tool_macos_arm64",
		"tool_windows_arm64.exe",
		"tool_linux_arm64.tar.gz",
		"tool_linux_arm64.zip",
		"tool_linux_arm64.sha256",
	} {
		if score := unmanagedRawBinaryAssetScore(name, "aarch64-3.10"); score >= 0 {
			t.Fatalf("unsafe or incompatible asset accepted: %s score=%d", name, score)
		}
	}
}

func TestDiscoverUnmanagedGitHubReleaseAndBeta(t *testing.T) {
	oldFetch := appSourceFetchBytes
	t.Cleanup(func() { appSourceFetchBytes = oldFetch })

	appSourceFetchBytes = func(ctx context.Context, rawURL string) ([]byte, error) {
		if !strings.Contains(rawURL, "/releases?per_page=20") {
			return nil, fmt.Errorf("unexpected URL: %s", rawURL)
		}
		return []byte(`[
		  {
		    "id": 10,
		    "tag_name": "v4.2.4",
		    "draft": false,
		    "prerelease": false,
		    "published_at": "2026-09-01T00:00:00Z",
		    "assets": [
		      {
		        "id": 100,
		        "name": "dpi_detector_v4.2.4_linux_arm64",
		        "browser_download_url": "https://github.com/Runnin4ik/dpi-detector/releases/download/v4.2.4/dpi_detector_v4.2.4_linux_arm64",
		        "size": 1234
		      }
		    ]
		  },
		  {
		    "id": 11,
		    "tag_name": "v4.3.0-beta.1",
		    "draft": false,
		    "prerelease": true,
		    "published_at": "2026-09-10T00:00:00Z",
		    "assets": [
		      {
		        "id": 101,
		        "name": "dpi_detector_v4.3.0-beta.1_linux_arm64",
		        "browser_download_url": "https://github.com/Runnin4ik/dpi-detector/releases/download/v4.3.0-beta.1/dpi_detector_v4.3.0-beta.1_linux_arm64",
		        "size": 2345
		      }
		    ]
		  }
		]`), nil
	}

	meta := discoverUnmanagedGitHubReleases(
		context.Background(),
		"Runnin4ik",
		"dpi-detector",
		"aarch64-3.10",
	)
	if meta.Stable == nil || meta.Stable.Tag != "v4.2.4" {
		t.Fatalf("stable candidate missing: %#v", meta)
	}
	if meta.Beta == nil || meta.Beta.Tag != "v4.3.0-beta.1" {
		t.Fatalf("beta candidate missing: %#v", meta)
	}
}

func TestUnmanagedGitHubChannelSelectionRequiresExplicitBeta(t *testing.T) {
	item := catalogItem{
		ID:             "src-123456789abc:dpi-detector",
		ManifestID:     "dpi-detector",
		RegistrySource: "src-123456789abc",
		Kind:           "integration",
		Name:           "dpi-detector",
		ProjectURL:     "https://github.com/Runnin4ik/dpi-detector",
		Trust:          catalogTrust{Status: "unverified"},
		UnmanagedGitHub: &catalogUnmanagedGitHub{
			Owner:  "Runnin4ik",
			Repo:   "dpi-detector",
			Target: "aarch64-3.10",
			Beta: &catalogUnmanagedGitHubAsset{
				Channel:      "beta",
				Tag:          "v4.3.0-beta.1",
				Version:      "4.3.0-beta.1",
				Prerelease:   true,
				Asset:        "dpi_detector_v4.3.0-beta.1_linux_arm64",
				URL:          "https://github.com/Runnin4ik/dpi-detector/releases/download/v4.3.0-beta.1/dpi_detector_v4.3.0-beta.1_linux_arm64",
				Architecture: "aarch64-3.10",
			},
		},
	}

	selectUnmanagedGitHubRelease(&item, "auto")
	if item.UnmanagedGitHub.Selected != nil || item.Install.Method != "" {
		t.Fatalf("AUTO silently selected beta: %#v", item.UnmanagedGitHub)
	}
	if !strings.Contains(unmanagedGitHubActionReason(item), "select Beta explicitly") {
		t.Fatalf("AUTO beta hint missing: %q", unmanagedGitHubActionReason(item))
	}

	selectUnmanagedGitHubRelease(&item, "beta")
	if item.UnmanagedGitHub.Selected == nil || item.UnmanagedGitHub.SelectedChannel != "beta" {
		t.Fatalf("explicit beta selection failed: %#v", item.UnmanagedGitHub)
	}
	if item.Install.Method != "github-release-binary" || item.Update.Method != "github-release-binary" {
		t.Fatalf("beta executable plan missing: install=%#v update=%#v", item.Install, item.Update)
	}
}

func TestUnmanagedGitHubReleasePlanLifecycle(t *testing.T) {
	selected := &catalogUnmanagedGitHubAsset{
		Channel:      "release",
		Tag:          "v4.2.4",
		Version:      "4.2.4",
		Asset:        "dpi_detector_v4.2.4_linux_arm64",
		URL:          "https://github.com/Runnin4ik/dpi-detector/releases/download/v4.2.4/dpi_detector_v4.2.4_linux_arm64",
		Architecture: "aarch64-3.10",
	}
	item := catalogItem{
		ID:             "src-123456789abc:dpi-detector",
		ManifestID:     "dpi-detector",
		RegistrySource: "src-123456789abc",
		Kind:           "integration",
		Name:           "dpi-detector",
		ProjectURL:     "https://github.com/Runnin4ik/dpi-detector",
		Trust:          catalogTrust{Status: "unverified"},
		UnmanagedGitHub: &catalogUnmanagedGitHub{
			Owner:            "Runnin4ik",
			Repo:             "dpi-detector",
			Target:           "aarch64-3.10",
			RequestedChannel: "release",
			SelectedChannel:  "release",
			Stable:           selected,
			Selected:         selected,
		},
		Install: catalogInstallPlan{Method: "github-release-binary", InstallerURL: selected.URL},
		Update:  catalogInstallPlan{Method: "github-release-binary", InstallerURL: selected.URL},
		Remove:  catalogInstallPlan{Method: "github-release-binary"},
	}

	if plan, ok := unverifiedGitHubReleasePlan(item, "install"); !ok || plan.Method != "github-release-binary" {
		t.Fatalf("install plan rejected: ok=%v plan=%#v", ok, plan)
	}

	item.Installed = true
	item.UpdateAvailable = true
	if plan, ok := unverifiedGitHubReleasePlan(item, "update"); !ok || plan.Method != "github-release-binary" {
		t.Fatalf("update plan rejected: ok=%v plan=%#v", ok, plan)
	}
	if plan, ok := unverifiedGitHubReleasePlan(item, "remove"); !ok || plan.Method != "github-release-binary" {
		t.Fatalf("remove plan rejected: ok=%v plan=%#v", ok, plan)
	}
}

func TestValidUnmanagedGitHubReleaseURLBindsRepository(t *testing.T) {
	item := catalogItem{
		UnmanagedGitHub: &catalogUnmanagedGitHub{Owner: "Runnin4ik", Repo: "dpi-detector"},
	}
	if !validUnmanagedGitHubReleaseURL(
		item,
		"https://github.com/Runnin4ik/dpi-detector/releases/download/v4.2.4/dpi_detector_v4.2.4_linux_arm64",
	) {
		t.Fatal("valid source-bound GitHub release URL rejected")
	}
	for _, raw := range []string{
		"https://github.com/other/dpi-detector/releases/download/v4.2.4/file",
		"https://github.com/Runnin4ik/other/releases/download/v4.2.4/file",
		"https://example.com/Runnin4ik/dpi-detector/releases/download/v4.2.4/file",
	} {
		if validUnmanagedGitHubReleaseURL(item, raw) {
			t.Fatalf("foreign release URL accepted: %s", raw)
		}
	}
}
