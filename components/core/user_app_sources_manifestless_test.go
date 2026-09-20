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
		case strings.Contains(rawURL, "/releases?per_page=20"):
			return []byte(`[]`), nil
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
	if item.Name != "DPI Detector" || item.Category != "DPI / Diagnostics" {
		t.Fatalf("DPI Detector curated card profile missing: %#v", item)
	}
	if item.Publisher.Name != "Runnin4ik" || item.ProjectURL != "https://github.com/Runnin4ik/dpi-detector" {
		t.Fatalf("DPI Detector attribution missing: %#v", item)
	}
	if !strings.Contains(item.Description, "v5.0.0+") {
		t.Fatalf("DPI Detector v5-only description missing: %q", item.Description)
	}
	if len(item.Detection.Packages) != 0 {
		t.Fatalf("manifestless source retained legacy opkg package guess: %#v", item.Detection.Packages)
	}
	if item.Install.Method != "" || item.Update.Method != "" || item.Remove.Method != "" {
		t.Fatalf("raw source unexpectedly gained lifecycle before catalog channel selection: %#v", item)
	}
	if item.UnmanagedGitHub == nil {
		t.Fatal("GitHub release metadata container missing")
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
