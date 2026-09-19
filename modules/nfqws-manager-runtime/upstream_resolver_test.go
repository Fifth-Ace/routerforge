package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func withUpstreamTestEnv(t *testing.T, handler http.HandlerFunc, now *time.Time) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	oldAPIBase, oldGitBase := nfqwsMenuAPIBase, nfqwsMenuGitBase
	oldClient, oldNow := upstreamHTTPClient, upstreamNow
	oldCachePath := nfqwsMenuCachePath

	nfqwsMenuAPIBase = server.URL
	nfqwsMenuGitBase = server.URL
	upstreamHTTPClient = server.Client()
	upstreamNow = func() time.Time { return now.UTC() }
	nfqwsMenuCachePath = filepath.Join(t.TempDir(), "nfqws-menu-ref.json")

	t.Cleanup(func() {
		server.Close()
		nfqwsMenuAPIBase, nfqwsMenuGitBase = oldAPIBase, oldGitBase
		upstreamHTTPClient, upstreamNow = oldClient, oldNow
		nfqwsMenuCachePath = oldCachePath
	})
	return server
}

func TestResolveNfqwsMenuRefAPIAndFreshCache(t *testing.T) {
	now := time.Date(2026, 9, 19, 9, 0, 0, 0, time.UTC)
	sha := strings.Repeat("a", 40)
	var apiCalls int32
	withUpstreamTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repos/") {
			atomic.AddInt32(&apiCalls, 1)
			w.Header().Set("X-RateLimit-Remaining", "59")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sha":"` + sha + `"}`))
			return
		}
		t.Fatalf("unexpected fallback request: %s", r.URL.String())
	}, &now)

	first := resolveNfqwsMenuRef()
	if first.State != "synced" || first.SHA != sha || first.Source != "github-api" {
		t.Fatalf("unexpected first resolution: %+v", first)
	}
	second := resolveNfqwsMenuRef()
	if second.State != "synced" || second.SHA != sha {
		t.Fatalf("unexpected cached resolution: %+v", second)
	}
	if got := atomic.LoadInt32(&apiCalls); got != 1 {
		t.Fatalf("expected one API call, got %d", got)
	}
}

func TestResolveNfqwsMenuRefRateLimitFallsBackToGitSmartHTTP(t *testing.T) {
	now := time.Date(2026, 9, 19, 9, 0, 0, 0, time.UTC)
	sha := strings.Repeat("b", 40)
	withUpstreamTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repos/") {
			w.Header().Set("X-RateLimit-Remaining", "0")
			http.Error(w, "rate limited", http.StatusForbidden)
			return
		}
		if strings.Contains(r.URL.Path, ".git/info/refs") {
			w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
			_, _ = w.Write([]byte("001e# service=git-upload-pack\n0000" + sha + " refs/heads/main\n0000"))
			return
		}
		http.NotFound(w, r)
	}, &now)

	got := resolveNfqwsMenuRef()
	if got.State != "synced" || got.SHA != sha || got.Source != "git-smart-http" {
		t.Fatalf("expected smart-http fallback, got %+v", got)
	}
	if got.RateLimitRemaining == nil || *got.RateLimitRemaining != 0 {
		t.Fatalf("expected rate limit metadata, got %+v", got)
	}
}

func TestResolveNfqwsMenuRefUsesStaleCacheWhenNetworkFails(t *testing.T) {
	now := time.Date(2026, 9, 19, 9, 0, 0, 0, time.UTC)
	sha := strings.Repeat("c", 40)
	var fail atomic.Bool
	withUpstreamTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if !fail.Load() && strings.HasPrefix(r.URL.Path, "/repos/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sha":"` + sha + `"}`))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/repos/") {
			http.Error(w, "rate limited", http.StatusForbidden)
			return
		}
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	}, &now)

	first := resolveNfqwsMenuRef()
	if first.SHA != sha {
		t.Fatalf("failed to seed cache: %+v", first)
	}
	fail.Store(true)
	now = now.Add(20 * time.Minute)

	stale := resolveNfqwsMenuRef()
	if stale.State != "stale_cache" || stale.SHA != sha || !stale.Stale {
		t.Fatalf("expected stale cache, got %+v", stale)
	}
}

func TestResolveNfqwsMenuRefRateLimitedWithoutCache(t *testing.T) {
	now := time.Date(2026, 9, 19, 9, 0, 0, 0, time.UTC)
	withUpstreamTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repos/") {
			w.Header().Set("X-RateLimit-Remaining", "0")
			http.Error(w, "rate limited", http.StatusForbidden)
			return
		}
		http.Error(w, "fallback unavailable", http.StatusServiceUnavailable)
	}, &now)

	got := resolveNfqwsMenuRef()
	if got.State != "rate_limited" || got.SHA != "" {
		t.Fatalf("expected rate_limited without SHA, got %+v", got)
	}
}
