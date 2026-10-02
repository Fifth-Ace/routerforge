package main

import "sort"

// P27 ecosystem catalog overlay.
//
// These entries are based on an upstream documentation audit. The overlay is
// applied after the normal RouterForge registry/runtime discovery path so a
// future canonical registry entry with the same id automatically wins.
func init() {
	baseDiscovery := catalogApplyRuntimeWebDiscovery
	catalogApplyRuntimeWebDiscovery = func(snapshot *catalogSnapshot, installed map[string]string) {
		baseDiscovery(snapshot, installed)
		applyAuditedEcosystemCatalog(snapshot, installed)
	}
}

func applyAuditedEcosystemCatalog(snapshot *catalogSnapshot, installed map[string]string) {
	if snapshot == nil {
		return
	}

	processes := readProcessNames()
	for _, incoming := range auditedEcosystemIntegrations() {
		if findCatalogItem(snapshot, incoming.ID, "integration") != nil {
			continue
		}
		incoming.RegistrySource = "routerforge-audited-extra"
		resetCatalogRuntime(&incoming)
		finalizeCatalogItem(&incoming, installed, processes, pathExists)
		incoming.Actions = deriveCatalogActions(incoming)
		applyCatalogTrustModel(&incoming)
		snapshot.Integrations = append(snapshot.Integrations, incoming)
	}

	applyReviewedOfficialScriptLifecycle(snapshot)

	sort.SliceStable(snapshot.Integrations, func(i, j int) bool {
		if snapshot.Integrations[i].Category != snapshot.Integrations[j].Category {
			return snapshot.Integrations[i].Category < snapshot.Integrations[j].Category
		}
		return snapshot.Integrations[i].Name < snapshot.Integrations[j].Name
	})
}

func auditedEcosystemIntegrations() []catalogItem {
	return []catalogItem{
		keeneticPolicyUICatalogItem(),
		entwareManagerCatalogItem(),
		zapretGUICatalogItem(),
		razvilkaCatalogItem(),
		keeneticMCPCatalogItem(),
	}
}

func auditedPublisher(name, url string) catalogPublisher {
	return catalogPublisher{ID: "upstream", Name: name, URL: url}
}

func auditedTrust(note string) catalogTrust {
	return catalogTrust{
		Status:     "verified",
		ReviewedBy: "routerforge",
		Note:       note,
	}
}

func keeneticPolicyUICatalogItem() catalogItem {
	return catalogItem{
		ID:           "keenetic-policy-ui",
		Kind:         "integration",
		Name:         "Keenetic Policy UI",
		Category:     "Routing",
		Description:  "Entware Web UI for assigning Keenetic/Netcraze connection policies and DNS profiles per device.",
		ProjectURL:   "https://github.com/JohnDoe150489/keenetic-policy-ui",
		Source:       "project-official",
		Publisher:    auditedPublisher("JohnDoe150489", "https://github.com/JohnDoe150489/keenetic-policy-ui"),
		Trust:        auditedTrust("Lifecycle manually matched against the upstream README on 2026-10-02."),
		Capabilities: []string{"detect", "version", "open-ui", "package-lifecycle", "policy-routing"},
		Detection: catalogDetection{
			Packages: []string{"keenetic-policy-ui"},
			Paths:    []string{"/opt/etc/keenetic-policy-ui", "/opt/share/www/keenetic-policy-ui"},
		},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   3000,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic / Netcraze", "Entware", "ca-certificates", "wget-ssl"},
		},
		Install: catalogInstallPlan{
			Method:   "structured",
			Packages: []string{"keenetic-policy-ui"},
			Notes:    []string{"Uses the upstream HTTPS opkg feed documented by the project."},
			Steps: []catalogLifecycleStep{
				{Type: "opkg-update"},
				{Type: "opkg-install", Packages: []string{"ca-certificates", "wget-ssl"}},
				{
					Type:    "write-opkg-feed",
					Path:    "/opt/etc/opkg/keenetic-policy-ui.conf",
					Content: "src/gz keenetic_policy_ui https://johndoe150489.github.io/keenetic-policy-ui",
				},
				{Type: "opkg-update"},
				{Type: "opkg-install", Packages: []string{"keenetic-policy-ui"}},
			},
		},
		Update: catalogInstallPlan{
			Method:   "structured",
			Packages: []string{"keenetic-policy-ui"},
			Steps: []catalogLifecycleStep{
				{Type: "opkg-update"},
				{Type: "opkg-upgrade", Packages: []string{"keenetic-policy-ui"}},
			},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"keenetic-policy-ui"},
			Notes:    []string{"Removes the package; project configuration cleanup remains an explicit manual choice."},
		},
	}
}

func entwareManagerCatalogItem() catalogItem {
	return catalogItem{
		ID:           "entware-manager",
		Kind:         "integration",
		Name:         "Entware Manager",
		Category:     "Administration",
		Description:  "Web panel for Entware package/service/process/log/file administration, monitoring, SMART and terminal access.",
		ProjectURL:   "https://github.com/Di1r1/entware-manager",
		Source:       "project-official",
		Publisher:    auditedPublisher("Di1r1", "https://github.com/Di1r1/entware-manager"),
		Trust:        auditedTrust("Upstream release/IPK lifecycle and default Web UI were reviewed on 2026-10-02; automatic install stays disabled until RouterForge can verify release assets by checksum."),
		Capabilities: []string{"detect", "version", "service-status", "open-ui", "install-preview", "entware-management"},
		Detection: catalogDetection{
			Packages: []string{"entware-manager"},
			Paths:    []string{"/opt/web_entware", "/opt/etc/entware-manager.conf"},
		},
		ProcessNames: []string{"entware-server"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   8087,
			Path:   "/entware-manager/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "mips-3.4", "mipsel-3.4"},
			Hints:   []string{"Keenetic / Netcraze", "Entware", "lighttpd/jq/curl/ttyd dependencies managed by upstream IPK"},
		},
		Install: catalogInstallPlan{
			Method:      "release-deploy",
			Repository:  "https://github.com/Di1r1/entware-manager/releases",
			Packages:    []string{"entware-manager"},
			PreviewOnly: true,
			Notes: []string{
				"Upstream publishes architecture-specific IPK assets and tar.gz bundles.",
				"RouterForge does not execute upstream install.sh or download an unpinned latest asset automatically.",
			},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"entware-manager"},
			Notes:    []string{"Safe only for installations tracked by opkg."},
		},
	}
}

func zapretGUICatalogItem() catalogItem {
	return catalogItem{
		ID:           "zapret-gui",
		Kind:         "integration",
		Name:         "Zapret Web-GUI",
		Category:     "DPI / Bypass",
		Description:  "Web UI for nfqws2/zapret2, VPN/tunnel helpers and routing on Keenetic/Entware and OpenWrt.",
		ProjectURL:   "https://github.com/avatarDD/zapret-gui",
		Source:       "project-official",
		Publisher:    auditedPublisher("avatarDD", "https://github.com/avatarDD/zapret-gui"),
		Trust:        auditedTrust("Keenetic IPK lifecycle, service path and default Web UI were reviewed against upstream documentation on 2026-10-02."),
		Capabilities: []string{"detect", "service-status", "open-ui", "install-preview", "nfqws2", "routing"},
		Detection: catalogDetection{
			Packages: []string{"zapret-gui"},
			Services: []string{"/opt/etc/init.d/S99zapret-gui"},
			Paths:    []string{"/opt/share/zapret-gui", "/opt/etc/zapret-gui"},
		},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   8080,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "mips-3.4", "mipsel-3.4"},
			Hints:   []string{"Keenetic", "Entware", "Python 3.11+", "nfqws2/tunnel components are optional and managed separately"},
		},
		Install: catalogInstallPlan{
			Method:      "release-deploy",
			Repository:  "https://github.com/avatarDD/zapret-gui/releases",
			Packages:    []string{"zapret-gui"},
			PreviewOnly: true,
			Notes: []string{
				"Upstream recommends zapret-gui-keenetic.ipk from GitHub Releases.",
				"Automatic latest-asset download remains disabled until RouterForge has pinned checksum metadata.",
			},
		},
		Update: catalogInstallPlan{
			Method:      "manual",
			Packages:    []string{"zapret-gui"},
			PreviewOnly: true,
			Notes:       []string{"Upstream documents opkg upgrade for package-managed installations."},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"zapret-gui"},
			Notes:    []string{"Applies to IPK-managed installations; script installs have their own uninstall path."},
		},
	}
}

func razvilkaCatalogItem() catalogItem {
	return catalogItem{
		ID:           "razvilka",
		Kind:         "integration",
		Name:         "RAZVILKA",
		Category:     "VPN / Routing",
		Description:  "Local Keenetic/Netcraze panel that evaluates available bypass methods and prepares routing plans for selected services/devices.",
		ProjectURL:   "https://github.com/ArtixSx/RAZVILKA",
		Source:       "project-official",
		Publisher:    auditedPublisher("ArtixSx", "https://github.com/ArtixSx/RAZVILKA"),
		Trust:        auditedTrust("Architecture support, bootstrap flow, healthcheck and default Web UI were reviewed against upstream documentation on 2026-10-02."),
		Capabilities: []string{"detect", "version", "service-status", "open-ui", "install-preview", "routing"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99razvilka"},
			Paths:    []string{"/opt/bin/razvilka"},
		},
		ProcessNames: []string{"razvilka"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   8787,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "mips-3.4", "mipsel-3.4"},
			Hints:   []string{"Keenetic / Netcraze", "Entware", "curl or wget-ssl", "ca-certificates", "coreutils-sha256sum", "tar"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/ArtixSx/RAZVILKA/main/scripts/bootstrap.sh",
			PreviewOnly:  true,
			Notes: []string{
				"Upstream bootstrap verifies release SHA-256 and performs backup/health checks.",
				"RouterForge never pipes or auto-executes upstream shell installers.",
			},
		},
	}
}

func keeneticMCPCatalogItem() catalogItem {
	return catalogItem{
		ID:           "keenetic-mcp",
		Kind:         "integration",
		Name:         "Keenetic MCP Server",
		Category:     "Automation",
		Description:  "MCP/HTTP server for monitoring and managing Keenetic through RCI, with dry-run writes, watcher rules and backups.",
		ProjectURL:   "https://github.com/st412m/keenetic-mcp",
		Source:       "community",
		Publisher:    auditedPublisher("st412m", "https://github.com/st412m/keenetic-mcp"),
		Trust:        auditedTrust("Entware/Python requirements and installation model were reviewed on 2026-10-02; git-clone deployment remains manual."),
		Capabilities: []string{"detect", "service-status", "install-preview", "mcp", "rci"},
		Detection: catalogDetection{
			Paths: []string{"/opt/keenetic-mcp/server.py", "/opt/keenetic-mcp/.env"},
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"mips-3.4", "mipsel-3.4"},
			Hints:   []string{"Keenetic", "Entware", "Python 3", "git-http", "project documents MIPS/MIPSel models; ARM64 support is not claimed in the audited README"},
		},
		Install: catalogInstallPlan{
			Method:      "manual",
			PreviewOnly: true,
			Notes: []string{
				"Upstream installs by git clone plus explicit .env configuration.",
				"RouterForge does not clone repositories or handle router/MCP credentials automatically.",
			},
		},
	}
}
