package main

// Port Access is a read-only preview until the packaging and lifecycle gates pass.
// Its underlying knockd/fwknopd services are managed separately from RouterForge.
func portAccessSeedModules() []catalogItem {
	return []catalogItem{{
		ID: "port-access-manager", Kind: "module", Name: "RouterForge Port Access Manager", Category: "Security",
		Description: "Контроль доступа: обнаружение knockd и fwknopd. Настройка и установка появятся после аппаратных проверок.",
		ProjectURL:  "https://github.com/Fifth-Ace/routerforge", Source: "routerforge-official", Managed: true, PackageAuthoritative: true,
		Publisher:     catalogPublisher{ID: "routerforge", Name: "RouterForge", URL: "https://github.com/Fifth-Ace/routerforge"},
		Trust:         catalogTrust{Status: "official", ReviewedBy: "routerforge", Note: "Read-only module preview. No firewall mutations or daemon installation."},
		Capabilities:  []string{"integration-manager", "port-access-status", "knockd-detection", "fwknopd-detection"},
		Detection:     catalogDetection{Packages: []string{"routerforge-port-access-manager"}, Services: []string{"/opt/etc/init.d/S95routerforge-port-access-manager"}},
		ProcessNames:  []string{"routerforge-port-access-manager"},
		Compatibility: catalogCompatibility{Status: "requirements", Hints: []string{"RouterForge Core", "Entware", "Keenetic / Netcraze", "K2A: preview only"}, Targets: []string{"aarch64-3.10"}},
		Presentation:  map[string]any{"dashboard": map[string]any{"enabled": false, "priority": 65}},
	}}
}
