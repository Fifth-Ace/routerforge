package main

// Port Access manages separate upstream Entware services with explicit activation.
// Installing the module never enables a protected service by itself.
func portAccessSeedModules() []catalogItem {
	return []catalogItem{{
		ID: "port-access-manager", Kind: "module", Name: "RouterForge Port Access Manager", Category: "Security",
		Description: "Контроль доступа: odin, knockd и fwknopd; установка и настройка по подтверждению.",
		ProjectURL:  "https://github.com/Fifth-Ace/routerforge", Source: "routerforge-official", Managed: true, PackageAuthoritative: true,
		Publisher:     catalogPublisher{ID: "routerforge", Name: "RouterForge", URL: "https://github.com/Fifth-Ace/routerforge"},
		Trust:         catalogTrust{Status: "official", ReviewedBy: "routerforge", Note: "Opt-in management of existing services; real protection requires hardware acceptance."},
		Capabilities:  []string{"integration-manager", "port-access-status", "knockd-detection", "fwknopd-detection", "managed-gate", "engine-config-wizard"},
		Detection:     catalogDetection{Packages: []string{"routerforge-port-access-manager"}, Services: []string{"/opt/etc/init.d/S95routerforge-port-access-manager"}},
		ProcessNames:  []string{"routerforge-port-access-manager"},
		Compatibility: catalogCompatibility{Status: "requirements", Hints: []string{"RouterForge Core", "Entware", "Keenetic / Netcraze", "Real firewall activation requires explicit confirmation"}, Targets: []string{"aarch64-3.10"}},
		Install:       catalogInstallPlan{Method: "routerforge-release", Repository: "routerforge-dev", Packages: []string{"routerforge-port-access-manager"}, Notes: []string{"Installs the manager only; knockd and fwknopd installation and activation are separate confirmed actions."}},
		Update:        catalogInstallPlan{Method: "routerforge-release", Repository: "routerforge-dev", Packages: []string{"routerforge-port-access-manager"}},
		Remove:        catalogInstallPlan{Method: "opkg", Packages: []string{"routerforge-port-access-manager"}},
		Presentation:  map[string]any{"dashboard": map[string]any{"enabled": false, "priority": 65}, "integration": map[string]any{"enabled": true, "label": "Port Access Manager", "order": 65, "href": "/integrations?open=port-access-manager"}},
	}}
}
