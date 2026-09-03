package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type catalogDetection struct {
	Packages []string `json:"packages,omitempty"`
	Services []string `json:"services,omitempty"`
	Paths    []string `json:"paths,omitempty"`
}

type catalogCompatibility struct {
	Status string   `json:"status"`
	Hints  []string `json:"hints,omitempty"`
}

type catalogInstallPlan struct {
	Method        string                 `json:"method,omitempty"`
	Repository    string                 `json:"repository,omitempty"`
	RepositoryURL string                 `json:"repository_url,omitempty"`
	Packages      []string               `json:"packages,omitempty"`
	InstallerURL  string                 `json:"installer_url,omitempty"`
	ChecksumURL   string                 `json:"checksum_url,omitempty"`
	AssetTemplate string                 `json:"asset_template,omitempty"`
	Notes         []string               `json:"notes,omitempty"`
	Steps         []catalogLifecycleStep `json:"steps,omitempty"`
	PreviewOnly   bool                   `json:"preview_only"`
}

type catalogItem struct {
	ID             string               `json:"id"`
	Kind           string               `json:"kind"`
	Name           string               `json:"name"`
	Category       string               `json:"category"`
	Description    string               `json:"description"`
	ProjectURL     string               `json:"project_url,omitempty"`
	Source         string               `json:"source"`
	State          string               `json:"state"`
	Installed      bool                 `json:"installed"`
	Enabled        bool                 `json:"enabled"`
	Managed        bool                 `json:"managed,omitempty"`
	Version        string               `json:"version,omitempty"`
	Service        string               `json:"service,omitempty"`
	ServiceRunning bool                 `json:"service_running"`
	WebPort        int                  `json:"web_port,omitempty"`
	WebPortSource  string               `json:"web_port_source,omitempty"`
	Capabilities   []string             `json:"capabilities,omitempty"`
	Detection      catalogDetection     `json:"detection,omitempty"`
	Compatibility  catalogCompatibility `json:"compatibility"`
	Install        catalogInstallPlan   `json:"install,omitempty"`
	Update         catalogInstallPlan   `json:"update,omitempty"`
	Remove         catalogInstallPlan   `json:"remove,omitempty"`
	Publisher      catalogPublisher     `json:"publisher,omitempty"`
	Trust          catalogTrust         `json:"trust,omitempty"`
	Actions        catalogActions       `json:"actions"`
	ManifestID     string               `json:"manifest_id,omitempty"`
	ManifestSHA256 string               `json:"manifest_sha256,omitempty"`
	ManifestSource string               `json:"manifest_source,omitempty"`
	RegistrySource string               `json:"registry_source,omitempty"`
	Presentation   map[string]any       `json:"presentation,omitempty"`

	Release         catalogRelease `json:"release,omitempty"`
	UpdateAvailable bool           `json:"update_available,omitempty"`

	ProcessNames         []string `json:"process_names,omitempty"`
	RunningPaths         []string `json:"running_paths,omitempty"`
	WebRequiresPackage   string   `json:"web_requires_package,omitempty"`
	PackageAuthoritative bool     `json:"package_authoritative,omitempty"`
	Builtin              bool     `json:"builtin,omitempty"`
}

type catalogSnapshot struct {
	GeneratedAt     time.Time                 `json:"generated_at"`
	ReadOnly        bool                      `json:"read_only"`
	InstallTestMode bool                      `json:"install_test_mode"`
	Phase           string                    `json:"phase"`
	Brand           string                    `json:"brand"`
	Registry        routerForgeRegistryStatus `json:"registry"`
	Modules         []catalogItem             `json:"modules"`
	Integrations    []catalogItem             `json:"integrations"`

	PackageManagementEnabled bool                     `json:"package_management_enabled"`
	Release                  routerForgeReleaseStatus `json:"release"`
}

func readCatalog() catalogSnapshot {
	installed := readInstalledPackages()
	processes := readProcessNames()
	snapshot := buildCatalog(installed, processes, pathExists)
	applyRouterForgeRegistry(&snapshot, installed, processes, pathExists)
	applyRouterForgeReleaseIndex(&snapshot)
	snapshot.InstallTestMode = marketplaceTestInstallEnabled()
	snapshot.PackageManagementEnabled = snapshot.InstallTestMode
	if snapshot.PackageManagementEnabled {
		snapshot.ReadOnly = false
		snapshot.Phase = "routerforge-package-mode"
	}
	return snapshot
}

func buildCatalog(installed map[string]string, processes map[string]bool, exists func(string) bool) catalogSnapshot {
	modules := builtinModuleCatalog()
	integrations := integrationCatalog()

	for i := range modules {
		finalizeCatalogItem(&modules[i], installed, processes, exists)
	}
	for i := range integrations {
		finalizeCatalogItem(&integrations[i], installed, processes, exists)
	}

	sort.Slice(modules, func(i, j int) bool {
		return moduleOrder(modules[i].ID) < moduleOrder(modules[j].ID)
	})
	sort.Slice(integrations, func(i, j int) bool {
		if integrations[i].Category != integrations[j].Category {
			return integrations[i].Category < integrations[j].Category
		}
		return integrations[i].Name < integrations[j].Name
	})

	return catalogSnapshot{
		GeneratedAt:  time.Now(),
		ReadOnly:     true,
		Phase:        "combat-preview",
		Modules:      modules,
		Integrations: integrations,
	}
}

func moduleOrder(id string) int {
	order := map[string]int{
		"routerforge-core": 1, "marketplace": 2, "dns": 3, "admin": 4, "system": 5,
		"thermal": 6, "storage": 7, "network": 8, "profiling": 9,
	}
	if n, ok := order[id]; ok {
		return n
	}
	return 99
}

func builtinModuleCatalog() []catalogItem {
	return []catalogItem{
		{
			ID: "routerforge-core", Kind: "module", Name: "RouterForge Core", Category: "Core",
			Description: "Локальная платформа RouterForge: web shell, Marketplace, auth, registry и API для устанавливаемых возможностей.",
			Source:      "builtin", Builtin: true, Enabled: true,
			Capabilities:  []string{"dns-observability", "plain-dns", "client-attribution", "routing-diagnostics"},
			Compatibility: catalogCompatibility{Status: "built-in"},
		},
		{
			ID: "marketplace", Kind: "module", Name: "Marketplace", Category: "Platform",
			Description: "Каталог модулей RouterForge и проверенных интеграций Keenetic/Netcraze + Entware.",
			Source:      "builtin", Builtin: true, Enabled: true,
			Capabilities:  []string{"catalog", "integration-detection", "install-plan-preview"},
			Compatibility: catalogCompatibility{Status: "built-in"},
		},
		managedModule(
			"admin", "Admin Tools", "Administration",
			"Read-only системная админка: CPU, RAM, процессы, порты, службы, opkg, storage и thermal.",
			"routerforge-admin", "/opt/etc/init.d/S91routerforge-admin", "routerforge-admin",
			[]string{"system-summary", "cpu", "memory", "processes", "ports", "services", "packages", "storage", "thermal"},
		),
		managedModule(
			"system", "System Monitor", "Monitoring",
			"Легковесная системная телеметрия: CPU по ядрам, RAM, swap, uptime, load и процессы.",
			"routerforge-system", "/opt/etc/init.d/S92routerforge-system", "routerforge-system",
			[]string{"system-summary", "cpu", "memory"},
		),
		managedModule(
			"thermal", "Thermal Monitor", "Monitoring",
			"Все реальные thermal/hwmon датчики, Wi-Fi debugfs и optional SMART температуры накопителей.",
			"routerforge-thermal", "/opt/etc/init.d/S93routerforge-thermal", "routerforge-thermal",
			[]string{"thermal", "hwmon", "wifi-thermal", "smart-temperature"},
		),
		managedModule(
			"storage", "Storage Monitor", "Monitoring",
			"Файловые системы, block devices и пассивные I/O rates без фоновых benchmark-тестов.",
			"routerforge-storage", "/opt/etc/init.d/S94routerforge-storage", "routerforge-storage",
			[]string{"mounts", "block-devices", "diskstats", "io-rates"},
		),
		managedModule(
			"network", "Network Monitor", "Monitoring",
			"Интерфейсы, адреса, RX/TX rates, errors/drops, wireless counters, routes и conntrack.",
			"routerforge-network", "/opt/etc/init.d/S95routerforge-network", "routerforge-network",
			[]string{"interfaces", "traffic-rates", "errors", "wireless", "routes", "conntrack"},
		),
		{
			ID: "profiling", Kind: "module", Name: "Profiling", Category: "Development",
			Description: "Опциональный pprof Core listener на loopback и slow-request logging.",
			Source:      "dns-monitor", Managed: true,
			Capabilities: []string{"pprof", "slow-request-log"},
			Detection:    catalogDetection{Packages: []string{"routerforge-profiling"}},
			RunningPaths: []string{profilingMarker},
			Compatibility: catalogCompatibility{
				Status: "requirements",
				Hints:  []string{"RouterForge Core", "loopback-only listener", "SSH tunnel for remote access"},
			},
			Install: catalogInstallPlan{
				Method: "opkg-feed", Repository: "dns-monitor", Packages: []string{"routerforge-profiling"},
				Notes: []string{
					"Listener по умолчанию 127.0.0.1:6061 и не публикуется в LAN.",
					"Пакет перезапускает RouterForge Core после установки/удаления.",
				},
				PreviewOnly: true,
			},
		},
	}
}

func managedModule(id, name, category, description, pkg, service, process string, capabilities []string) catalogItem {
	return catalogItem{
		ID: id, Kind: "module", Name: name, Category: category, Description: description,
		Source: "routerforge", Managed: true, Capabilities: capabilities,
		Detection: catalogDetection{
			Packages: []string{pkg},
			Services: []string{service},
		},
		ProcessNames: []string{process},
		Compatibility: catalogCompatibility{
			Status: "requirements",
			Hints:  []string{"RouterForge Core", "Entware", "Keenetic / Netcraze ARM64"},
		},
		Install: catalogInstallPlan{
			Method: "opkg-feed", Repository: "dns-monitor", Packages: []string{pkg},
			Notes: []string{
				"Отдельный optional IPK. Core не тянет модуль как зависимость.",
				"v1 API модуля только read-only.",
			},
			PreviewOnly: true,
		},
	}
}

func integrationCatalog() []catalogItem {
	return []catalogItem{
		awgManagerIntegration(),
		nfqwsIntegration(),
		nfqws2Integration(),
		nfqwsWebIntegration(),
		hydraRouteIntegration(),
		xkeenIntegration(),
		xkeenUIIntegration(),
		keenPBRIntegration(),
		kvasIntegration(),
		bypassKeeneticIntegration(),
		trafficViaVPNIntegration(),
		adGuardHomeIntegration(),
		skeenIntegration(),
		churKeeneticIntegration(),
		keeneticSingBoxUIIntegration(),
		entwareExtrasIntegration(),
	}
}

func awgManagerIntegration() catalogItem {
	return catalogItem{
		ID: "awg-manager", Kind: "integration", Name: "AWG Manager", Category: "VPN / Routing",
		Description: "Управление AmneziaWG и sing-box туннелями непосредственно на Keenetic.",
		ProjectURL:  "https://github.com/hoaxisr/awg-manager", Source: "project-official",
		Capabilities: []string{"detect", "version", "service-status", "open-ui", "install-preview"},
		Detection: catalogDetection{
			Packages: []string{"awg-manager"},
			Services: []string{"/opt/etc/init.d/S99awg-manager"},
			Paths:    []string{"/opt/etc/awg-manager"},
		},
		ProcessNames: []string{"awg-manager"}, WebPort: 2222, WebPortSource: "project-default",
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"Keenetic / Netcraze", "Entware", "WireGuard component"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/hoaxisr/awg-manager/master/scripts/install.sh",
			Packages:     []string{"awg-manager"},
			Notes:        []string{"HTTPS GitHub installer опубликован самим проектом.", "Автовыполнение из Marketplace отключено."},
			PreviewOnly:  true,
		},
	}
}

func nfqwsIntegration() catalogItem {
	return catalogItem{
		ID: "nfqws", Kind: "integration", Name: "nfqws", Category: "DPI / Bypass",
		Description: "Классическая ветка nfqws для Keenetic. Для новых установок предпочтительнее nfqws2.",
		ProjectURL:  "https://github.com/nfqws/nfqws-keenetic", Source: "project-official",
		Capabilities: []string{"detect", "version", "service-status", "open-ui"},
		Detection: catalogDetection{
			Packages: []string{"nfqws-keenetic"},
			Services: []string{"/opt/etc/init.d/S51nfqws", "/opt/etc/init.d/S99zapret"},
			Paths:    []string{"/opt/etc/nfqws"},
		},
		ProcessNames: []string{"nfqws"},
		WebPort:      90, WebPortSource: "nfqws-keenetic-web",
		WebRequiresPackage: "nfqws-keenetic-web",
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"Entware", "Netfilter kernel modules", "Legacy branch"},
		},
		Install: catalogInstallPlan{
			Method: "manual", Notes: []string{"Legacy integration: Marketplace only detects it."}, PreviewOnly: true,
		},
	}
}

func nfqws2Integration() catalogItem {
	return catalogItem{
		ID: "nfqws2", Kind: "integration", Name: "nfqws2", Category: "DPI / Bypass",
		Description: "Актуальная nfqws ветка для Keenetic с Entware package и optional web UI.",
		ProjectURL:  "https://github.com/nfqws/nfqws2-keenetic", Source: "project-official",
		Capabilities: []string{"detect", "version", "service-status", "open-ui", "install-preview", "conflict-detection"},
		Detection: catalogDetection{
			Packages: []string{"nfqws2-keenetic"},
			Services: []string{"/opt/etc/init.d/S51nfqws2"},
			Paths:    []string{"/opt/etc/nfqws2"},
		},
		ProcessNames: []string{"nfqws2"},
		WebPort:      90, WebPortSource: "nfqws-keenetic-web",
		WebRequiresPackage: "nfqws-keenetic-web",
		Compatibility: catalogCompatibility{
			Status: "requirements",
			Hints:  []string{"Entware", "Netfilter kernel modules", "Legacy nfqws must be removed before migration"},
		},
		Install: catalogInstallPlan{
			Method: "opkg-feed", Repository: "nfqws2-keenetic",
			RepositoryURL: "https://nfqws.github.io/nfqws2-keenetic/all",
			Packages:      []string{"nfqws2-keenetic"},
			Notes: []string{
				"nfqws-keenetic-web is a separate optional package.",
				"Do not install side-by-side with legacy nfqws.",
			},
			PreviewOnly: true,
		},
	}
}

func nfqwsWebIntegration() catalogItem {
	return catalogItem{
		ID: "nfqws-web", Kind: "integration", Name: "nfqws Web UI", Category: "DPI / Bypass",
		Description: "Официальный веб-интерфейс для nfqws и nfqws2 на Keenetic/Netcraze и Entware.",
		ProjectURL:  "https://github.com/nfqws/nfqws-keenetic-web", Source: "project-official",
		Capabilities: []string{"detect", "version", "open-ui", "install-preview"},
		Detection: catalogDetection{
			Packages: []string{"nfqws-keenetic-web"},
			Paths:    []string{"/opt/etc/nfqws_web.conf"},
		},
		WebPort: 90, WebPortSource: "project-default",
		Compatibility: catalogCompatibility{
			Status: "requirements",
			Hints:  []string{"Entware", "nfqws or nfqws2", "php8-cgi", "lighttpd"},
		},
		Install: catalogInstallPlan{
			Method: "opkg-feed", Repository: "nfqws-keenetic-web",
			RepositoryURL: "https://nfqws.github.io/nfqws-keenetic-web/all",
			Packages:      []string{"nfqws-keenetic-web"},
			Notes: []string{
				"Web UI supports both nfqws and nfqws2.",
				"Default local panel port is :90.",
			},
			PreviewOnly: true,
		},
	}
}

func hydraRouteIntegration() catalogItem {
	return catalogItem{
		ID: "hydraroute-neo", Kind: "integration", Name: "HydraRoute Neo", Category: "Routing",
		Description: "Маршрутизация по доменам/CIDR и L7 SNI/HTTP/QUIC через Keenetic policies/interfaces.",
		ProjectURL:  "https://github.com/Ground-Zerro/HydraRoute", Source: "project-official",
		Capabilities: []string{"detect", "version", "service-status", "open-ui", "install-preview"},
		Detection: catalogDetection{
			Packages: []string{"hrneo"},
			Services: []string{"/opt/etc/init.d/S99hrneo"},
			Paths:    []string{"/opt/etc/HydraRoute"},
		},
		ProcessNames: []string{"hrneo"}, WebPort: 2000, WebPortSource: "project-default",
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"KeeneticOS > 4.3.6", "Entware", "Xtables-addons for Netfilter"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://git.zerrolabs.org/Ground-Zerro/release/pages/keenetic/install-neo.sh",
			Packages:     []string{"hrneo"},
			Notes:        []string{"Project installer also deploys HRweb.", "Execution remains disabled."},
			PreviewOnly:  true,
		},
	}
}

func xkeenIntegration() catalogItem {
	return catalogItem{
		ID: "xkeen", Kind: "integration", Name: "XKeen", Category: "VPN / Routing",
		Description: "Xray runtime/update toolkit for Keenetic with GeoIP/GeoSite management.",
		ProjectURL:  "https://github.com/Skrill0/XKeen", Source: "community",
		Capabilities: []string{"detect", "version", "service-status", "install-preview"},
		Detection: catalogDetection{
			Packages: []string{"xkeen"},
			Paths:    []string{"/opt/sbin/xkeen", "/opt/etc/xray/configs"},
		},
		ProcessNames: []string{"xray", "xray-linux-arm64"},
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"Keenetic", "Entware", "curl", "tar", "Xray configuration"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/Skrill0/XKeen/main/install.sh",
			Notes: []string{
				"Official installer downloads the latest xkeen.tar release and runs xkeen -i.",
				"Inspect existing Xray/sing-box routing stacks before installation.",
			},
			PreviewOnly: true,
		},
	}
}

func xkeenUIIntegration() catalogItem {
	return catalogItem{
		ID: "xkeen-ui", Kind: "integration", Name: "XKeen UI", Category: "VPN / Routing",
		Description: "Легковесная веб-панель управления XKeen, Xray/Mihomo конфигами, логами и ядрами.",
		ProjectURL:  "https://github.com/zxc-rv/XKeen-UI", Source: "community",
		Capabilities: []string{"detect", "service-status", "open-ui", "install-preview"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99xkeen-ui"},
			Paths:    []string{"/opt/sbin/xkeen-ui", "/opt/share/www/XKeen-UI"},
		},
		ProcessNames: []string{"xkeen-ui"},
		WebPort:      1000, WebPortSource: "project-default",
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"Keenetic / Netcraze", "Entware", "XKeen installed"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/zxc-rv/XKeen-UI/main/setup.sh",
			Notes: []string{
				"Upstream documents XKeen as its only application dependency.",
				"Default port is :1000 and can be changed in S99xkeen-ui.",
			},
			PreviewOnly: true,
		},
	}
}

func keenPBRIntegration() catalogItem {
	return catalogItem{
		ID: "keen-pbr", Kind: "integration", Name: "keen-pbr", Category: "Routing",
		Description: "Policy-based routing daemon for Keenetic/OpenWrt; full variant includes REST API and WebUI.",
		ProjectURL:  "https://github.com/maksimkurb/keen-pbr", Source: "community",
		Capabilities: []string{"detect", "version", "service-status", "install-preview"},
		Detection: catalogDetection{
			Packages: []string{"keen-pbr", "keen-pbr-headless"},
			Services: []string{"/opt/etc/init.d/S80keen-pbr"},
			Paths:    []string{"/opt/etc/keen-pbr"},
		},
		ProcessNames: []string{"keen-pbr"},
		Compatibility: catalogCompatibility{
			Status: "requirements",
			Hints:  []string{"Entware", "conntrack/dnsmasq/ipset/iptables", "Keenetic Netfilter"},
		},
		Install: catalogInstallPlan{
			Method: "manual", Packages: []string{"keen-pbr", "keen-pbr-headless"},
			Notes:       []string{"Project builds dedicated Keenetic packages.", "Choose full OR headless variant; port is intentionally not guessed."},
			PreviewOnly: true,
		},
	}
}

func kvasIntegration() catalogItem {
	return catalogItem{
		ID: "kvas", Kind: "integration", Name: "КВАС", Category: "VPN / Routing",
		Description: "Domain-selective routing/VPN toolkit using ipset with dnsmasq/dnscrypt or AdGuard Home.",
		ProjectURL:  "https://github.com/qzeleza/kvas", Source: "community",
		Capabilities: []string{"detect", "version", "service-status", "install-preview"},
		Detection: catalogDetection{
			Packages: []string{"kvas"},
			Paths:    []string{"/opt/apps/kvas", "/opt/etc/kvas.conf", "/opt/etc/kvas.list"},
		},
		ProcessNames: []string{"kvas", "kvas-failover"},
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"Keenetic", "Entware", "curl for installer"},
		},
		Install: catalogInstallPlan{
			Method: "official-script", InstallerURL: "http://kvas.zeleza.ru/install", Packages: []string{"kvas"},
			Notes: []string{
				"Upstream installer uses HTTP: Marketplace marks this plan high-risk and never executes it automatically.",
				"Interactive setup remains manual.",
			},
			PreviewOnly: true,
		},
	}
}

func bypassKeeneticIntegration() catalogItem {
	return catalogItem{
		ID: "bypass-keenetic", Kind: "integration", Name: "bypass_keenetic", Category: "VPN / Routing",
		Description: "Selective bypass via VPN/Shadowsocks/Tor with Telegram bot management.",
		ProjectURL:  "https://github.com/keenetic-dev/bypass_keenetic", Source: "community",
		Capabilities: []string{"detect", "service-status", "install-preview"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99unblock"},
			Paths:    []string{"/opt/etc/bot.py", "/opt/etc/unblock"},
		},
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"Keenetic", "Entware", "Python 3", "VPN/Shadowsocks/Tor depending on setup"},
		},
		Install: catalogInstallPlan{
			Method: "manual",
			Notes: []string{
				"Telegram credentials and interactive setup require explicit manual configuration.",
				"RouterForge only detects/documents this integration.",
			},
			PreviewOnly: true,
		},
	}
}

func trafficViaVPNIntegration() catalogItem {
	return catalogItem{
		ID: "traffic-via-vpn", Kind: "integration", Name: "keenetic-traffic-via-vpn", Category: "VPN / Routing",
		Description: "Small Entware script set that redirects selected domains/IP/CIDR into a chosen VPN interface.",
		ProjectURL:  "https://github.com/rustrict/keenetic-traffic-via-vpn", Source: "community",
		Capabilities: []string{"detect", "install-preview"},
		Detection: catalogDetection{
			Paths: []string{"/opt/etc/unblock/config", "/opt/etc/unblock/unblock-list.txt"},
		},
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"Keenetic", "Entware", "curl", "bind-dig", "cron", "grep", "existing VPN interface"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://raw.githubusercontent.com/rustrict/keenetic-traffic-via-vpn/main/install.sh",
			Notes:        []string{"Installer creates /opt/etc/unblock and IFACE must be configured afterwards."},
			PreviewOnly:  true,
		},
	}
}

func adGuardHomeIntegration() catalogItem {
	return catalogItem{
		ID: "adguardhome-keenetic", Kind: "integration", Name: "AdGuard Home", Category: "DNS / Filtering",
		Description: "Network-wide DNS filtering. Keenetic setup can use Entware adguardhome-go.",
		ProjectURL:  "https://github.com/Corvus-Malus/AdGuardHome-Keenetic", Source: "community",
		Capabilities: []string{"detect", "version", "service-status", "open-ui", "install-preview"},
		Detection: catalogDetection{
			Packages: []string{"adguardhome-go"},
			Services: []string{"/opt/etc/init.d/S99adguardhome"},
			Paths:    []string{"/opt/etc/AdGuardHome"},
		},
		ProcessNames: []string{"AdGuardHome", "adguardhome-go"},
		WebPort:      3000, WebPortSource: "initial-setup-default",
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"Entware", "Port 53 ownership", "Keenetic dns-override only for explicit DNS replacement"},
		},
		Install: catalogInstallPlan{
			Method: "opkg", Repository: "entware", Packages: []string{"adguardhome-go"},
			Notes: []string{
				"Installing the package alone does not replace Keenetic DNS.",
				"dns-override changes DNS ownership and remains an explicit manual step.",
			},
			PreviewOnly: true,
		},
	}
}

func skeenIntegration() catalogItem {
	return catalogItem{
		ID: "skeen", Kind: "integration", Name: "SKeen", Category: "VPN / Routing",
		Description: "Keenetic/Netcraze TProxy & Redirect toolkit around sing-box with built-in web dashboard.",
		ProjectURL:  "https://github.com/jinndi/SKeen", Source: "community",
		Capabilities: []string{"detect", "service-status", "open-ui", "install-preview"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99SKeen"},
			Paths:    []string{"/opt/etc/skeen", "/opt/bin/skeen"},
		},
		ProcessNames: []string{"skeen", "skeen-box"},
		WebPort:      9999, WebPortSource: "project-default",
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"Entware", "Netfilter kernel modules", "curl", "256 MB RAM recommended by project"},
		},
		Install: catalogInstallPlan{
			Method:       "official-script",
			InstallerURL: "https://github.com/jinndi/SKeen/releases/latest/download/skeen",
			Notes: []string{
				"Installer is interactive and can install/select sing-box.",
				"Project dashboard defaults to :9999.",
			},
			PreviewOnly: true,
		},
	}
}

func churKeeneticIntegration() catalogItem {
	return catalogItem{
		ID: "chur-keenetic", Kind: "integration", Name: "Chur Keenetic", Category: "VPN / Routing",
		Description: "Web-менеджер VPN-интерфейсов Keenetic/Entware с optional AmneziaWG runtime.",
		ProjectURL:  "https://github.com/ward-sentry/chur-keenetic", Source: "community",
		Capabilities: []string{"detect", "version", "service-status", "open-ui", "install-preview"},
		Detection: catalogDetection{
			Packages: []string{"chur-keenetic"},
			Services: []string{"/opt/etc/init.d/S99chur-keenetic"},
		},
		ProcessNames: []string{"chur-keenetic"},
		WebPort:      8088, WebPortSource: "project-default",
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"Keenetic", "Entware", "HTTPS-capable opkg feed", "aarch64-3.10 supported"},
		},
		Install: catalogInstallPlan{
			Method: "opkg-feed", Repository: "chur",
			RepositoryURL: "https://ward-sentry.github.io/chur-keenetic/latest/aarch64-3.10",
			Packages:      []string{"chur-keenetic"},
			Notes: []string{
				"Marketplace target build is ARM64, therefore the preview shows the aarch64-3.10 feed.",
				"AmneziaWG runtime is installed separately by Chur only when requested.",
			},
			PreviewOnly: true,
		},
	}
}

func keeneticSingBoxUIIntegration() catalogItem {
	return catalogItem{
		ID: "keenetic-sing-box-ui", Kind: "integration", Name: "Keenetic sing-box UI", Category: "VPN / Routing",
		Description: "Aarch64 web UI for sing-box with selective TProxy/REDIRECT routing, diagnostics and authentication.",
		ProjectURL:  "https://github.com/CoOre/keenetic-sing-box-ui", Source: "community",
		Capabilities: []string{"detect", "service-status", "open-ui", "install-preview"},
		Detection: catalogDetection{
			Services: []string{"/opt/etc/init.d/S99keenetic-sing-box-ui"},
			Paths:    []string{"/opt/bin/keenetic-sing-box-ui", "/opt/sbin/keenetic-sing-box-ui"},
		},
		ProcessNames: []string{"keenetic-sing-box-ui"},
		WebPort:      9091, WebPortSource: "project-default",
		Compatibility: catalogCompatibility{
			Status: "requirements",
			Hints:  []string{"Keenetic / Netcraze", "Entware", "aarch64", "Netfilter modules for TProxy mode"},
		},
		Install: catalogInstallPlan{
			Method: "release-deploy",
			Notes: []string{
				"Upstream deploys from a host script or a verified GitHub release rather than an Entware feed.",
				"Transparent proxy modes can modify iptables/ipset; execution stays preview-only.",
			},
			PreviewOnly: true,
		},
	}
}

func entwareExtrasIntegration() catalogItem {
	return catalogItem{
		ID: "keenetic-entware-extras", Kind: "integration", Name: "Keenetic Entware Extras", Category: "Toolkit",
		Description: "Geo routing, SmartDNS integration, DNS redirect, network diagnostics and optional web dashboard.",
		ProjectURL:  "https://github.com/0xkee/keenetic-entware-extras", Source: "community",
		Capabilities: []string{"detect", "version", "service-status", "open-ui", "install-preview"},
		Detection: catalogDetection{
			Packages: []string{"keenetic-entware-extras", "geo-split", "smartdns-geo-conf", "smartdns-redirect", "net-check", "webui"},
			Services: []string{"/opt/etc/init.d/S80nginx-webui"},
			Paths:    []string{"/opt/keenetic-entware-extras"},
		},
		WebPort: 8080, WebPortSource: "webui-default", WebRequiresPackage: "webui",
		Compatibility: catalogCompatibility{
			Status: "requirements", Hints: []string{"KeeneticOS 5.0+", "Entware"},
		},
		Install: catalogInstallPlan{
			Method: "opkg-feed", Repository: "kee",
			RepositoryURL: "https://0xkee.github.io/keenetic-entware-extras/stable",
			Packages:      []string{"geo-split", "smartdns-geo-conf", "smartdns-redirect", "net-check", "webui"},
			InstallerURL:  "https://raw.githubusercontent.com/0xkee/keenetic-entware-extras/master/scripts/install.sh",
			Notes: []string{
				"Packages are selectable; do not install the whole suite unless desired.",
				"WebUI is optional and listens on :8080.",
			},
			PreviewOnly: true,
		},
	}
}

func finalizeCatalogItem(item *catalogItem, installed map[string]string, processes map[string]bool, exists func(string) bool) {
	if item.Builtin {
		item.Installed = true
		item.State = "installed"
		if item.ID == "routerforge-core" {
			item.Version = version
		}
		return
	}

	packageDetected := false
	for _, pkg := range item.Detection.Packages {
		if version, ok := installed[pkg]; ok {
			item.Installed = true
			packageDetected = true
			if item.Version == "" {
				item.Version = version
			}
		}
	}
	for _, service := range item.Detection.Services {
		if exists(service) {
			if !item.PackageAuthoritative || packageDetected {
				item.Installed = true
			}
			if item.Service == "" {
				item.Service = service
			}
		}
	}
	for _, p := range item.Detection.Paths {
		if exists(p) && !item.PackageAuthoritative {
			item.Installed = true
		}
	}
	for _, name := range item.ProcessNames {
		if processes[strings.ToLower(name)] {
			item.ServiceRunning = true
			break
		}
	}
	for _, runningPath := range item.RunningPaths {
		if exists(runningPath) {
			item.ServiceRunning = true
			break
		}
	}

	if item.WebRequiresPackage != "" {
		if _, ok := installed[item.WebRequiresPackage]; !ok {
			item.WebPort = 0
		}
	}

	if item.Installed {
		if item.Managed {
			item.State = "installed"
		} else {
			item.State = "installed_external"
		}
		item.Enabled = item.ServiceRunning
	} else {
		item.State = "available"
	}
}

func readInstalledPackages() map[string]string {
	paths := []string{"/opt/lib/opkg/status", "/opt/var/opkg/status"}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		out := parseOpkgStatus(f)
		_ = f.Close()
		if len(out) > 0 {
			return out
		}
	}
	return map[string]string{}
}

func parseOpkgStatus(r io.Reader) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	var pkg, version string

	commit := func() {
		if pkg != "" {
			out[pkg] = version
		}
		pkg, version = "", ""
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			commit()
			continue
		}
		if strings.HasPrefix(line, "Package:") {
			pkg = strings.TrimSpace(strings.TrimPrefix(line, "Package:"))
		}
		if strings.HasPrefix(line, "Version:") {
			version = strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
		}
	}
	commit()
	return out
}

func readProcessNames() map[string]bool {
	out := map[string]bool{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		base := filepath.Join("/proc", entry.Name())
		if b, err := os.ReadFile(filepath.Join(base, "comm")); err == nil {
			name := strings.ToLower(strings.TrimSpace(string(b)))
			if name != "" {
				out[name] = true
			}
		}
		if b, err := os.ReadFile(filepath.Join(base, "cmdline")); err == nil {
			parts := strings.Split(string(b), "\x00")
			if len(parts) > 0 && parts[0] != "" {
				name := strings.ToLower(filepath.Base(parts[0]))
				if name != "" {
					out[name] = true
				}
			}
		}
	}
	return out
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
