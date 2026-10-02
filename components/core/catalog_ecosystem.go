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
		antiGoblinCatalogItem(),
		xkeenPanelCatalogItem(),
		keenSnapCatalogItem(),
		sms2gramCatalogItem(),
		web4staticCatalogItem(),
		magiTrickleCatalogItem(),
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

func antiGoblinCatalogItem() catalogItem {
	return catalogItem{
		ID:           "antigoblin",
		Kind:         "integration",
		Name:         "AntiGoblin",
		Category:     "VPN / Routing",
		Description:  "Keenetic/Entware routing panel for XKeen/Xray and sing-box with per-device policies, subscriptions and Web UI.",
		ProjectURL:   "https://github.com/MaksimSamarin/AntiGoblin",
		Source:       "project-official",
		Publisher:    auditedPublisher("MaksimSamarin", "https://github.com/MaksimSamarin/AntiGoblin"),
		Trust:        auditedTrust("Official on-router installer and Web UI defaults were reviewed against upstream README/install.sh on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "routing", "xray", "sing-box"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99antigoblin"},
			Paths:    []string{"/opt/etc/antigoblin.conf", "/opt/share/xkeen-manager"},
		},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   8899,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic / Netcraze", "Entware", "Netfilter/Xtables", "Xray and sing-box are deployed by upstream"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/MaksimSamarin/AntiGoblin/main/install.sh",
			Notes: []string{
				"Uses the upstream one-command on-router installer.",
				"RouterForge downloads the script first, records SHA256 and streams output instead of piping curl to sh.",
			},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/MaksimSamarin/AntiGoblin/main/install.sh",
			Notes:        []string{"Repeats the upstream installer contract for in-place refresh/update."},
		},
	}
}

func xkeenPanelCatalogItem() catalogItem {
	return catalogItem{
		ID:           "xkeen-panel",
		Kind:         "integration",
		Name:         "XKeen Panel",
		Category:     "VPN / Routing",
		Description:  "Web panel for XKeen with routing, subscriptions and release-aware updates on Keenetic.",
		ProjectURL:   "https://github.com/Dearonski/xkeen-panel",
		Source:       "project-official",
		Publisher:    auditedPublisher("Dearonski", "https://github.com/Dearonski/xkeen-panel"),
		Trust:        auditedTrust("Official installer and default Web UI were reviewed on 2026-10-03; automatic execution remains disabled because upstream installer accepts architecture arguments and defaults to aarch64."),
		Capabilities: []string{"detect", "service-status", "open-ui", "install-preview", "routing"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99xkeen-panel"},
			Paths:    []string{"/opt/etc/xkeen-panel"},
		},
		ProcessNames: []string{"xkeen-panel"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   3000,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "mips-3.4", "mipsel-3.4"},
			Hints:   []string{"Keenetic", "Entware", "XKeen", "installer architecture must match router"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/Dearonski/xkeen-panel/main/install.sh",
			PreviewOnly:  true,
			Notes: []string{
				"Upstream installer accepts aarch64/mips/mipsel and defaults to aarch64.",
				"RouterForge keeps this preview-only until architecture-specific script arguments are selected dynamically.",
			},
		},
	}
}

func keenSnapCatalogItem() catalogItem {
	return catalogItem{
		ID:           "keensnap",
		Kind:         "integration",
		Name:         "KeenSnap",
		Category:     "Backup",
		Description:  "KeeneticOS/Entware backup utility with Telegram, Google Drive and mounted-storage delivery.",
		ProjectURL:   "https://github.com/spatiumstas/KeenSnap",
		Source:       "project-official",
		Publisher:    auditedPublisher("spatiumstas", "https://github.com/spatiumstas/KeenSnap"),
		Trust:        auditedTrust("Official install script, package name and documented opkg removal were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "version", "package-lifecycle", "backup"},
		Detection: catalogDetection{
			Packages: []string{"keensnap"},
			Paths:    []string{"/opt/var/log/keensnap.log"},
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "curl", "ca-certificates", "wget-ssl"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/spatiumstas/keensnap/main/install.sh",
			Packages:     []string{"keensnap"},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/spatiumstas/keensnap/main/install.sh",
			Packages:     []string{"keensnap"},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"keensnap"},
			Notes:    []string{"Matches upstream documented package removal; shared feed configuration is left intact."},
		},
	}
}

func sms2gramCatalogItem() catalogItem {
	return catalogItem{
		ID:           "sms2gram",
		Kind:         "integration",
		Name:         "sms2gram",
		Category:     "Automation",
		Description:  "Keenetic modem SMS automation for Telegram, VK, ntfy, forwarding and controlled remote commands.",
		ProjectURL:   "https://github.com/spatiumstas/sms2gram",
		Source:       "project-official",
		Publisher:    auditedPublisher("spatiumstas", "https://github.com/spatiumstas/sms2gram"),
		Trust:        auditedTrust("Official install script, package name and documented opkg removal were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "version", "package-lifecycle", "automation", "sms"},
		Detection: catalogDetection{
			Packages: []string{"sms2gram"},
			Paths:    []string{"/opt/root/sms2gram", "/opt/var/log/sms2gram.log"},
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "supported USB modem/NDIS/QMI environment", "curl", "ca-certificates", "wget-ssl"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/spatiumstas/sms2gram/main/install.sh",
			Packages:     []string{"sms2gram"},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/spatiumstas/sms2gram/main/install.sh",
			Packages:     []string{"sms2gram"},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"sms2gram"},
			Notes:    []string{"Matches upstream documented package removal; notification credentials/config cleanup remains explicit."},
		},
	}
}

func web4staticCatalogItem() catalogItem {
	return catalogItem{
		ID:           "web4static",
		Kind:         "integration",
		Name:         "web4static",
		Category:     "Administration",
		Description:  "Web UI for editing and managing configuration of routing, DPI-bypass, VPN and network tools on Keenetic/Entware.",
		ProjectURL:   "https://github.com/spatiumstas/web4static",
		Source:       "project-official",
		Publisher:    auditedPublisher("spatiumstas", "https://github.com/spatiumstas/web4static"),
		Trust:        auditedTrust("Official install script, port 99 Web UI, package name and documented opkg removal were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "version", "open-ui", "package-lifecycle", "configuration"},
		Detection: catalogDetection{
			Packages: []string{"web4static"},
			Paths:    []string{"/opt/etc/lighttpd/conf.d/81-w4s-local.conf"},
		},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   99,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "lighttpd/PHP dependencies are managed by upstream package", "curl", "ca-certificates", "wget-ssl"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/spatiumstas/web4static/main/install.sh",
			Packages:     []string{"web4static"},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/spatiumstas/web4static/main/install.sh",
			Packages:     []string{"web4static"},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"web4static"},
			Notes:    []string{"Matches upstream documented package removal; shared feed configuration is left intact."},
		},
	}
}

func magiTrickleCatalogItem() catalogItem {
	return catalogItem{
		ID:           "magitrickle",
		Kind:         "integration",
		Name:         "MagiTrickle",
		Category:     "Routing",
		Description:  "Domain-aware selective routing package for Keenetic/Entware and OpenWrt.",
		ProjectURL:   "https://github.com/MagiTrickle/MagiTrickle",
		Source:       "project-official",
		Publisher:    auditedPublisher("MagiTrickle", "https://github.com/MagiTrickle/MagiTrickle"),
		Trust:        auditedTrust("Entware package lifecycle and upstream repository bootstrap were reviewed on 2026-10-03; automatic bootstrap remains disabled because upstream documents an HTTP installer endpoint."),
		Capabilities: []string{"detect", "version", "service-status", "install-preview", "routing"},
		Detection: catalogDetection{
			Packages: []string{"magitrickle"},
			Services: []string{"/opt/etc/init.d/S99magitrickle"},
		},
		ProcessNames: []string{"magitrickle"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "upstream repository bootstrap currently documented over HTTP"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "http://bin.magitrickle.dev/packages/add_repo.sh",
			Packages:     []string{"magitrickle"},
			PreviewOnly:  true,
			Notes: []string{
				"Upstream Entware flow adds its repository via HTTP and then installs magitrickle with opkg.",
				"RouterForge will not auto-execute this bootstrap until upstream provides an approved HTTPS source.",
			},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"magitrickle"},
		},
	}
}
