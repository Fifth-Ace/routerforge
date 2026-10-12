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
