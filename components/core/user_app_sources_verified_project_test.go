package main

import (
	"strings"
	"testing"
)

func verifiedProjectFixture() routerForgeRegistryDocument {
	return routerForgeRegistryDocument{
		SchemaVersion: 1,
		RegistryID:    "routerforge-community",
		Brand:         "RouterForge",
		Entries: []catalogItem{
			{
				ID:         "demo",
				Kind:       "integration",
				ProjectURL: "https://github.com/example-owner/example-repo",
				Publisher: catalogPublisher{
					ID:   "example-owner",
					Name: "Example Owner",
				},
				Trust: catalogTrust{
					Status:     "verified",
					ReviewedBy: "routerforge",
					Note:       "Project identity verified.",
				},
			},
		},
	}
}

func TestMatchVerifiedProjectUsesRepositoryAndPublisherIdentity(t *testing.T) {
	doc := verifiedProjectFixture()
	item := catalogItem{
		Publisher: catalogPublisher{ID: "example-owner", Name: "Example Owner"},
	}

	trust, ok := matchVerifiedProject(
		doc,
		"https://github.com/Example-Owner/example-repo.git",
		item,
	)
	if !ok {
		t.Fatal("verified project identity did not match")
	}
	if trust.Status != "verified" || trust.ReviewedBy != "routerforge" {
		t.Fatalf("unexpected trust: %#v", trust)
	}

	item.Publisher.ID = "other-owner"
	if _, ok := matchVerifiedProject(doc, "https://github.com/example-owner/example-repo", item); ok {
		t.Fatal("publisher mismatch was accepted as verified")
	}

	item.Publisher.ID = "example-owner"
	if _, ok := matchVerifiedProject(doc, "https://github.com/example-owner/other-repo", item); ok {
		t.Fatal("repository mismatch was accepted as verified")
	}
}

func TestMatchVerifiedProjectRequiresCentralVerifiedState(t *testing.T) {
	doc := verifiedProjectFixture()
	doc.Entries[0].Trust.Status = "unverified"
	item := catalogItem{Publisher: catalogPublisher{ID: "example-owner"}}
	if _, ok := matchVerifiedProject(doc, "https://github.com/example-owner/example-repo", item); ok {
		t.Fatal("unverified central registry entry promoted user source")
	}
}

func TestVerifiedUserSourceLifecycleRemainsRestricted(t *testing.T) {
	item := catalogItem{
		ID:             "src-123456789abc:demo",
		RegistrySource: "src-123456789abc",
		Trust:          catalogTrust{Status: "verified"},
		Actions: catalogActions{
			Install: true,
			Update:  true,
			Remove:  true,
		},
	}

	appSourceApplyActionPolicy(&item)
	if item.Actions.Install || item.Actions.Update {
		t.Fatalf("verified identity opened developer lifecycle: %#v", item.Actions)
	}
	if !item.Actions.Remove {
		t.Fatal("remove was unexpectedly blocked")
	}
	if !strings.Contains(strings.ToLower(item.Actions.Reason), "lifecycle") {
		t.Fatalf("restricted lifecycle reason missing: %q", item.Actions.Reason)
	}

	if reason := appSourceActionBlockReason(item, "install", ""); !strings.Contains(reason, "does not authorize") {
		t.Fatalf("verified install block reason=%q", reason)
	}
	if reason := appSourceActionBlockReason(item, "remove", ""); reason != "" {
		t.Fatalf("remove should remain available: %q", reason)
	}
}
