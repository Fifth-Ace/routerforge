package main

import "testing"

func TestApplyCatalogTrustModelSeparatesMetadataFromLifecycle(t *testing.T) {
	item := catalogItem{
		ProjectURL:     "https://example.test/project",
		Source:         "project-official",
		ManifestSource: "marketplace/submissions/example.json",
		RegistrySource: "routerforge-community",
		Publisher: catalogPublisher{
			ID:   "example",
			Name: "Example",
			URL:  "https://example.test",
		},
		Trust: catalogTrust{
			Status:     "verified",
			ReviewedBy: "routerforge",
		},
		Actions: catalogActions{
			Install: true,
			Update:  true,
			Remove:  true,
		},
	}

	before := item.Actions
	applyCatalogTrustModel(&item)

	if item.Provenance.PublisherID != "example" {
		t.Fatalf("publisher provenance = %q", item.Provenance.PublisherID)
	}
	if item.Provenance.ManifestSource != "marketplace/submissions/example.json" {
		t.Fatalf("manifest provenance = %q", item.Provenance.ManifestSource)
	}
	if item.Provenance.RegistrySource != "routerforge-community" {
		t.Fatalf("registry provenance = %q", item.Provenance.RegistrySource)
	}
	if item.LifecycleTrust.Status != "reviewed" {
		t.Fatalf("lifecycle trust = %q", item.LifecycleTrust.Status)
	}
	if item.Actions != before {
		t.Fatalf("trust model must not grant or revoke actions: before=%+v after=%+v", before, item.Actions)
	}
}

func TestApplyCatalogTrustModelRestrictsUnverifiedMetadata(t *testing.T) {
	item := catalogItem{
		Trust: catalogTrust{Status: "unverified"},
	}

	applyCatalogTrustModel(&item)

	if item.LifecycleTrust.Status != "restricted" {
		t.Fatalf("unverified lifecycle trust = %q", item.LifecycleTrust.Status)
	}
}

func TestApplyCatalogTrustModelBlocksBlockedMetadata(t *testing.T) {
	item := catalogItem{
		Trust: catalogTrust{Status: "blocked"},
	}

	applyCatalogTrustModel(&item)

	if item.LifecycleTrust.Status != "blocked" {
		t.Fatalf("blocked lifecycle trust = %q", item.LifecycleTrust.Status)
	}
}
