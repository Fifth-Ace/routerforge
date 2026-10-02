package main

import "strings"

// P29 reviewed lifecycle overrides.
//
// These overrides intentionally sit on top of the current community registry
// while the large third-party catalog is migrated in reviewed batches. They
// grant executable lifecycle authority only to exact upstream contracts that
// were re-audited on 2026-10-02.
func applyReviewedOfficialScriptLifecycle(snapshot *catalogSnapshot) {
	if snapshot == nil {
		return
	}

	if item := findCatalogItem(snapshot, "antiscan", "integration"); item != nil {
		item.Source = "project-official"
		item.Publisher = auditedPublisher("dimon27254", "https://github.com/dimon27254/antiscan")
		item.Trust = auditedTrust("Official Antiscan install/update script re-audited against upstream README and install.sh on 2026-10-02.")
		item.Capabilities = lifecycleCapabilities(item.Capabilities)
		item.Install = catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/dimon27254/antiscan/refs/heads/main/install.sh",
			Packages:     []string{"antiscan"},
			Notes: []string{
				"Upstream official installer adds the Antiscan feed and installs/reinstalls the antiscan package.",
				"RouterForge downloads the script first, records the executed SHA256 and streams the installer output.",
			},
		}
		item.Update = item.Install
		item.Remove = catalogInstallPlan{}
		refreshReviewedLifecycle(item)
	}

	if item := findCatalogItem(snapshot, "awg-manager", "integration"); item != nil {
		item.Source = "project-official"
		item.Publisher = auditedPublisher("hoaxisr", "https://github.com/hoaxisr/awg-manager")
		item.Trust = auditedTrust("Official AWG Manager HTTPS installer re-audited against the upstream develop README and scripts/install.sh on 2026-10-02.")
		item.Capabilities = lifecycleCapabilities(item.Capabilities)
		item.Install = catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/hoaxisr/awg-manager/develop/scripts/install.sh",
			Packages:     []string{"awg-manager"},
			Notes: []string{
				"Uses the upstream HTTPS GitHub installer documented for Keenetic.",
				"The installer detects architecture, configures the upstream feed, installs or updates awg-manager and performs its own health check.",
			},
		}
		item.Update = item.Install
		item.Remove = catalogInstallPlan{}
		refreshReviewedLifecycle(item)
	}

	if item := findCatalogItem(snapshot, "hydraroute-neo", "integration"); item != nil {
		item.Source = "project-official"
		item.Publisher = auditedPublisher("Ground-Zerro", "https://github.com/Ground-Zerro/HydraRoute")
		item.Trust = auditedTrust("HydraRoute Neo installer re-audited against the upstream README and Ground-Zerro release repository on 2026-10-02.")
		item.Capabilities = lifecycleCapabilities(item.Capabilities)
		item.Install = catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://git.zerrolabs.org/Ground-Zerro/release/pages/keenetic/install-neo.sh",
			Packages:     []string{"hrneo", "hrweb"},
			Notes: []string{
				"Uses the exact HydraRoute Neo installer documented by the upstream project.",
				"The upstream uninstall script is deliberately not exposed automatically because it removes multiple shared packages and reboots the router.",
			},
		}
		item.Update = item.Install
		item.Remove = catalogInstallPlan{}
		refreshReviewedLifecycle(item)
	}

	if item := findCatalogItem(snapshot, "razvilka", "integration"); item != nil {
		item.Source = "project-official"
		item.Publisher = auditedPublisher("ArtixSx", "https://github.com/ArtixSx/RAZVILKA")
		item.Trust = auditedTrust("RAZVILKA bootstrap install/update/uninstall contract re-audited against upstream README and scripts/bootstrap.sh on 2026-10-02.")
		item.Capabilities = lifecycleCapabilities(item.Capabilities)
		item.Install = catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/ArtixSx/RAZVILKA/main/scripts/bootstrap.sh",
			Notes: []string{
				"Upstream bootstrap verifies the release archive SHA256 before delegating writes to the release installer.",
				"Default invocation installs or updates the panel without the optional starter component pack.",
			},
		}
		item.Update = item.Install
		item.Remove = catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/ArtixSx/RAZVILKA/main/scripts/bootstrap.sh",
			Args:         []string{"--uninstall"},
			Notes: []string{
				"Uses the upstream --uninstall contract, which removes the panel and owned routes while preserving settings, connections and backups.",
			},
		}
		refreshReviewedLifecycle(item)
	}

	if item := findCatalogItem(snapshot, "xkeen-ui", "integration"); item != nil {
		item.Source = "project-official"
		item.Publisher = auditedPublisher("zxc-rv", "https://github.com/zxc-rv/XKeen-UI")
		item.Trust = auditedTrust("Official setup.sh, architecture detection, service path and default Web port were reviewed on 2026-10-03; execution remains preview-only because setup.sh presents an interactive install/update/remove menu.")
		item.Install = catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/zxc-rv/XKeen-UI/main/setup.sh",
			PreviewOnly:  true,
			Notes: []string{
				"Upstream setup.sh is an interactive menu rather than a noninteractive install action.",
				"RouterForge keeps the reviewed source visible but does not guess menu input.",
			},
		}
		item.Update = catalogInstallPlan{}
		item.Remove = catalogInstallPlan{}
		refreshReviewedLifecycle(item)
	}

	if item := findCatalogItem(snapshot, "keen-pbr", "integration"); item != nil {
		item.Source = "project-official"
		item.Publisher = auditedPublisher("maksimkurb", "https://github.com/maksimkurb/keen-pbr")
		item.Trust = auditedTrust("Dedicated Keenetic/Entware feed, full/headless package split and Web UI behavior were reviewed on 2026-10-03; install remains preview-only because choosing keen-pbr versus keen-pbr-headless is an explicit user decision.")
		item.Install = catalogInstallPlan{
			Method:      "manual",
			Packages:    []string{"keen-pbr", "keen-pbr-headless"},
			PreviewOnly: true,
			Notes: []string{
				"Upstream provides a dedicated Keenetic Entware repository.",
				"Choose exactly one package variant: keen-pbr with Web UI or keen-pbr-headless without it.",
			},
		}
		item.Update = catalogInstallPlan{}
		item.Remove = catalogInstallPlan{}
		refreshReviewedLifecycle(item)
	}
}

func lifecycleCapabilities(values []string) []string {
	out := make([]string, 0, len(values)+1)
	seen := false
	for _, value := range values {
		if value == "install-preview" {
			continue
		}
		if value == "package-lifecycle" {
			seen = true
		}
		out = append(out, value)
	}
	if !seen {
		out = append(out, "package-lifecycle")
	}
	return out
}

func refreshReviewedLifecycle(item *catalogItem) {
	if item == nil {
		return
	}
	item.Actions = deriveCatalogActions(*item)
	applyCatalogTrustModel(item)
	if strings.EqualFold(item.Trust.Status, "verified") && item.LifecycleTrust.Status == "" {
		item.LifecycleTrust = catalogLifecycleTrust{
			Status: "reviewed",
			Reason: "Reviewed upstream lifecycle contract.",
		}
	}
}
