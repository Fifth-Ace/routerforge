package main

import "testing"

func TestPackageManagementFollowsRiskAcceptance(t *testing.T) {
	withTempAppSources(t)

	if packageManagementEnabled() {
		t.Fatal("package management must be disabled before current risk acceptance")
	}

	if _, err := setAppSourceSecurity(appSourceSecurityRequest{
		AllowUnverified:  true,
		Accepted:         true,
		AgreementVersion: appSourcesAgreementVersion,
	}); err != nil {
		t.Fatal(err)
	}

	if !packageManagementEnabled() {
		t.Fatal("package management must be enabled after current risk acceptance")
	}

	if _, err := setAppSourceSecurity(appSourceSecurityRequest{
		AllowUnverified: false,
	}); err != nil {
		t.Fatal(err)
	}

	if packageManagementEnabled() {
		t.Fatal("package management must be disabled after risk permission is revoked")
	}
}

func TestPackageRemovalRemainsAllowedWhenRiskPermissionIsOff(t *testing.T) {
	withTempAppSources(t)

	if packageManagementAllowsAction("install") {
		t.Fatal("install must stay disabled before risk acceptance")
	}
	if packageManagementAllowsAction("update") {
		t.Fatal("update must stay disabled before risk acceptance")
	}
	if !packageManagementAllowsAction("remove") {
		t.Fatal("remove must remain available while package management is disabled")
	}

	if _, err := setAppSourceSecurity(appSourceSecurityRequest{
		AllowUnverified:  true,
		Accepted:         true,
		AgreementVersion: appSourcesAgreementVersion,
	}); err != nil {
		t.Fatal(err)
	}
	if !packageManagementAllowsAction("install") || !packageManagementAllowsAction("update") {
		t.Fatal("install/update must be enabled after risk acceptance")
	}

	if _, err := setAppSourceSecurity(appSourceSecurityRequest{
		AllowUnverified: false,
	}); err != nil {
		t.Fatal(err)
	}
	if packageManagementAllowsAction("install") || packageManagementAllowsAction("update") {
		t.Fatal("install/update must be disabled after permission is revoked")
	}
	if !packageManagementAllowsAction("remove") {
		t.Fatal("remove must remain available after permission is revoked")
	}
}