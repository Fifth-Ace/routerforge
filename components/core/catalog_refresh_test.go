package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testCatalogRefreshResult() catalogRefreshResult {
	return catalogRefreshResult{
		Release:  routerForgeReleaseStatus{Supported: true, Online: true},
		Registry: routerForgeRegistryStatus{Online: true},
		Catalog:  catalogSnapshot{Phase: "rf007-test"},
	}
}

func TestCatalogRefreshCoordinatorSingleflight(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	coordinator := newCatalogRefreshCoordinator(time.Minute, func() catalogRefreshResult {
		if runs.Add(1) == 1 {
			close(started)
		}
		<-release
		return testCatalogRefreshResult()
	})

	const callers = 8
	start := make(chan struct{})
	modes := make(chan string, callers)
	errors := make(chan error, callers)
	var wg sync.WaitGroup
	wg.Add(callers)

	for i := 0; i < callers; i++ {
		go func() {
			defer wg.Done()
			<-start
			_, mode, err := coordinator.do(context.Background())
			modes <- mode
			errors <- err
		}()
	}

	close(start)
	<-started
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	close(modes)
	close(errors)

	if got := runs.Load(); got != 1 {
		t.Fatalf("concurrent refreshes executed %d expensive runs, want 1", got)
	}

	fresh := 0
	for mode := range modes {
		switch mode {
		case "fresh":
			fresh++
		case "joined", "cached":
		default:
			t.Fatalf("unexpected refresh mode %q", mode)
		}
	}
	if fresh != 1 {
		t.Fatalf("fresh responses=%d, want 1", fresh)
	}
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent refresh returned error: %v", err)
		}
	}

	_, mode, err := coordinator.do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if mode != "cached" {
		t.Fatalf("immediate refresh mode=%q, want cached", mode)
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("cached refresh executed another expensive run: %d", got)
	}
}

func TestCatalogRefreshCoordinatorCooldownExpiry(t *testing.T) {
	now := time.Unix(1700000000, 0)
	runs := 0
	coordinator := newCatalogRefreshCoordinator(5*time.Second, func() catalogRefreshResult {
		runs++
		return testCatalogRefreshResult()
	})
	coordinator.now = func() time.Time { return now }

	_, mode, err := coordinator.do(context.Background())
	if err != nil || mode != "fresh" {
		t.Fatalf("first refresh mode=%q err=%v, want fresh", mode, err)
	}

	_, mode, err = coordinator.do(context.Background())
	if err != nil || mode != "cached" {
		t.Fatalf("cooldown refresh mode=%q err=%v, want cached", mode, err)
	}

	now = now.Add(5*time.Second + time.Nanosecond)
	_, mode, err = coordinator.do(context.Background())
	if err != nil || mode != "fresh" {
		t.Fatalf("post-cooldown refresh mode=%q err=%v, want fresh", mode, err)
	}
	if runs != 2 {
		t.Fatalf("expensive runs=%d, want 2", runs)
	}
}

func TestCatalogRefreshCoordinatorFreshBypassesCooldown(t *testing.T) {
	runs := 0
	coordinator := newCatalogRefreshCoordinator(time.Minute, func() catalogRefreshResult {
		runs++
		return testCatalogRefreshResult()
	})

	if _, mode, err := coordinator.do(context.Background()); err != nil || mode != "fresh" {
		t.Fatalf("initial refresh mode=%q err=%v, want fresh", mode, err)
	}

	if _, mode, err := coordinator.do(context.Background()); err != nil || mode != "cached" {
		t.Fatalf("ordinary cooldown refresh mode=%q err=%v, want cached", mode, err)
	}

	if _, mode, err := coordinator.doFresh(context.Background()); err != nil || mode != "fresh" {
		t.Fatalf("manual fresh refresh mode=%q err=%v, want fresh", mode, err)
	}

	if runs != 2 {
		t.Fatalf("expensive refresh runs=%d, want 2", runs)
	}
}

func TestCatalogRefreshHandlerFreshQueryBypassesCooldown(t *testing.T) {
	runs := 0
	coordinator := newCatalogRefreshCoordinator(time.Minute, func() catalogRefreshResult {
		runs++
		return testCatalogRefreshResult()
	})

	first := httptest.NewRequest(http.MethodPost, "http://router.local/api/catalog/refresh", nil)
	firstResponse := httptest.NewRecorder()
	handleCatalogRefreshWithCoordinator(firstResponse, first, coordinator)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first status=%d, want %d", firstResponse.Code, http.StatusOK)
	}

	cached := httptest.NewRequest(http.MethodPost, "http://router.local/api/catalog/refresh", nil)
	cachedResponse := httptest.NewRecorder()
	handleCatalogRefreshWithCoordinator(cachedResponse, cached, coordinator)
	if got := cachedResponse.Header().Get(catalogRefreshModeHeader); got != "cached" {
		t.Fatalf("ordinary second refresh mode=%q, want cached", got)
	}

	fresh := httptest.NewRequest(http.MethodPost, "http://router.local/api/catalog/refresh?fresh=1", nil)
	freshResponse := httptest.NewRecorder()
	handleCatalogRefreshWithCoordinator(freshResponse, fresh, coordinator)
	if freshResponse.Code != http.StatusOK {
		t.Fatalf("fresh status=%d, want %d", freshResponse.Code, http.StatusOK)
	}
	if got := freshResponse.Header().Get(catalogRefreshModeHeader); got != "fresh" {
		t.Fatalf("fresh query mode=%q, want fresh", got)
	}
	if runs != 2 {
		t.Fatalf("expensive refresh runs=%d, want 2", runs)
	}
}

func TestCatalogRefreshHandlerRejectsCrossOrigin(t *testing.T) {
	runs := 0
	coordinator := newCatalogRefreshCoordinator(time.Minute, func() catalogRefreshResult {
		runs++
		return testCatalogRefreshResult()
	})

	request := httptest.NewRequest(http.MethodPost, "http://router.local/api/catalog/refresh", nil)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()

	handleCatalogRefreshWithCoordinator(response, request, coordinator)

	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status=%d, want %d", response.Code, http.StatusForbidden)
	}
	if runs != 0 {
		t.Fatalf("cross-origin request executed %d expensive refreshes, want 0", runs)
	}
}

func TestCatalogRefreshHandlerAllowsSameOriginAndCLI(t *testing.T) {
	tests := []struct {
		name   string
		origin string
	}{
		{name: "same origin", origin: "http://router.local"},
		{name: "no origin cli", origin: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs := 0
			coordinator := newCatalogRefreshCoordinator(time.Minute, func() catalogRefreshResult {
				runs++
				return testCatalogRefreshResult()
			})

			request := httptest.NewRequest(http.MethodPost, "http://router.local/api/catalog/refresh", nil)
			if tt.origin != "" {
				request.Header.Set("Origin", tt.origin)
			}
			response := httptest.NewRecorder()

			handleCatalogRefreshWithCoordinator(response, request, coordinator)

			if response.Code != http.StatusOK {
				t.Fatalf("status=%d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
			}
			if got := response.Header().Get(catalogRefreshModeHeader); got != "fresh" {
				t.Fatalf("refresh mode header=%q, want fresh", got)
			}
			if runs != 1 {
				t.Fatalf("expensive refresh runs=%d, want 1", runs)
			}
		})
	}
}

func TestCatalogRefreshHandlerMethodGate(t *testing.T) {
	coordinator := newCatalogRefreshCoordinator(time.Minute, testCatalogRefreshResult)
	request := httptest.NewRequest(http.MethodGet, "http://router.local/api/catalog/refresh", nil)
	response := httptest.NewRecorder()

	handleCatalogRefreshWithCoordinator(response, request, coordinator)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status=%d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if got := response.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("Allow=%q, want POST", got)
	}
}
