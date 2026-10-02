package main

import "testing"

func TestP29EcosystemBatchA(t *testing.T) {
	items := auditedEcosystemIntegrations()
	tests := []struct {
		id         string
		executable bool
		preview    bool
		remove     bool
	}{
		{"antigoblin", true, false, false},
		{"xkeen-panel", true, false, false},
		{"keensnap", true, false, true},
		{"sms2gram", true, false, true},
		{"web4static", true, false, true},
		{"magitrickle", false, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			var item *catalogItem
			for i := range items {
				if items[i].ID == tt.id {
					item = &items[i]
					break
				}
			}
			if item == nil {
				t.Fatal("catalog item missing")
			}
			if item.Trust.Status != "verified" {
				t.Fatalf("trust=%q", item.Trust.Status)
			}
			if item.Install.PreviewOnly != tt.preview {
				t.Fatalf("preview=%v want=%v", item.Install.PreviewOnly, tt.preview)
			}
			if got := executableCatalogPlan(item.Install); got != tt.executable {
				t.Fatalf("executable=%v want=%v plan=%#v", got, tt.executable, item.Install)
			}
			if tt.executable {
				if err := validateCatalogPlan(item.Install); err != nil {
					t.Fatalf("install plan invalid: %v", err)
				}
			}
			if tt.remove && item.Remove.Method == "" {
				t.Fatal("remove plan missing")
			}
		})
	}
}

func TestP29ExecutableBatchUsesApprovedHTTPSInstallerHosts(t *testing.T) {
	for _, item := range auditedEcosystemIntegrations() {
		switch item.ID {
		case "antigoblin", "xkeen-panel", "keensnap", "sms2gram", "web4static":
			if item.Install.Method != "official-script" {
				t.Fatalf("%s method=%q", item.ID, item.Install.Method)
			}
			if !validOfficialScriptURL(item.Install.InstallerURL) {
				t.Fatalf("%s installer URL rejected: %s", item.ID, item.Install.InstallerURL)
			}
		}
	}
}
