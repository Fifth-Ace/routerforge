package main

import "testing"

func TestPackageManagementFollowsRiskAcceptance(t *testing.T) {
	withTempAppSources(t)

	if marketplaceTestInstallEnabled() {
		t.Fatal("package management must be disabled before current risk acceptance")
	}

	if _, err := setAppSourceSecurity(appSourceSecurityRequest{
		AllowUnverified:  true,
		Accepted:         true,
		AgreementVersion: appSourcesAgreementVersion,
	}); err != nil {
		t.Fatal(err)
	}

	if !marketplaceTestInstallEnabled() {
		t.Fatal("package management must be enabled after current risk acceptance")
	}

	if _, err := setAppSourceSecurity(appSourceSecurityRequest{
		AllowUnverified: false,
	}); err != nil {
		t.Fatal(err)
	}

	if marketplaceTestInstallEnabled() {
		t.Fatal("package management must be disabled after risk permission is revoked")
	}
}
