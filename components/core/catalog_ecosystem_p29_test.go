package main

import "testing"

func TestP29EcosystemBatchA(t *testing.T) {
	requireAuditedEcosystemIDs(t,
		"antigoblin",
		"xkeen-panel",
		"keensnap",
		"sms2gram",
		"web4static",
		"magitrickle",
	)
}

func TestP29ExecutableBatchUsesApprovedHTTPSInstallerHosts(t *testing.T) {
	for _, item := range auditedEcosystemIntegrations() {
		switch item.ID {
		case "antigoblin", "xkeen-panel", "keensnap", "sms2gram":
			if item.Install.Method != "official-script" {
				t.Fatalf("%s method=%q", item.ID, item.Install.Method)
			}
			if !validOfficialScriptURL(item.Install.InstallerURL) {
				t.Fatalf("%s installer URL rejected: %s", item.ID, item.Install.InstallerURL)
			}
		case "web4static":
			if item.Install.Method != "verified-ipk" {
				t.Fatalf("%s method=%q", item.ID, item.Install.Method)
			}
			if err := validateCatalogPlan(item.Install); err != nil {
				t.Fatalf("%s install invalid: %v", item.ID, err)
			}
		}
	}
}
