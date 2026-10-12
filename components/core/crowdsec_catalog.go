package main

// The CrowdSec Manager is a RouterForge module. The external firewall bouncer
// remains independently installed on the Keenetic; LAPI runs off-router.
func crowdsecSeedModules() []catalogItem {
	return []catalogItem{{
		ID: "crowdsec-manager", Kind: "module", Name: "RouterForge CrowdSec Manager", Category: "Integrations",
		Description: "Проверка реальной работы CrowdSec Firewall Bouncer, LAPI, ipset, iptables и NDM, управление через защищённый интерфейс.",
		Source:      "routerforge-official", Managed: true, PackageAuthoritative: true,
		ProjectURL:    "https://github.com/siamskoi/keenetic-crowdsec-bouncer",
		Publisher:     catalogPublisher{ID: "routerforge", Name: "RouterForge", URL: "https://github.com/Fifth-Ace/routerforge"},
		Trust:         catalogTrust{Status: "official", ReviewedBy: "routerforge", Note: "RouterForge manager; bouncer runtime is third-party software by siamskoi/CrowdSec."},
		Capabilities:  []string{"status", "firewall-integrity", "ipset-members", "metrics", "lapi-config", "service-control", "ndm-hook"},
		Detection:     catalogDetection{Packages: []string{"routerforge-crowdsec-manager"}, Services: []string{"/opt/etc/init.d/S99routerforge-crowdsec-manager"}},
		ProcessNames:  []string{"routerforge-crowdsec-manager"},
		Compatibility: catalogCompatibility{Status: "requirements", Hints: []string{"Keenetic Entware", "Firewall Bouncer installed separately", "remote CrowdSec LAPI on Windows"}, Targets: []string{"aarch64-3.10"}},
		Install:       catalogInstallPlan{Method: "routerforge-release", Repository: "routerforge-dev", Packages: []string{"routerforge-crowdsec-manager"}, Notes: []string{"Устанавливает интерфейс RouterForge, а не CrowdSec LAPI или bouncer."}},
		Update:        catalogInstallPlan{Method: "routerforge-release", Repository: "routerforge-dev", Packages: []string{"routerforge-crowdsec-manager"}},
		Remove:        catalogInstallPlan{Method: "opkg", Packages: []string{"routerforge-crowdsec-manager"}, Notes: []string{"Bouncer, LAPI, конфигурация и правила firewall не удаляются."}},
		Presentation:  map[string]any{"dashboard": map[string]any{"enabled": false, "priority": 85}, "integration": map[string]any{"enabled": true, "label": "CrowdSec Manager", "href": "/integrations?open=crowdsec-manager", "order": 35, "target_ids": []string{"crowdsec"}}},
	}}
}

// Expose the external bouncer as its own integration so the Integrations
// navigation has a concrete target even when the bouncer is not installed.
// This is a discovery/onboarding record; it NEVER invokes upstream scripts.
func crowdsecBouncerIntegration() []catalogItem {
	return []catalogItem{{
		ID: "crowdsec", Kind: "integration", Name: "CrowdSec Firewall Bouncer", Category: "Security",
		Description: "Защита Keenetic с внешним CrowdSec LAPI. Установите bouncer на Entware и управляйте им через CrowdSec Manager.",
		Source:      "project-official", ProjectURL: "https://github.com/siamskoi/keenetic-crowdsec-bouncer",
		Publisher:     catalogPublisher{ID: "siamskoi", Name: "siamskoi", URL: "https://github.com/siamskoi/keenetic-crowdsec-bouncer"},
		Trust:         catalogTrust{Status: "unverified", Note: "Ручная установка внешнего bouncer; RouterForge не запускает сторонние скрипты автоматически."},
		Capabilities:  []string{"detect", "service-status", "security", "install-preview"},
		Detection:     catalogDetection{Services: []string{"/opt/etc/init.d/S99csfirewall"}, Paths: []string{"/opt/bin/cs-firewall-bouncer", "/opt/etc/crowdsec/crowdsec-firewall-bouncer.yaml"}},
		ProcessNames:  []string{"cs-firewall-bouncer"},
		Compatibility: catalogCompatibility{Status: "requirements", Hints: []string{"Keenetic Entware", "iptables + ipset", "Windows CrowdSec LAPI"}, Targets: []string{"aarch64-3.10"}},
		Install:       catalogInstallPlan{Method: "manual", PreviewOnly: true, Notes: []string{"Установите бинарник из проверенной сборки под ARM64, iptables/ipset и S99csfirewall по upstream-инструкции. LAPI работает отдельно на Windows."}},
		Presentation:  map[string]any{"integration": map[string]any{"enabled": true, "label": "CrowdSec", "href": "/integrations?open=crowdsec-manager", "order": 34}},
	}}
}
