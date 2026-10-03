package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestGitHubManifestlessRepositoryFallsBackAfterManifest404s(t *testing.T) {
	oldFetch := appSourceFetchBytes
	t.Cleanup(func() { appSourceFetchBytes = oldFetch })

	const headSHA = "0123456789abcdef0123456789abcdef01234567"
	appSourceFetchBytes = func(ctx context.Context, rawURL string) ([]byte, error) {
		switch {
		case rawURL == "https://api.github.com/repos/Runnin4ik/dpi-detector":
			return []byte(`{"default_branch":"main"}`), nil
		case strings.Contains(rawURL, "raw.githubusercontent.com/Runnin4ik/dpi-detector/main/"):
			return nil, &appSourceHTTPError{URL: rawURL, Status: 404}
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
		case rawURL == "https://raw.githubusercontent.com/example/broken/main/.routerforge/index.json":
			return []byte(`{}`), nil
		case strings.Contains(rawURL, "raw.githubusercontent.com/example/broken/main/"):
			return nil, &appSourceHTTPError{URL: rawURL, Status: 404}
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

func TestGitHubRepositoryFileUsesRawHost(t *testing.T) {
	oldFetch := appSourceFetchBytes
	t.Cleanup(func() { appSourceFetchBytes = oldFetch })

	var requested string
	appSourceFetchBytes = func(ctx context.Context, rawURL string) ([]byte, error) {
		requested = rawURL
		return []byte(`{"schema_version":1}`), nil
	}

	data, resolved, err := fetchGitHubRepositoryFile(
		context.Background(),
		"example",
		"repo",
		".routerforge/manifest.json",
		"main",
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://raw.githubusercontent.com/example/repo/main/.routerforge/manifest.json"
	if requested != want || resolved != want {
		t.Fatalf("raw URL mismatch: requested=%q resolved=%q want=%q", requested, resolved, want)
	}
	if string(data) != `{"schema_version":1}` {
		t.Fatalf("unexpected raw payload: %q", string(data))
	}
}

func TestGitHubRateLimitClassificationAndBackoff(t *testing.T) {
	err := fmt.Errorf("repository metadata: %w", &appSourceHTTPError{
		URL:                "https://api.github.com/repos/example/repo",
		Status:             403,
		RateLimitRemaining: "0",
	})
	if !appSourceGitHubRateLimited(err) {
		t.Fatal("GitHub rate-limit error was not classified")
	}

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	source := appSourceRecord{
		Error:       "GitHub API rate limit reached; cached source remains available. Retry later.",
		LastErrorAt: now.Add(-15 * time.Minute).Format(time.RFC3339),
	}
	if !appSourceAutomaticRefreshDeferred(source, now) {
		t.Fatal("automatic refresh should be deferred during rate-limit backoff")
	}
	source.LastErrorAt = now.Add(-61 * time.Minute).Format(time.RFC3339)
	if appSourceAutomaticRefreshDeferred(source, now) {
		t.Fatal("automatic refresh remained deferred after backoff")
	}
}
