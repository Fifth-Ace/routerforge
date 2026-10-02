package main

import "testing"

func requireAuditedEcosystemIDs(t *testing.T, ids ...string) {
	t.Helper()
	items := auditedEcosystemIntegrations()

	for _, id := range ids {
		id := id
		t.Run(id, func(t *testing.T) {
			for i := range items {
				if items[i].ID != id {
					continue
				}
				if items[i].Trust.Status != "verified" {
					t.Fatalf("trust=%q", items[i].Trust.Status)
				}
				return
			}
			t.Fatal("catalog item missing")
		})
	}
}
