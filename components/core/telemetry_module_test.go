package main

import "testing"

func TestInstalledRouterForgeModulesStrictAllowlist(t *testing.T) {
	got := installedRouterForgeModules(map[string]string{
		"routerforge-dns":           "0.13.0~beta.9",
		"routerforge-nfqws-manager": "0.12.0",
		"routerforge-custom-danger": "1.0.0",
		"routerforge-admin":         "not allowed version with spaces",
	})
	if len(got) != 2 || got[0].ID != "dns" || got[1].ID != "nfqws-manager" {
		t.Fatalf("unexpected module selection: %#v", got)
	}
}
