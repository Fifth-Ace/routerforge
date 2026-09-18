package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestGitHubManifestlessRepositoryFallsBackAfterManifest404s(t *testing.T) {
	oldFetch := appSourceFetchBytes
	t.Cleanup(func() { appSourceFetchBytes = oldFetch })

	const headSHA = "0123456789abcdef0123456789abcdef01234567"
	appSourceFetchBytes = func(ctx context.Context, rawURL string) ([]byte, error) {
		switch {
		case rawURL == "https://api.github.com/repos/Runnin4ik/dpi-detector":
			return []byte(`{"default_branch":"main"}`), nil
		case strings.Contains(rawURL, "/contents/"):
			return nil, fmt.Errorf("source HTTP 404")
		case rawURL == "https://api.github.com/repos/Runnin4ik/dpi-detector/branches/main":
			return []byte(`{"commit":{"sha":"` + headSHA + `"}}`), nil
		default:
			return nil, fmt.Errorf("unexpected URL: %s", rawURL)
		}
	}

	cache, err := resolveGitHubSource(
		context.Background(),
		"https://github.com/Runnin4ik/dpi-detector/",
		"auto",
	)
	if err != nil {
		t.Fatalf("manifestless GitHub source rejected: %v", err)
	}
	if !cache.Manifestless {
		t.Fatalf("manifestless=%v, want true", cache.Manifestless)
	}
	if cache.Kind != "app" || cache.RegistryID != "github-manifestless" {
		t.Fatalf("unexpected fallback cache: %#v", cache)
	}
	if len(cache.Entries) != 1 {
		t.Fatalf("entries=%d, want 1", len(cache.Entries))
	}
	item := cache.Entries[0]
	if item.ID != "dpi-detector" || item.Kind != "integration" {
		t.Fatalf("unexpected synthetic item: %#v", item)
	}
	if len(item.Detection.Packages) != 1 || item.Detection.Packages[0] != "dpi-detector" {
		t.Fatalf("safe opkg package candidate missing: %#v", item.Detection.Packages)
	}
	if item.Install.Method != "" || item.Update.Method != "" || item.Remove.Method != "" {
		t.Fatalf("manifestless source unexpectedly gained declared executable lifecycle: %#v", item)
	}
	if len(cache.ManifestSHA256) != 64 {
		t.Fatalf("fingerprint length=%d, want 64", len(cache.ManifestSHA256))
	}
	if !strings.Contains(cache.ResolvedURL, "/commit/"+headSHA) {
		t.Fatalf("resolved URL is not pinned to HEAD: %q", cache.ResolvedURL)
	}

	preview := previewFromAppSourceCache(cache)
	if !preview.Manifestless || preview.Fingerprint != cache.ManifestSHA256 {
		t.Fatalf("manifestless preview metadata missing: %#v", preview)
	}

	// Existing safety model may expose a direct opkg action only after the
	// package is actually present in configured feeds.
	item.AvailableVersion = "1.0.0"
	plan, ok := unverifiedDirectOpkgPlan(item, "install")
	if !ok || plan.Method != "opkg" || len(plan.Packages) != 1 || plan.Packages[0] != "dpi-detector" {
		t.Fatalf("safe direct-opkg fallback unavailable: ok=%v plan=%#v", ok, plan)
	}
}

func TestGitHubInvalidManifestDoesNotDowngradeToManifestless(t *testing.T) {
	oldFetch := appSourceFetchBytes
	t.Cleanup(func() { appSourceFetchBytes = oldFetch })

	branchFetches := 0
	appSourceFetchBytes = func(ctx context.Context, rawURL string) ([]byte, error) {
		switch {
		case rawURL == "https://api.github.com/repos/example/broken":
			return []byte(`{"default_branch":"main"}`), nil
		case strings.Contains(rawURL, "/contents/.routerforge/index.json"):
			// A file exists, but its decoded document is not a valid RouterForge registry.
			return []byte(`{"content":"e30=","encoding":"base64","download_url":"https://raw.githubusercontent.com/example/broken/main/.routerforge/index.json","type":"file"}`), nil
		case strings.Contains(rawURL, "/contents/"):
			return nil, fmt.Errorf("source HTTP 404")
		case strings.Contains(rawURL, "/branches/"):
			branchFetches++
			return []byte(`{"commit":{"sha":"0123456789abcdef0123456789abcdef01234567"}}`), nil
		default:
			return nil, fmt.Errorf("unexpected URL: %s", rawURL)
		}
	}

	cache, err := resolveGitHubSource(
		context.Background(),
		"https://github.com/example/broken",
		"auto",
	)
	if err == nil {
		t.Fatalf("invalid manifest was silently downgraded: %#v", cache)
	}
	if branchFetches != 0 {
		t.Fatalf("manifestless fallback was attempted after invalid manifest: branch fetches=%d", branchFetches)
	}
}
