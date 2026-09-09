package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCatalogGETDoesNotRunDiscovery(t *testing.T) {
	originalSnapshot := readCatalog()
	originalReadInstalledPackages := catalogReadInstalledPackages
	originalReadProcessNames := catalogReadProcessNames
	originalLoadOpkgCatalog := catalogLoadOpkgCatalog
	originalApplyRuntimeWebDiscovery := catalogApplyRuntimeWebDiscovery

	defer func() {
		catalogReadInstalledPackages = originalReadInstalledPackages
		catalogReadProcessNames = originalReadProcessNames
		catalogLoadOpkgCatalog = originalLoadOpkgCatalog
		catalogApplyRuntimeWebDiscovery = originalApplyRuntimeWebDiscovery
		storeCatalogSnapshot(originalSnapshot)
	}()

	markerTime := time.Unix(123456789, 0).UTC()
	storeCatalogSnapshot(catalogSnapshot{
		GeneratedAt: markerTime,
		ReadOnly:    true,
		Phase:       "cached-get-test",
		Modules: []catalogItem{
			{ID: "cached-test", Kind: "module", Name: "Cached Test"},
		},
	})

	discoveryCalls := 0
	catalogReadInstalledPackages = func() map[string]string {
		discoveryCalls++
		return nil
	}
	catalogReadProcessNames = func() map[string]bool {
		discoveryCalls++
		return nil
	}
	catalogLoadOpkgCatalog = func(context.Context, bool) ([]entwarePackage, error) {
		discoveryCalls++
		return nil, nil
	}
	catalogApplyRuntimeWebDiscovery = func(*catalogSnapshot, map[string]string) {
		discoveryCalls++
	}

	for i := 0; i < 100; i++ {
		request := httptest.NewRequest(http.MethodGet, "/api/catalog", nil)
		response := httptest.NewRecorder()

		handleCatalogRead(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("GET /api/catalog #%d status=%d, want %d", i+1, response.Code, http.StatusOK)
		}
		var snapshot catalogSnapshot
		if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
			t.Fatalf("GET /api/catalog #%d returned invalid JSON: %v", i+1, err)
		}
		if !snapshot.GeneratedAt.Equal(markerTime) {
			t.Fatalf("GET /api/catalog #%d refreshed snapshot: generated_at=%s, want %s", i+1, snapshot.GeneratedAt, markerTime)
		}
		if snapshot.Phase != "cached-get-test" {
			t.Fatalf("GET /api/catalog #%d phase=%q, want cached-get-test", i+1, snapshot.Phase)
		}
	}

	if discoveryCalls != 0 {
		t.Fatalf("100 GET /api/catalog requests invoked active discovery %d times, want 0", discoveryCalls)
	}
}
