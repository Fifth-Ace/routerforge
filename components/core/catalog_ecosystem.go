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
		susaninCatalogItem(),
		trustTunnelKeeneticCatalogItem(),
		tgWSProxyGoCatalogItem(),
		tgWSProxyRSCatalogItem(),
		aiwayManagerCatalogItem(),
		bird4StaticCatalogItem(),
		ipset4StaticCatalogItem(),
		keeGeoSplitCatalogItem(),
		keeSmartDNSGeoConfCatalogItem(),
		keeSmartDNSRedirectCatalogItem(),
		keeNetCheckCatalogItem(),
		keeWebUICatalogItem(),
		trustTunnelNativeCatalogItem(),
		tgWSKeeneticCatalogItem(),
		wireguardDPIBypassCatalogItem(),
		magiTrickleBadigitCatalogItem(),
		magiTrickleLarinCatalogItem(),
		xkeenUIFan92CatalogItem(),
		dropwebXKeenCatalogItem(),
		wdttServerEntwareCatalogItem(),
		netcrazeAWG3CatalogItem(),
		b4CatalogItem(),
		hydraBridgeCatalogItem(),
		ssClashGoCatalogItem(),
		broRayCatalogItem(),
		keeneticAutoSetupCatalogItem(),
		qeliKeeneticCatalogItem(),
		xkeenUIUmarchehCatalogItem(),
		keenPBRHeadlessCatalogItem(),
		wayHopCatalogItem(),
		qWDTTKeeneticCatalogItem(),
		detourKeeneticCatalogItem(),
		keeneticXrayAutoCatalogItem(),
		xkeenSmartRouteCatalogItem(),
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
	install := catalogInstallPlan{
		Method:   "verified-ipk-target",
		Packages: []string{"entware-manager"},
		VerifiedIPKTargets: map[string]catalogVerifiedIPKAsset{
			"aarch64-3.10": {
				InstallerURL:   "https://github.com/Di1r1/entware-manager/releases/download/v1.16.27/entware-manager_arm64.ipk",
				ExpectedSHA256: "8707a53c9194b047214d40b85fb3993dd7989abf381cf14360f2feed84836f9e",
			},
			"mips-3.4": {
				InstallerURL:   "https://github.com/Di1r1/entware-manager/releases/download/v1.16.27/entware-manager_mips.ipk",
				ExpectedSHA256: "b87db48f5e7493171b8807836f1027ea744266f14fdaea075f7469ac83b520e3",
			},
			"mipsel-3.4": {
				InstallerURL:   "https://github.com/Di1r1/entware-manager/releases/download/v1.16.27/entware-manager_mipsel.ipk",
				ExpectedSHA256: "14df49ff31cb4a23e90d1b8623358e6bbaf9fff58ec39c1bc456a3188aa78f3e",
			},
		},
		Notes: []string{"Pins the exact v1.16.27 GitHub Release IPK and GitHub-published digest for the detected Entware target."},
	}
	return catalogItem{
		ID:           "entware-manager",
		Kind:         "integration",
		Name:         "Entware Manager",
		Category:     "Administration",
		Description:  "Web panel for Entware package/service/process/log/file administration, monitoring, SMART and terminal access.",
		ProjectURL:   "https://github.com/Di1r1/entware-manager",
		Source:       "project-official",
		Publisher:    auditedPublisher("Di1r1", "https://github.com/Di1r1/entware-manager"),
		Trust:        auditedTrust("Architecture-specific v1.16.27 IPKs, GitHub-published digests and Web UI contract were re-audited on 2026-10-03."),
		Capabilities: []string{"detect", "version", "service-status", "open-ui", "package-lifecycle", "entware-management"},
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
		Install: install,
		Update:  install,
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"entware-manager"},
		},
	}
}

func zapretGUICatalogItem() catalogItem {
	install := catalogInstallPlan{
		Method:         "verified-ipk",
		Packages:       []string{"zapret-gui"},
		InstallerURL:   "https://github.com/avatarDD/zapret-gui/releases/download/v0.25.5/zapret-gui-keenetic.ipk",
		ExpectedSHA256: "76d9b57e911ccc9b4ce03712be983c1559101bed80c3b887b26acb90ec536274",
		Notes: []string{
			"Pins the exact upstream v0.25.5 Keenetic IPK and GitHub-published SHA256 digest.",
			"RouterForge verifies the complete IPK before handing it to opkg.",
		},
	}
	return catalogItem{
		ID:           "zapret-gui",
		Kind:         "integration",
		Name:         "Zapret Web-GUI",
		Category:     "DPI / Bypass",
		Description:  "Web UI for nfqws2/zapret2, VPN/tunnel helpers and routing on Keenetic/Entware and OpenWrt.",
		ProjectURL:   "https://github.com/avatarDD/zapret-gui",
		Source:       "project-official",
		Publisher:    auditedPublisher("avatarDD", "https://github.com/avatarDD/zapret-gui"),
		Trust:        auditedTrust("Keenetic IPK lifecycle, exact v0.25.5 GitHub Release asset digest, service path and default Web UI were re-audited on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "nfqws2", "routing"},
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
		Install: install,
		Update:  install,
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
		Trust:        auditedTrust("Official installer, current architecture auto-detection and default Web UI were re-audited against upstream README/install.sh on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "routing"},
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
			Hints:   []string{"Keenetic", "Entware", "XKeen", "installer auto-detects uname architecture and normalizes arm64/mips/mipsel"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/Dearonski/xkeen-panel/main/install.sh",
			Notes: []string{
				"Current upstream installer auto-detects the router architecture from uname and maps aarch64/arm64, mips and mipsel/mipsle to matching release assets.",
				"Existing config.yaml is preserved during reinstall/update.",
			},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/Dearonski/xkeen-panel/main/install.sh",
			Notes:        []string{"Upstream documents rerunning install.sh as the update path; existing configuration is not overwritten."},
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

func susaninCatalogItem() catalogItem {
	return catalogItem{
		ID:           "susanin-keenetic",
		Kind:         "integration",
		Name:         "Susanin.Keenetic",
		Category:     "VPN / Routing",
		Description:  "Adaptive selective VPN routing for Keenetic/Entware that learns blocked destinations from connection behaviour.",
		ProjectURL:   "https://github.com/R17a/Susanin.Keenetic",
		Source:       "project-official",
		Publisher:    auditedPublisher("R17a", "https://github.com/R17a/Susanin.Keenetic"),
		Trust:        auditedTrust("Official installer, architecture detection and --yes noninteractive mode were reviewed against upstream documentation on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "package-lifecycle", "routing", "wireguard", "amneziawg"},
		Detection: catalogDetection{
			Paths: []string{"/opt/susanin", "/opt/susanin/tools/susanin.sh"},
		},
		ProcessNames: []string{"susanin-agent"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"KeeneticOS 5.x", "Entware", "ipset", "iptables", "conntrack", "working WireGuard/AmneziaWG tunnel"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/R17a/Susanin.Keenetic/main/install.sh",
			Args:         []string{"--yes"},
			Notes:        []string{"Uses upstream noninteractive --yes mode and preserves existing Susanin configuration on update."},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/R17a/Susanin.Keenetic/main/install.sh",
			Args:         []string{"--yes"},
		},
	}
}

func trustTunnelKeeneticCatalogItem() catalogItem {
	return catalogItem{
		ID:           "trusttunnel-keenetic",
		Kind:         "integration",
		Name:         "TrustTunnel Keenetic",
		Category:     "VPN / Routing",
		Description:  "Keenetic/Entware integration for TrustTunnel client with SOCKS5 or TUN modes and Keenetic interface hooks.",
		ProjectURL:   "https://github.com/artemevsevev/TrustTunnel-Keenetic",
		Source:       "project-official",
		Publisher:    auditedPublisher("artemevsevev", "https://github.com/artemevsevev/TrustTunnel-Keenetic"),
		Trust:        auditedTrust("Bootstrap and interactive configure flow were reviewed on 2026-10-03; execution remains preview-only because configuration requires interactive mode/interface choices."),
		Capabilities: []string{"detect", "service-status", "install-preview", "routing", "trusttunnel"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99trusttunnel"},
			Paths:    []string{"/opt/etc/trusttunnel", "/opt/etc/trusttunnel/mode.conf"},
		},
		ProcessNames: []string{"trusttunnel"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "curl", "configured TrustTunnel server", "interactive SOCKS5/TUN setup"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/artemevsevev/TrustTunnel-Keenetic/main/install.sh",
			PreviewOnly:  true,
			Notes:        []string{"Upstream configure.sh is interactive; RouterForge does not guess SOCKS5/TUN mode or interface indices."},
		},
	}
}

func tgWSProxyGoCatalogItem() catalogItem {
	return catalogItem{
		ID:           "tg-ws-proxy-go",
		Kind:         "integration",
		Name:         "TG WS Proxy Go",
		Category:     "Proxy",
		Description:  "Telegram MTProto WebSocket proxy package for Keenetic/Entware.",
		ProjectURL:   "https://github.com/spatiumstas/tg-ws-proxy-go",
		Source:       "project-official",
		Publisher:    auditedPublisher("spatiumstas", "https://github.com/spatiumstas/tg-ws-proxy-go"),
		Trust:        auditedTrust("Keenetic feed/package lifecycle, feedly architecture bootstrap, service path and configuration locations were re-audited on 2026-10-03."),
		Capabilities: []string{"detect", "version", "service-status", "package-lifecycle", "proxy", "telegram"},
		Detection: catalogDetection{
			Packages: []string{"tg-ws-proxy"},
			Services: []string{"/opt/etc/init.d/S99tg-ws-proxy"},
			Paths:    []string{"/opt/etc/tg-ws-proxy/config.conf", "/opt/etc/tg-ws-proxy/secret.conf"},
		},
		ProcessNames: []string{"tg-ws-proxy"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "armv7-3.2", "mips-3.4", "mipsel-3.4"},
			Hints:   []string{"Keenetic", "Entware", "spatiumstas feedly repository"},
		},
		Install: catalogInstallPlan{
			Method:   "structured",
			Packages: []string{"tg-ws-proxy"},
			Notes: []string{
				"Mirrors the reviewed feedly add-repo.sh contract without piping a remote shell script.",
				"RouterForge selects only architectures reported by opkg and supported by upstream feedly.",
			},
			Steps: []catalogLifecycleStep{
				{Type: "opkg-install", Packages: []string{"ca-certificates", "wget-ssl"}},
				{Type: "opkg-remove", Packages: []string{"wget-nossl"}, IgnoreFailure: true},
				{
					Type:    "write-opkg-feed-arch",
					Path:    "/opt/etc/opkg/feedly.conf",
					Content: "src/gz feedly_{arch} https://spatiumstas.github.io/feedly/{arch}",
					Args:    []string{"aarch64-3.10", "armv7-3.2", "mips-3.4", "mipsel-3.4"},
				},
				{Type: "opkg-update"},
				{Type: "opkg-install", Packages: []string{"tg-ws-proxy"}},
			},
		},
		Update: catalogInstallPlan{
			Method:   "structured",
			Packages: []string{"tg-ws-proxy"},
			Steps: []catalogLifecycleStep{
				{Type: "opkg-update"},
				{Type: "opkg-upgrade", Packages: []string{"tg-ws-proxy"}},
			},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"tg-ws-proxy"},
		},
	}
}

func tgWSProxyRSCatalogItem() catalogItem {
	return catalogItem{
		ID:           "tg-ws-proxy-rs",
		Kind:         "integration",
		Name:         "TG WS Proxy Rust",
		Category:     "Proxy",
		Description:  "Static Rust Telegram MTProto WebSocket bridge for Entware/OpenWrt with architecture-aware installer and release checksum verification.",
		ProjectURL:   "https://github.com/valnesfjord/tg-ws-proxy-rs",
		Source:       "project-official",
		Publisher:    auditedPublisher("valnesfjord", "https://github.com/valnesfjord/tg-ws-proxy-rs"),
		Trust:        auditedTrust("Official Entware installer, architecture detection and immutable release checksum verification were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "package-lifecycle", "proxy", "telegram"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99tg-ws-proxy-rs"},
			Paths:    []string{"/opt/bin/tg-ws-proxy-rs"},
		},
		ProcessNames: []string{"tg-ws-proxy-rs"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "armv7-3.2", "mips-3.4", "mipsel-3.4", "x64-3.2"},
			Hints:   []string{"Keenetic / Entware", "supported musl target", "installer verifies release assets against SHA256 metadata"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/valnesfjord/tg-ws-proxy-rs/main/install.sh",
			Notes:        []string{"Uses upstream stable-channel installer; release payload checksum verification is performed by upstream installer."},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/valnesfjord/tg-ws-proxy-rs/main/install.sh",
		},
	}
}

func aiwayManagerCatalogItem() catalogItem {
	return catalogItem{
		ID:           "aiway-manager",
		Kind:         "integration",
		Name:         "AIWAY Manager",
		Category:     "DNS / Routing",
		Description:  "Keenetic panel for AIWAY DNS/SNI routing, VPS profiles and local runtime control.",
		ProjectURL:   "https://github.com/kirniy/aiway",
		Source:       "project-official",
		Publisher:    auditedPublisher("kirniy", "https://github.com/kirniy/aiway"),
		Trust:        auditedTrust("Official Keenetic installer, architecture-aware IPK selection, service path and Web UI were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "version", "service-status", "open-ui", "package-lifecycle", "dns", "routing"},
		Detection: catalogDetection{
			Packages: []string{"aiway-manager"},
			Services: []string{"/opt/etc/init.d/S99aiway-manager"},
			Paths:    []string{"/opt/bin/aiway-manager", "/opt/etc/aiway-manager"},
		},
		ProcessNames: []string{"aiway-manager"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   2233,
			Path:   "/routing",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "armv7-3.2", "x64-3.2", "mips-3.4", "mipsel-3.4"},
			Hints:   []string{"Keenetic", "Entware", "curl or wget", "managed-VPS features additionally require SSH credentials"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/kirniy/aiway/main/router/scripts/install.sh",
			Packages:     []string{"aiway-manager"},
			Notes:        []string{"Upstream installer detects Entware architecture, selects the matching GitHub release IPK and installs it with opkg."},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/kirniy/aiway/main/router/scripts/install.sh",
			Packages:     []string{"aiway-manager"},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"aiway-manager"},
			Notes:    []string{"Removes the router package only; remote VPS state is not touched."},
		},
	}
}

func bird4StaticCatalogItem() catalogItem {
	return catalogItem{
		ID:           "bird4static",
		Kind:         "integration",
		Name:         "Bird4Static",
		Category:     "Routing",
		Description:  "Keenetic/Entware BIRD routing helper for antifilter.download, antifilter.network, re:filter and custom route sources.",
		ProjectURL:   "https://github.com/DennoN-RUS/Bird4Static",
		Source:       "project-official",
		Publisher:    auditedPublisher("DennoN-RUS", "https://github.com/DennoN-RUS/Bird4Static"),
		Trust:        auditedTrust("Git-clone and interactive install flow were reviewed on 2026-10-03; lifecycle remains manual because upstream installer requires interactive choices."),
		Capabilities: []string{"detect", "install-preview", "routing", "bird"},
		Detection: catalogDetection{
			Paths: []string{"/opt/etc/bird4static", "/opt/root/Bird4Static"},
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "git", "git-http", "interactive installer"},
		},
		Install: catalogInstallPlan{
			Method:      "manual",
			PreviewOnly: true,
			Notes:       []string{"Upstream installation clones the repository and runs an interactive install.sh; RouterForge does not guess routing choices."},
		},
	}
}

func ipset4StaticCatalogItem() catalogItem {
	return catalogItem{
		ID:           "ipset4static",
		Kind:         "integration",
		Name:         "IPset4Static",
		Category:     "Routing",
		Description:  "Keenetic/Entware ipset+iptables domain routing helper that can run standalone or alongside Bird4Static.",
		ProjectURL:   "https://github.com/DennoN-RUS/IPset4Static",
		Source:       "project-official",
		Publisher:    auditedPublisher("DennoN-RUS", "https://github.com/DennoN-RUS/IPset4Static"),
		Trust:        auditedTrust("Git-clone installation and AdGuardHome/dnsmasq prerequisite were reviewed on 2026-10-03; lifecycle remains manual because upstream installer is interactive."),
		Capabilities: []string{"detect", "install-preview", "routing", "ipset"},
		Detection: catalogDetection{
			Paths: []string{"/opt/etc/ipset4static", "/opt/root/IPset4Static"},
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "AdGuardHome or dnsmasq", "git", "git-http", "interactive installer"},
		},
		Install: catalogInstallPlan{
			Method:      "manual",
			PreviewOnly: true,
			Notes:       []string{"Upstream installation clones the repository and runs an interactive install.sh after DNS prerequisite setup."},
		},
	}
}

func keePackageCatalogItem(id, name, category, description, pkg string, capabilities []string) catalogItem {
	install := catalogInstallPlan{
		Method:       "official-script",
		InstallerURL: "https://raw.githubusercontent.com/0xkee/keenetic-entware-extras/master/scripts/install.sh",
		Args:         []string{pkg},
		Packages:     []string{pkg},
		Notes: []string{
			"Uses the upstream noninteractive package-selection form of the Keenetic Entware Extras installer.",
			"The upstream script configures its HTTPS opkg feeds and installs the selected package with dependencies.",
		},
	}
	return catalogItem{
		ID:           id,
		Kind:         "integration",
		Name:         name,
		Category:     category,
		Description:  description,
		ProjectURL:   "https://github.com/0xkee/keenetic-entware-extras",
		Source:       "project-official",
		Publisher:    auditedPublisher("0xkee", "https://github.com/0xkee/keenetic-entware-extras"),
		Trust:        auditedTrust("Keenetic Entware Extras package lifecycle and noninteractive package-selection installer were reviewed on 2026-10-03."),
		Capabilities: capabilities,
		Detection: catalogDetection{
			Packages: []string{pkg},
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"KeeneticOS 5.0+", "Entware", "curl", "dependencies resolved by upstream opkg feed"},
		},
		Install: install,
		Update:  install,
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{pkg},
			Notes:    []string{"Removes only the selected package; the shared Keenetic Entware Extras feed remains for other installed components."},
		},
	}
}

func keeGeoSplitCatalogItem() catalogItem {
	return keePackageCatalogItem(
		"kee-geo-split",
		"Keenetic Extras: Geo Split",
		"Routing",
		"Country/region-aware policy routing for Keenetic using GeoIP subnet and domain data.",
		"geo-split",
		[]string{"detect", "version", "package-lifecycle", "routing", "geoip"},
	)
}

func keeSmartDNSGeoConfCatalogItem() catalogItem {
	return keePackageCatalogItem(
		"kee-smartdns-geo-conf",
		"Keenetic Extras: SmartDNS Geo",
		"DNS / Routing",
		"Regional SmartDNS configuration with selectable local/international providers and optional tunnel binding.",
		"smartdns-geo-conf",
		[]string{"detect", "version", "package-lifecycle", "dns", "routing"},
	)
}

func keeSmartDNSRedirectCatalogItem() catalogItem {
	return keePackageCatalogItem(
		"kee-smartdns-redirect",
		"Keenetic Extras: SmartDNS Redirect",
		"DNS / Routing",
		"LAN DNS interception/redirect helper for local SmartDNS, AdGuard Home or Unbound on Keenetic.",
		"smartdns-redirect",
		[]string{"detect", "version", "package-lifecycle", "dns", "iptables"},
	)
}

func keeNetCheckCatalogItem() catalogItem {
	return keePackageCatalogItem(
		"kee-net-check",
		"Keenetic Extras: Net Check",
		"Diagnostics",
		"Network diagnostic toolkit for egress, DNS, TLS MITM, IPv6 leaks, reachability and interface comparison.",
		"net-check",
		[]string{"detect", "version", "package-lifecycle", "diagnostics", "dns", "tls"},
	)
}

func keeWebUICatalogItem() catalogItem {
	item := keePackageCatalogItem(
		"kee-webui",
		"Keenetic Extras: Web UI",
		"Administration",
		"Web dashboard and configuration editor for the Keenetic Entware Extras package family.",
		"webui",
		[]string{"detect", "version", "open-ui", "package-lifecycle", "configuration"},
	)
	item.Web = &catalogWebMetadata{
		Scheme: "http",
		Port:   8080,
		Path:   "/",
		Mode:   "probe-required",
		Embed:  true,
	}
	return item
}

func trustTunnelNativeCatalogItem() catalogItem {
	return catalogItem{
		ID:           "trusttunnel-keenetic-native",
		Kind:         "integration",
		Name:         "TrustTunnel Keenetic Native",
		Category:     "VPN / Routing",
		Description:  "Extended TrustTunnel integration for Keenetic with native OpkgTun attach mode and dashboard traffic statistics.",
		ProjectURL:   "https://github.com/alex-combine/TrustTunnel-Keenetic-Native",
		Source:       "project-official",
		Publisher:    auditedPublisher("alex-combine", "https://github.com/alex-combine/TrustTunnel-Keenetic-Native"),
		Trust:        auditedTrust("Release installer, attach-mode statistics flow and rollback protections were reviewed on 2026-10-03; install remains preview-only because mode/interface selection is interactive."),
		Capabilities: []string{"detect", "service-status", "install-preview", "routing", "trusttunnel", "statistics"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99trusttunnel"},
			Paths:    []string{"/opt/trusttunnel_client/mode.conf", "/opt/bin/tt-stats"},
		},
		ProcessNames: []string{"trusttunnel_client"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "TrustTunnel server", "interactive SOCKS5/TUN and interface selection", "TUN attach statistics require compatible client/KeeneticOS"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/alex-combine/TrustTunnel-Keenetic-Native/main/install.sh",
			PreviewOnly:  true,
			Notes:        []string{"Upstream installer prompts for mode and Keenetic interface operations; RouterForge does not guess those choices."},
		},
	}
}

func tgWSKeeneticCatalogItem() catalogItem {
	return catalogItem{
		ID:           "tg-ws-keenetic",
		Kind:         "integration",
		Name:         "TG WS Keenetic",
		Category:     "Proxy",
		Description:  "Native Rust Telegram MTProto WebSocket proxy for Keenetic/Entware with built-in lightweight Web panel.",
		ProjectURL:   "https://github.com/Omn1z/tg-ws-keenetic",
		Source:       "project-official",
		Publisher:    auditedPublisher("Omn1z", "https://github.com/Omn1z/tg-ws-keenetic"),
		Trust:        auditedTrust("Installer, release checksum flow, Entware service layout and built-in Web UI were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "proxy", "telegram"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99tgwsproxy"},
			Paths:    []string{"/opt/bin/tgwsproxy", "/opt/etc/tgwsproxy/config.json"},
		},
		ProcessNames: []string{"tgwsproxy"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   1434,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "armv7-3.2", "mips-3.4", "mipsel-3.4", "x64-3.2"},
			Hints:   []string{"Keenetic / Entware", "supported static release architecture", "ca-bundle for TLS downloads"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/Omn1z/tg-ws-keenetic/main/scripts/install.sh",
			Args:         []string{"--system", "entware"},
			Notes:        []string{"Uses upstream installer with explicit Entware mode; installer selects the stable release and verifies release metadata/checksums."},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/Omn1z/tg-ws-keenetic/main/scripts/install.sh",
			Args:         []string{"--system", "entware"},
		},
		Remove: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/Omn1z/tg-ws-keenetic/main/scripts/uninstall.sh",
			Args:         []string{"--system", "entware"},
			Notes:        []string{"Uses upstream uninstall while preserving configuration; destructive --purge is intentionally not exposed."},
		},
	}
}

func wireguardDPIBypassCatalogItem() catalogItem {
	return catalogItem{
		ID:           "wireguard-dpi-bypass",
		Kind:         "integration",
		Name:         "WireGuard DPI Handshake Bypass",
		Category:     "DPI / Bypass",
		Description:  "Keenetic helper that recovers stalled WireGuard/AmneziaWG handshakes by probing and rotating the local listen port.",
		ProjectURL:   "https://github.com/Ground-Zerro/Wireguard-DPI-blocking-bypass",
		Source:       "project-official",
		Publisher:    auditedPublisher("Ground-Zerro", "https://github.com/Ground-Zerro/Wireguard-DPI-blocking-bypass"),
		Trust:        auditedTrust("Official one-command installer and interface-triggered WireGuard/AmneziaWG recovery model were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "package-lifecycle", "dpi-bypass", "wireguard", "amneziawg"},
		Detection: catalogDetection{
			Paths: []string{"/opt/etc/ndm/netfilter.d", "/opt/etc/ndm/wan.d"},
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"KeeneticOS 4.x", "Entware", "curl", "WireGuard or AmneziaWG interfaces"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://github.com/Ground-Zerro/Wireguard-DPI-blocking-bypass/raw/refs/heads/main/install.sh",
			Notes:        []string{"Uses the upstream automatic-mode installer exactly as documented."},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://github.com/Ground-Zerro/Wireguard-DPI-blocking-bypass/raw/refs/heads/main/install.sh",
		},
	}
}

func magiTrickleBadigitCatalogItem() catalogItem {
	return catalogItem{
		ID:           "magitrickle-badigit",
		Kind:         "integration",
		Name:         "MagiTrickle mod_badigit",
		Category:     "Routing",
		Description:  "MagiTrickle fork with redir/TPROXY mode, rule subscriptions, DNS capture and extended routing features.",
		ProjectURL:   "https://github.com/badigit/MagiTrickle_mod_badigit",
		Source:       "project-official",
		Publisher:    auditedPublisher("badigit", "https://github.com/badigit/MagiTrickle_mod_badigit"),
		Trust:        auditedTrust("Official Entware/OpenWrt installer and same-command update flow were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "routing", "tproxy", "dns"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99magitrickle"},
			Paths:    []string{"/opt/var/lib/magitrickle/config.yaml"},
		},
		ProcessNames: []string{"magitrickle"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   8080,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic / Entware", "wget-ssl", "ca-certificates", "transparent proxy features require compatible proxy/netfilter setup"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/badigit/MagiTrickle_mod_badigit/mod_badigit/scripts/install.sh",
			Notes:        []string{"Installer auto-detects Entware/OpenWrt and architecture; the same command is documented for update."},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/badigit/MagiTrickle_mod_badigit/mod_badigit/scripts/install.sh",
		},
	}
}

func magiTrickleLarinCatalogItem() catalogItem {
	return catalogItem{
		ID:           "magitrickle-larin",
		Kind:         "integration",
		Name:         "MagiTrickle Mod",
		Category:     "Routing",
		Description:  "Extended MagiTrickle fork with optimized rule lookup, bulk UI operations, conflict detection and built-in updates.",
		ProjectURL:   "https://github.com/LarinIvan/MagiTrickle_Mod",
		Source:       "project-official",
		Publisher:    auditedPublisher("LarinIvan", "https://github.com/LarinIvan/MagiTrickle_Mod"),
		Trust:        auditedTrust("Official repository bootstrap, Entware install/update flow, Web UI and opkg removal were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "version", "service-status", "open-ui", "package-lifecycle", "routing"},
		Detection: catalogDetection{
			Packages: []string{"magitrickle_mod"},
			Services: []string{"/opt/etc/init.d/S99magitrickle"},
			Paths:    []string{"/opt/var/lib/magitrickle/config.yaml"},
		},
		ProcessNames: []string{"magitrickle"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   8080,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic / Entware", "wget-ssl", "ca-certificates", "original magitrickle package must be removed before installing this fork"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/LarinIvan/MagiTrickle_Mod/develop/add_repo.sh",
			Packages:     []string{"magitrickle_mod"},
			Notes:        []string{"Upstream bootstrap auto-detects Entware/OpenWrt, adds its repository and installs/starts magitrickle_mod."},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/LarinIvan/MagiTrickle_Mod/develop/add_repo.sh",
			Packages:     []string{"magitrickle_mod"},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"magitrickle_mod"},
		},
	}
}

func xkeenUIFan92CatalogItem() catalogItem {
	return catalogItem{
		ID:           "xkeen-ui-fan92",
		Kind:         "integration",
		Name:         "XKEEN-UI (fan92rus)",
		Category:     "VPN / Routing",
		Description:  "Single-binary Web UI for XKeen configuration, logs, commands, Xray/Mihomo switching and optional AmneziaWG management.",
		ProjectURL:   "https://github.com/fan92rus/xkeen-ui",
		Source:       "project-official",
		Publisher:    auditedPublisher("fan92rus", "https://github.com/fan92rus/xkeen-ui"),
		Trust:        auditedTrust("Official setup.sh one-command install, Web UI port and service lifecycle were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "xkeen", "xray", "mihomo", "amneziawg"},
		Detection: catalogDetection{
			Paths: []string{"/opt/bin/xkeen-ui", "/opt/etc/xkeen-ui/config.json"},
		},
		ProcessNames: []string{"xkeen-ui"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   8089,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "mips-3.4", "mipsel-3.4"},
			Hints:   []string{"Keenetic", "Entware", "XKeen installed", "default first-login password must be changed"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/fan92rus/xkeen-ui/master/xkeen-go/scripts/setup.sh",
			Notes:        []string{"Uses the upstream quick-install setup.sh exactly as documented."},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/fan92rus/xkeen-ui/master/xkeen-go/scripts/setup.sh",
		},
	}
}

func dropwebXKeenCatalogItem() catalogItem {
	return catalogItem{
		ID:           "dropweb-xkeen",
		Kind:         "integration",
		Name:         "dropweb-xkeen",
		Category:     "VPN / Routing",
		Description:  "Mihomo/XKeen selective-routing deployment with subscription refresh, watchdog and local Web controls.",
		ProjectURL:   "https://github.com/enkinvsh/dropweb-xkeen",
		Source:       "project-official",
		Publisher:    auditedPublisher("enkinvsh", "https://github.com/enkinvsh/dropweb-xkeen"),
		Trust:        auditedTrust("Official install script, subscription/HWID prompts, Web panel defaults and uninstall path were reviewed on 2026-10-03; install stays preview-only because credentials/subscription input is required."),
		Capabilities: []string{"detect", "service-status", "open-ui", "install-preview", "routing", "mihomo", "xkeen"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S96mihomo-panel", "/opt/etc/init.d/S99xkeen"},
			Paths:    []string{"/opt/etc/mihomo/config.yaml", "/opt/sbin/mihomo-panel.py"},
		},
		ProcessNames: []string{"mihomo"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   8181,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "XKeen", "Mihomo/Clash subscription URL", "optional HWID/device headers"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/enkinvsh/dropweb-xkeen/main/install.sh",
			PreviewOnly:  true,
			Notes:        []string{"Installer prompts for subscription URL, HWID, device metadata and Web port; RouterForge does not collect or guess these secrets yet."},
		},
	}
}

func wdttServerEntwareCatalogItem() catalogItem {
	return catalogItem{
		ID:           "wdtt-server-entware",
		Kind:         "integration",
		Name:         "WDTT Server Entware",
		Category:     "VPN / Routing",
		Description:  "Entware server deployment for WDTT with architecture-specific binaries, init service and firewall/NAT integration.",
		ProjectURL:   "https://github.com/kkvoru/wdtt-server-entware",
		Source:       "project-official",
		Publisher:    auditedPublisher("kkvoru", "https://github.com/kkvoru/wdtt-server-entware"),
		Trust:        auditedTrust("Architecture-specific release bundle, installer, service layout and uninstall command were reviewed on 2026-10-03; automatic install remains disabled because upstream setup is interactive and bundle-local."),
		Capabilities: []string{"detect", "service-status", "install-preview", "vpn", "wireguard"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99wdtt"},
			Paths:    []string{"/opt/bin/wdtt-server", "/opt/etc/wdtt/wdtt.env"},
		},
		ProcessNames: []string{"wdtt-server"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic / Entware", "interactive firewall/NAT setup", "matching architecture binary and installer must be staged together"},
		},
		Install: catalogInstallPlan{
			Method:      "manual",
			PreviewOnly: true,
			Notes:       []string{"Upstream installer prompts for confirmation/network choices and is designed to run beside architecture-specific payload files."},
		},
	}
}

func netcrazeAWG3CatalogItem() catalogItem {
	return catalogItem{
		ID:           "netcraze-giga-awg3",
		Kind:         "integration",
		Name:         "Netcraze Giga AWG3",
		Category:     "VPN / Routing",
		Description:  "Hardware-specific AmneziaWG v3 kernel/module integration for Keenetic/Netcraze Giga KN-1012 / MT7981.",
		ProjectURL:   "https://github.com/Sergekkk/netcraze-giga-awg3.1",
		Source:       "community",
		Publisher:    auditedPublisher("Sergekkk", "https://github.com/Sergekkk/netcraze-giga-awg3.1"),
		Trust:        auditedTrust("Hardware/kernel requirements, local router installer, service and watchdog layout were reviewed on 2026-10-03; deployment stays manual because the prebuilt kernel module is device/firmware specific."),
		Capabilities: []string{"detect", "service-status", "install-preview", "amneziawg", "kernel-module", "routing"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99awg3", "/opt/etc/init.d/S100awg3-watchdog"},
			Paths:    []string{"/opt/etc/awg3", "/opt/bin/awg3-split-config"},
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10"},
			Hints:   []string{"Keenetic/Netcraze Giga KN-1012 / NC-1012", "MT7981", "KeeneticOS kernel 4.9.337-ndm-5 compatible build", "Entware"},
		},
		Install: catalogInstallPlan{
			Method:      "manual",
			PreviewOnly: true,
			Notes:       []string{"Upstream quick start copies router/ plus prebuilt KN-1012 kernel/module payloads and runs the local install.sh; RouterForge must not apply it to generic ARM64 devices."},
		},
	}
}

func b4CatalogItem() catalogItem {
	return catalogItem{
		ID:           "b4",
		Kind:         "integration",
		Name:         "b4",
		Category:     "DPI / Bypass",
		Description:  "Keenetic/Entware traffic routing and DPI-bypass platform with rule sets, transparent modes and Telegram transport features.",
		ProjectURL:   "https://github.com/DanielLavrushin/b4",
		Source:       "project-official",
		Publisher:    auditedPublisher("DanielLavrushin", "https://github.com/DanielLavrushin/b4"),
		Trust:        auditedTrust("Official Keenetic installer, Entware paths, architecture detection and NDMS netfilter hook were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "package-lifecycle", "routing", "dpi-bypass", "telegram"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99b4"},
			Paths:    []string{"/opt/sbin/b4", "/opt/etc/b4/b4.json"},
		},
		ProcessNames: []string{"b4"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "mipsel-3.4"},
			Hints:   []string{"Keenetic", "Entware", "Netfilter", "Xtables-addons/xt_connbytes", "iptables"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/DanielLavrushin/b4/main/install.sh",
			Notes:        []string{"Uses the exact one-command Keenetic installer documented upstream; architecture is detected by upstream."},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/DanielLavrushin/b4/main/install.sh",
		},
	}
}

func hydraBridgeCatalogItem() catalogItem {
	return catalogItem{
		ID:           "hydra-bridge",
		Kind:         "integration",
		Name:         "HydraBridge",
		Category:     "Automation",
		Description:  "Authenticated HTTP control-plane API for HydraRoute Neo configuration, diagnostics, backup and lifecycle operations.",
		ProjectURL:   "https://github.com/astronaut808/hrbridge",
		Source:       "project-official",
		Publisher:    auditedPublisher("astronaut808", "https://github.com/astronaut808/hrbridge"),
		Trust:        auditedTrust("Official Keenetic/Entware installer, package feed, service path, API port and bounded HR Neo scope were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "version", "service-status", "package-lifecycle", "api", "hydraroute"},
		Detection: catalogDetection{
			Packages: []string{"hrbridge"},
			Services: []string{"/opt/etc/init.d/S99hrbridge"},
			Paths:    []string{"/opt/etc/hrbridge/hrbridge", "/opt/etc/hrbridge/hrbridge.conf"},
		},
		ProcessNames: []string{"hrbridge"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   2080,
			Path:   "/api/v1/health",
			Mode:   "probe-required",
			Embed:  false,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "mipsel-3.4", "mips-3.4"},
			Hints:   []string{"Keenetic", "Entware", "HydraRoute Neo 3.11.0-1", "bearer token generated on first launch"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://astronaut808.github.io/hrbridge/keenetic/install.sh",
			Packages:     []string{"hrbridge"},
			Notes:        []string{"Upstream installer detects Entware architecture, adds the official feed, installs hrbridge and starts S99hrbridge."},
		},
		Update: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"hrbridge"},
			Notes:    []string{"Uses the installed upstream feed; restart is handled separately by the package/service lifecycle."},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"hrbridge"},
			Notes:    []string{"Removes the package only; HR Neo configuration under /opt/etc/HydraRoute is not owned by HydraBridge."},
		},
	}
}

func ssClashGoCatalogItem() catalogItem {
	return catalogItem{
		ID:           "ssclash-go",
		Kind:         "integration",
		Name:         "SSClash-Go",
		Category:     "VPN / Routing",
		Description:  "Self-contained Mihomo control plane for Keenetic/Entware with embedded Web UI, selective routing and netfilter integration.",
		ProjectURL:   "https://github.com/zerolabnet/SSClash-Go",
		Source:       "project-official",
		Publisher:    auditedPublisher("zerolabnet", "https://github.com/zerolabnet/SSClash-Go"),
		Trust:        auditedTrust("Official Keenetic installer, Entware service, Web UI defaults and netfilter requirements were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "routing", "mihomo", "tproxy", "tun"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99ssclash"},
			Paths:    []string{"/opt/clash/bin/ssclash", "/opt/clash/config.yaml"},
		},
		ProcessNames: []string{"ssclash"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   9091,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "wget-ssl", "ca-certificates", "Netfilter kernel modules", "xtables iptables", "ip-full"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://github.com/zerolabnet/SSClash-Go/raw/refs/heads/main/install-ssclash-go.sh",
			Notes:        []string{"Uses the upstream Keenetic-aware installer; defaults to Web UI port 9091 and HYBRID mode."},
		},
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://github.com/zerolabnet/SSClash-Go/raw/refs/heads/main/install-ssclash-go.sh",
		},
	}
}

func broRayCatalogItem() catalogItem {
	return catalogItem{
		ID:           "broray",
		Kind:         "integration",
		Name:         "BROray",
		Category:     "VPN / Routing",
		Description:  "Keenetic Xray client and network-policy manager with subscriptions, routing, DNS-over-TLS and authenticated Web UI.",
		ProjectURL:   "https://github.com/BROadmin/BROray",
		Source:       "project-official",
		Publisher:    auditedPublisher("BROadmin", "https://github.com/BROadmin/BROray"),
		Trust:        auditedTrust("Stable installer contract, signed release index, exact installer SHA256, Web UI and aarch64-only compatibility were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "xray", "routing", "subscriptions", "dns"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S24broray", "/opt/etc/init.d/S25broray-web"},
			Paths:    []string{"/opt/broray"},
		},
		ProcessNames: []string{"broray"},
		Web: &catalogWebMetadata{
			Scheme: "http",
			Port:   8080,
			Path:   "/",
			Mode:   "probe-required",
			Embed:  true,
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10"},
			Hints:   []string{"Keenetic", "Entware aarch64-3.10", "proxy/opkg/ndns components", "curl", "jq", "sufficient /opt and /tmp free space"},
		},
		Install: catalogInstallPlan{
			Method:         "official-script",
			InstallerURL:   "https://api.brovibe.cloud/releases/stable/broray/3.1.1-r12/INSTALL-ON-ROUTER.sh",
			ExpectedSHA256: "ac334c4f3ce16e9119dcc3b84e21bd076ba5fce5252cf0f6aef2df5510edba1a",
			Notes: []string{
				"RouterForge verifies the exact upstream-published SHA256 before executing the Stable installer.",
				"The installer then validates BROray signed release metadata and its own target payloads.",
			},
		},
		Update: catalogInstallPlan{
			Method:         "official-script",
			InstallerURL:   "https://api.brovibe.cloud/releases/stable/broray/3.1.1-r12/INSTALL-ON-ROUTER.sh",
			ExpectedSHA256: "ac334c4f3ce16e9119dcc3b84e21bd076ba5fce5252cf0f6aef2df5510edba1a",
			Notes:          []string{"Uses the same upstream Stable installer contract; installed Xray is preserved by BROray updater semantics."},
		},
	}
}

func keeneticAutoSetupCatalogItem() catalogItem {
	return catalogItem{
		ID:           "keenetic-auto-setup",
		Kind:         "integration",
		Name:         "Keenetic Auto-Setup Suite",
		Category:     "VPN / Routing",
		Description:  "Opinionated Keenetic/Entware Mihomo + MagiTrickle deployment with storage/RAM gates, routing integration, watchdog and diagnostics.",
		ProjectURL:   "https://github.com/saymer-alt/keenetic-auto-setup",
		Source:       "project-official",
		Publisher:    auditedPublisher("saymer-alt", "https://github.com/saymer-alt/keenetic-auto-setup"),
		Trust:        auditedTrust("Stable setup/install flow, resource gates, Mihomo service layout and post-install interactive config import were reviewed on 2026-10-03; install remains preview-only because setup enters an interactive YAML import stage."),
		Capabilities: []string{"detect", "service-status", "install-preview", "routing", "mihomo", "magitrickle", "watchdog"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99mihomo"},
			Paths:    []string{"/opt/etc/mihomo/config.yaml", "/opt/etc/cron.5mins/mihomo_watchdog"},
		},
		ProcessNames: []string{"mihomo"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "required KeeneticOS proxy/dns-filter/netfilter components", "supported storage/RAM/swap profile", "interactive Mihomo YAML import after bootstrap"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/saymer-alt/keenetic-auto-setup/stable/setup.sh",
			PreviewOnly:  true,
			Notes:        []string{"The bootstrap is safety-gated but intentionally interactive after installation; RouterForge does not synthesize or paste a Mihomo config on the user's behalf."},
		},
	}
}

func qeliKeeneticCatalogItem() catalogItem {
	return catalogItem{
		ID:           "qeli-keenetic",
		Kind:         "integration",
		Name:         "Qeli for Keenetic",
		Category:     "VPN / Routing",
		Description:  "Qeli client deployment for Keenetic/Entware with TUN gateway mode, optional split routing and native OpkgTun integration.",
		ProjectURL:   "https://github.com/litvinovtd/qeli",
		Source:       "project-official",
		Publisher:    auditedPublisher("litvinovtd", "https://github.com/litvinovtd/qeli"),
		Trust:        auditedTrust("Keenetic deployment guide, architecture-specific client bundle, TUN requirements and service layout were reviewed on 2026-10-03; installation remains preview-only because server-issued credentials and interface choices must be supplied by the user."),
		Capabilities: []string{"detect", "service-status", "install-preview", "vpn", "tun", "routing", "opkgtun"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99qeli"},
			Paths:    []string{"/opt/bin/qeli-client", "/opt/etc/qeli/client.conf"},
		},
		ProcessNames: []string{"qeli-client"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "mipsel-3.4"},
			Hints:   []string{"Keenetic", "Entware", "KeeneticOS VPN component providing /dev/net/tun", "ip-full", "iptables", "matching Qeli server credentials"},
		},
		Install: catalogInstallPlan{
			Method:      "manual",
			PreviewOnly: true,
			Notes: []string{
				"Upstream Keenetic bundle chooses the architecture and installs S99qeli, but requires a qeli server profile, credentials and server public-key policy.",
				"RouterForge exposes the project without inventing VPN credentials, LAN interface names or full-tunnel/split-tunnel choices.",
			},
		},
	}
}

func xkeenUIUmarchehCatalogItem() catalogItem {
	return catalogItem{
		ID:           "xkeen-ui-umarcheh",
		Kind:         "integration",
		Name:         "Xkeen UI (umarcheh001)",
		Category:     "VPN / Routing",
		Description:  "Feature-rich XKeen/Xray/Mihomo Web UI with subscriptions, PTY, file manager, DAT tools, diagnostics and optional Android companion.",
		ProjectURL:   "https://github.com/umarcheh001/Xkeen-UI",
		Source:       "project-official",
		Publisher:    auditedPublisher("umarcheh001", "https://github.com/umarcheh001/Xkeen-UI"),
		Trust:        auditedTrust("Release archive install flow, init-script ownership guard, optional components and dynamic Web port selection were reviewed on 2026-10-03; installation remains preview-only because upstream install.sh is bundle-local and depends on release archive contents."),
		Capabilities: []string{"detect", "service-status", "open-ui", "install-preview", "xkeen", "xray", "mihomo", "subscriptions", "diagnostics"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99xkeen-ui-umarcheh001", "/opt/etc/init.d/S99xkeen-ui"},
			Paths:    []string{"/opt/etc/xkeen-ui/run_server.py", "/opt/etc/xkeen-ui/app.py"},
		},
		ProcessNames: []string{"python3"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "mipsel-3.4"},
			Hints:   []string{"Keenetic", "Entware", "Python 3", "XKeen", "release archive xkeen-ui-routing.tar.gz", "Web port auto-selects 8088, then 8091, then 8100-8199"},
		},
		Install: catalogInstallPlan{
			Method:      "manual",
			PreviewOnly: true,
			Notes: []string{
				"Upstream online install downloads xkeen-ui-routing.tar.gz, extracts it under /opt and runs the bundled xkeen-ui/install.sh.",
				"RouterForge does not execute the raw repository install.sh by itself because it relies on sibling files from the release archive.",
			},
		},
	}
}

func keenPBRStructuredPlan(pkg string) catalogInstallPlan {
	return catalogInstallPlan{
		Method:   "structured",
		Packages: []string{pkg},
		Notes: []string{
			"Uses the upstream stable Keenetic/NetCraze package feed with runtime architecture selection.",
			"Upstream postinst is noninteractive when RouterForge runs it without a TTY; existing dnsmasq.conf is left unchanged unless explicitly requested by the user outside RouterForge.",
		},
		Steps: []catalogLifecycleStep{
			{
				Type:    "write-opkg-feed-arch",
				Path:    "/opt/etc/opkg/keen-pbr.conf",
				Content: "src/gz keen_pbr_{arch} https://repo.keen-pbr.fyi/repository/stable/keenetic/current/{arch}",
				Args:    []string{"aarch64-3.10", "mips-3.4", "mipsel-3.4", "armv7-3.2", "x64-3.2"},
			},
			{Type: "opkg-update"},
			{Type: "opkg-install", Packages: []string{pkg}},
		},
	}
}

func keenPBRHeadlessCatalogItem() catalogItem {
	install := keenPBRStructuredPlan("keen-pbr-headless")
	return catalogItem{
		ID:           "keen-pbr-headless",
		Kind:         "integration",
		Name:         "keen-pbr Headless",
		Category:     "Routing",
		Description:  "Headless keen-pbr policy-routing daemon for Keenetic/NetCraze without the API server and Web UI.",
		ProjectURL:   "https://github.com/maksimkurb/keen-pbr",
		Source:       "project-official",
		Publisher:    auditedPublisher("maksimkurb", "https://github.com/maksimkurb/keen-pbr"),
		Trust:        auditedTrust("Keenetic/NetCraze stable repository layout, supported Entware architectures, headless package and non-TTY postinst behavior were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "version", "service-status", "package-lifecycle", "policy-routing"},
		Detection: catalogDetection{
			Packages: []string{"keen-pbr-headless"},
			Services: []string{"/opt/etc/init.d/S80keen-pbr"},
			Paths:    []string{"/opt/etc/keen-pbr/config.json"},
		},
		ProcessNames: []string{"keen-pbr"},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "mips-3.4", "mipsel-3.4", "armv7-3.2", "x64-3.2"},
			Hints:   []string{"Keenetic / NetCraze", "Entware", "Netfilter subsystem", "Xtables-addons", "dnsmasq integration may require explicit user configuration"},
		},
		Install: install,
		Update: catalogInstallPlan{
			Method:   "structured",
			Packages: []string{"keen-pbr-headless"},
			Steps: []catalogLifecycleStep{
				{Type: "opkg-update"},
				{Type: "opkg-upgrade", Packages: []string{"keen-pbr-headless"}},
			},
		},
		Remove: catalogInstallPlan{
			Method:   "opkg",
			Packages: []string{"keen-pbr-headless"},
		},
	}
}

func wayHopCatalogItem() catalogItem {
	install := catalogInstallPlan{
		Method:   "verified-ipk-target",
		Packages: []string{"wayhop"},
		VerifiedIPKTargets: map[string]catalogVerifiedIPKAsset{
			"aarch64-3.10": {
				InstallerURL:   "https://github.com/awadak3davra/wayhop/releases/download/v0.5.7/wayhop_0.5.7-1_aarch64-3.10.ipk",
				ExpectedSHA256: "2580adc6868607960e32484cdd9089654ef0468e7bc1100fa68293e15c2152e5",
			},
			"armv7-3.2": {
				InstallerURL:   "https://github.com/awadak3davra/wayhop/releases/download/v0.5.7/wayhop_0.5.7-1_armv7sf-k3.2.ipk",
				ExpectedSHA256: "16a92d01690f62e0254827393a85d26092674197be0e590044c2152c999fac27",
			},
			"mips-3.4": {
				InstallerURL:   "https://github.com/awadak3davra/wayhop/releases/download/v0.5.7/wayhop_0.5.7-1_mips-3.4.ipk",
				ExpectedSHA256: "a1b289e09297438e9c15fee2e8b8279b399c70a0efc2eda2fff568428e0c696f",
			},
			"mipsel-3.4": {
				InstallerURL:   "https://github.com/awadak3davra/wayhop/releases/download/v0.5.7/wayhop_0.5.7-1_mipselsf-k3.4.ipk",
				ExpectedSHA256: "79af1f7dda8fc5ac16ce8e5747802aa285af5f160b414cdf213dade477e24149",
			},
		},
		Notes: []string{"Pins WayHop v0.5.7 Entware IPKs to GitHub-published digests for each supported target."},
	}
	return catalogItem{
		ID:           "wayhop",
		Kind:         "integration",
		Name:         "WayHop",
		Category:     "VPN / Routing",
		Description:  "Router Web panel for VPN/proxy protocols, failover and selective routing with reversible apply/rollback.",
		ProjectURL:   "https://github.com/awadak3davra/wayhop",
		Source:       "project-official",
		Publisher:    auditedPublisher("awadak3davra", "https://github.com/awadak3davra/wayhop"),
		Trust:        auditedTrust("Keenetic/Entware package targets, v0.5.7 IPKs, GitHub digests, service path and Web UI were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "routing", "sing-box", "failover"},
		Detection: catalogDetection{
			Packages: []string{"wayhop"},
			Services: []string{"/opt/etc/init.d/S99wayhop"},
			Paths:    []string{"/opt/etc/wayhop"},
		},
		ProcessNames: []string{"wayhop"},
		Web:          &catalogWebMetadata{Scheme: "http", Port: 8088, Path: "/", Mode: "probe-required", Embed: true},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "armv7-3.2", "mips-3.4", "mipsel-3.4"},
			Hints:   []string{"Keenetic / Entware", "~20 MB free", "ipset/iptables for kernel selective routing"},
		},
		Install: install,
		Update:  install,
		Remove:  catalogInstallPlan{Method: "opkg", Packages: []string{"wayhop"}},
	}
}

func qWDTTKeeneticCatalogItem() catalogItem {
	install := catalogInstallPlan{
		Method:   "verified-ipk-target",
		Packages: []string{"qwdtt"},
		VerifiedIPKTargets: map[string]catalogVerifiedIPKAsset{
			"aarch64-3.10": {
				InstallerURL:   "https://github.com/SemerDevLab/qWDTT_Server_Keenetic/releases/download/qwdtt_0.1.0-42/qwdtt_0.1.0-42_aarch64-3.10-kn.ipk",
				ExpectedSHA256: "e1d7cec69468713d4364426a7197b25f4ef9b5c92b320ec63e5bdf09fb4de6b4",
			},
			"armv7-3.2": {
				InstallerURL:   "https://github.com/SemerDevLab/qWDTT_Server_Keenetic/releases/download/qwdtt_0.1.0-42/qwdtt_0.1.0-42_armv7-3.2-kn.ipk",
				ExpectedSHA256: "07ff4a03b27128a2d2d14cda1aaa9cb197f540b90872e7250d49dacedbf15ad9",
			},
			"mipsel-3.4": {
				InstallerURL:   "https://github.com/SemerDevLab/qWDTT_Server_Keenetic/releases/download/qwdtt_0.1.0-42/qwdtt_0.1.0-42_mipsel-3.4-kn.ipk",
				ExpectedSHA256: "b8ad077bd158e71f18ae7ecd16e557075efabd9b60b522363af7b81768a95010",
			},
		},
		Notes: []string{"Pins qWDTT 0.1.0-42 Keenetic IPKs to GitHub-published digests by target."},
	}
	return catalogItem{
		ID:           "qwdtt-keenetic",
		Kind:         "integration",
		Name:         "qWDTT Server Keenetic",
		Category:     "VPN / Routing",
		Description:  "Protected tunnel server for Keenetic with WireGuard/Raw-IP transports, profiles, firewall and Web control.",
		ProjectURL:   "https://github.com/SemerDevLab/qWDTT_Server_Keenetic",
		Source:       "project-official",
		Publisher:    auditedPublisher("SemerDevLab", "https://github.com/SemerDevLab/qWDTT_Server_Keenetic"),
		Trust:        auditedTrust("0.1.0-42 Keenetic IPKs, GitHub digests, service lifecycle, conffile and Web UI were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "wireguard", "tunnel"},
		Detection: catalogDetection{
			Packages: []string{"qwdtt"},
			Services: []string{"/opt/etc/init.d/S99qwdtt"},
			Paths:    []string{"/opt/etc/qwdtt/config.json"},
		},
		ProcessNames: []string{"qwdtt"},
		Web:          &catalogWebMetadata{Scheme: "http", Port: 3333, Path: "/", Mode: "probe-required", Embed: true},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "armv7-3.2", "mipsel-3.4"},
			Hints:   []string{"Keenetic", "Entware", "wireguard-tools", "iptables", "TUN/WireGuard support"},
		},
		Install: install,
		Update:  install,
		Remove:  catalogInstallPlan{Method: "opkg", Packages: []string{"qwdtt"}},
	}
}

func detourKeeneticCatalogItem() catalogItem {
	install := catalogInstallPlan{
		Method:         "verified-ipk",
		Packages:       []string{"detour-keenetic"},
		InstallerURL:   "https://github.com/varyen/detour/releases/download/v2.3.0/detour-keenetic_2.3.0_all.ipk",
		ExpectedSHA256: "b175ed208ccd4379f7d1a6a76f0574eafdf4004cd60f9f0dec44da316df2ee8f",
		Notes:          []string{"Pins the exact v2.3.0 Keenetic/Entware all-architecture IPK to its GitHub-published digest."},
	}
	return catalogItem{
		ID:           "detour-keenetic",
		Kind:         "integration",
		Name:         "Detour for Keenetic",
		Category:     "VPN / Routing",
		Description:  "Self-hosted routing/DPI-bypass panel with sing-box, tpws, WARP chains and route maps for Keenetic/Entware.",
		ProjectURL:   "https://github.com/varyen/detour",
		Source:       "project-official",
		Publisher:    auditedPublisher("varyen", "https://github.com/varyen/detour"),
		Trust:        auditedTrust("v2.3.0 Keenetic IPK, GitHub digest, Entware install path and Web panel contract were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "routing", "dpi-bypass", "sing-box"},
		Detection: catalogDetection{
			Packages: []string{"detour-keenetic"},
			Paths:    []string{"/opt/etc/detour", "/opt/var/log/detour-install.log"},
		},
		Web: &catalogWebMetadata{Scheme: "http", Port: 8080, Path: "/detour/", Mode: "probe-required", Embed: true},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"all"},
			Hints:   []string{"Keenetic", "Entware", "package bootstraps its own feed and routing helpers"},
		},
		Install: install,
		Update:  install,
		Remove:  catalogInstallPlan{Method: "opkg", Packages: []string{"detour-keenetic"}},
	}
}

func keeneticXrayAutoCatalogItem() catalogItem {
	install := catalogInstallPlan{
		Method:       "official-script",
		InstallerURL: "https://raw.githubusercontent.com/Kuzz007/keenetic_xray_installer/main/xray_vless_failover_auto_latest.sh",
		Args:         []string{"--yes"},
		Notes: []string{
			"Upstream auto-selects Full Go or Minimal Go from /opt free space and existing installation state.",
			"RouterForge forces the documented noninteractive --yes mode; VLESS/subscription configuration remains a user action.",
		},
	}
	return catalogItem{
		ID:           "keenetic-xray-auto",
		Kind:         "integration",
		Name:         "Keenetic Xray VLESS Auto Installer",
		Category:     "VPN / Routing",
		Description:  "Keenetic Xray/VLESS failover stack with automatic Full Go or Minimal Go selection, recovery and diagnostics.",
		ProjectURL:   "https://github.com/kuzzrus/keenetic_xray_installer",
		Source:       "project-official",
		Publisher:    auditedPublisher("kuzzrus", "https://github.com/kuzzrus/keenetic_xray_installer"),
		Trust:        auditedTrust("Current auto-latest selector, --yes mode, Entware requirement and Full/Minimal Go behavior were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "package-lifecycle", "xray", "vless", "failover"},
		Detection: catalogDetection{
			Paths: []string{"/opt/bin/xray-go", "/opt/bin/minimal-go-status", "/opt/etc/xray"},
		},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "mipsel-3.4"},
			Hints:   []string{"Keenetic", "Entware", "Proxy client component", "curl/ca-certificates"},
		},
		Install: install,
		Update: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/Kuzz007/keenetic_xray_installer/main/xray_vless_failover_auto_latest.sh",
			Args:         []string{"--update-only"},
			Notes:        []string{"Uses upstream repair-lite update-only mode and automatically assumes yes."},
		},
	}
}

func xkeenSmartRouteCatalogItem() catalogItem {
	install := catalogInstallPlan{
		Method:       "official-script",
		InstallerURL: "https://raw.githubusercontent.com/LackyCraft/xkeen-smartroute/master/install.sh",
		Notes: []string{
			"Upstream explicitly documents the same idempotent install.sh for Keenetic installation and update.",
			"The installer contains non-TTY fallbacks for its Keenetic dependency prompts.",
		},
	}
	return catalogItem{
		ID:           "xkeen-smartroute",
		Kind:         "integration",
		Name:         "XKeen SmartRoute",
		Category:     "VPN / Routing",
		Description:  "Domain/device/IP selective routing for XKeen/Xray with subscriptions, health-aware server groups and a standalone Web panel.",
		ProjectURL:   "https://github.com/LackyCraft/xkeen-smartroute",
		Source:       "project-official",
		Publisher:    auditedPublisher("LackyCraft", "https://github.com/LackyCraft/xkeen-smartroute"),
		Trust:        auditedTrust("Keenetic install/update contract, non-TTY handling, service paths and v2.3.1 gateway release were reviewed on 2026-10-03."),
		Capabilities: []string{"detect", "service-status", "open-ui", "package-lifecycle", "xray", "subscriptions", "routing"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S98smartroute-gateway"},
			Paths:    []string{"/opt/share/xkeen-smartroute", "/opt/etc/xray/configs/05_routing.smartroute.json"},
		},
		ProcessNames: []string{"smartroute-gateway"},
		Web:          &catalogWebMetadata{Scheme: "http", Port: 1001, Path: "/", Mode: "probe-required", Embed: true},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Targets: []string{"aarch64-3.10", "armv7-3.2", "mips-3.4", "mipsel-3.4", "x64-3.2"},
			Hints:   []string{"KeeneticOS + Entware or OpenWrt", "XKeen/Xray stack", "~25 MB free minimum", "128 MB RAM minimum"},
		},
		Install: install,
		Update:  install,
		Remove: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/LackyCraft/xkeen-smartroute/master/uninstall.sh",
			Notes:        []string{"Uses upstream non-purge uninstall; XKeen, XKeen-UI and Entware remain untouched."},
		},
	}
}
