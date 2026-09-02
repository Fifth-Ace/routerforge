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
	Method        string   `json:"method,omitempty"`
	Repository    string   `json:"repository,omitempty"`
	RepositoryURL string   `json:"repository_url,omitempty"`
	Packages      []string `json:"packages,omitempty"`
	InstallerURL  string   `json:"installer_url,omitempty"`
	Notes         []string `json:"notes,omitempty"`
	PreviewOnly   bool     `json:"preview_only"`
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
	Version        string               `json:"version,omitempty"`
	Service        string               `json:"service,omitempty"`
	ServiceRunning bool                 `json:"service_running"`
	WebPort        int                  `json:"web_port,omitempty"`
	WebPortSource  string               `json:"web_port_source,omitempty"`
	Capabilities   []string             `json:"capabilities,omitempty"`
	Detection      catalogDetection     `json:"detection,omitempty"`
	Compatibility  catalogCompatibility `json:"compatibility"`
	Install        catalogInstallPlan   `json:"install,omitempty"`

	ProcessNames       []string `json:"-"`
	WebRequiresPackage string   `json:"-"`
	Builtin            bool     `json:"-"`
	Planned            bool     `json:"-"`
}

type catalogSnapshot struct {
	GeneratedAt  time.Time     `json:"generated_at"`
	ReadOnly     bool          `json:"read_only"`
	Phase        string        `json:"phase"`
	Modules      []catalogItem `json:"modules"`
	Integrations []catalogItem `json:"integrations"`
}

func readCatalog() catalogSnapshot {
	return buildCatalog(readInstalledPackages(), readProcessNames(), pathExists)
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
		if modules[i].Category != modules[j].Category {
			return modules[i].Category < modules[j].Category
		}
		return modules[i].Name < modules[j].Name
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
		Phase:        "catalog-foundation",
		Modules:      modules,
		Integrations: integrations,
	}
}

func builtinModuleCatalog() []catalogItem {
	return []catalogItem{
		{
			ID:            "dns-core",
			Kind:          "module",
			Name:          "DNS Core",
			Category:      "Core",
			Description:   "Базовый DNS-мониторинг, корреляция клиентов, resolver health и диагностика маршрутизации.",
			Source:        "builtin",
			Builtin:       true,
			Enabled:       true,
			Capabilities:  []string{"dns-observability", "client-attribution", "routing-diagnostics"},
			Compatibility: catalogCompatibility{Status: "built-in"},
		},
		{
			ID:            "marketplace",
			Kind:          "module",
			Name:          "Marketplace",
			Category:      "Platform",
			Description:   "Каталог модулей и сторонних интеграций. На первом этапе работает только в режиме обнаружения и предпросмотра.",
			Source:        "builtin",
			Builtin:       true,
			Enabled:       true,
			Capabilities:  []string{"catalog", "integration-detection", "install-plan-preview"},
			Compatibility: catalogCompatibility{Status: "built-in"},
		},
		plannedModule("system", "System Monitor", "Monitoring", "CPU по ядрам, RAM, uptime, load average и системная сводка."),
		plannedModule("thermal", "Thermal Monitor", "Monitoring", "Температуры по всем доступным thermal/hwmon датчикам."),
		plannedModule("storage", "Storage Monitor", "Monitoring", "Файловые системы, свободное место, I/O, USB и состояние хранилищ."),
		plannedModule("network", "Network Monitor", "Monitoring", "Интерфейсы, RX/TX, ошибки и сетевые счётчики."),
		plannedModule("admin", "Admin Tools", "Administration", "Процессы, службы, opkg, файлы и терминал. Потребует отдельной авторизации."),
		plannedModule("profiling", "Profiling", "Development", "pprof, slow-request logging и внутренняя диагностика DNS Monitor."),
	}
}

func plannedModule(id, name, category, description string) catalogItem {
	return catalogItem{
		ID:            id,
		Kind:          "module",
		Name:          name,
		Category:      category,
		Description:   description,
		Source:        "dns-monitor",
		Planned:       true,
		Capabilities:  []string{"planned"},
		Compatibility: catalogCompatibility{Status: "planned"},
	}
}

func integrationCatalog() []catalogItem {
	return []catalogItem{
		{
			ID:           "awg-manager",
			Kind:         "integration",
			Name:         "AWG Manager",
			Category:     "VPN / Routing",
			Description:  "Управление AmneziaWG и sing-box туннелями на Keenetic.",
			ProjectURL:   "https://github.com/hoaxisr/awg-manager",
			Source:       "official",
			Capabilities: []string{"detect", "version", "service-status", "open-ui", "install-preview"},
			Detection: catalogDetection{
				Packages: []string{"awg-manager"},
				Services: []string{"/opt/etc/init.d/S99awg-manager"},
				Paths:    []string{"/opt/etc/awg-manager"},
			},
			ProcessNames:  []string{"awg-manager"},
			WebPort:       2222,
			WebPortSource: "default-hint",
			Compatibility: catalogCompatibility{
				Status: "requirements",
				Hints:  []string{"Keenetic / Netcraze", "Entware", "Компонент WireGuard KeeneticOS"},
			},
			Install: catalogInstallPlan{
				Method:       "official-script",
				InstallerURL: "http://repo.hoaxisr.ru/install.sh",
				Packages:     []string{"awg-manager"},
				Notes:        []string{"Официальный install.sh AWG Manager.", "Выполнение из DNS Monitor пока отключено."},
				PreviewOnly:  true,
			},
		},
		{
			ID:           "nfqws",
			Kind:         "integration",
			Name:         "nfqws",
			Category:     "DPI / Bypass",
			Description:  "Классическая nfqws-keenetic интеграция. Поддерживается обнаружение; для новых установок предпочтительнее nfqws2.",
			ProjectURL:   "https://github.com/Anonym-tsk/nfqws-keenetic",
			Source:       "official",
			Capabilities: []string{"detect", "version", "service-status", "open-ui"},
			Detection: catalogDetection{
				Packages: []string{"nfqws-keenetic"},
				Services: []string{"/opt/etc/init.d/S51nfqws", "/opt/etc/init.d/S99zapret"},
				Paths:    []string{"/opt/etc/nfqws"},
			},
			ProcessNames:       []string{"nfqws"},
			WebPort:            90,
			WebPortSource:      "nfqws-keenetic-web",
			WebRequiresPackage: "nfqws-keenetic-web",
			Compatibility: catalogCompatibility{
				Status: "requirements",
				Hints:  []string{"Entware", "Netfilter kernel modules", "Legacy: nfqws2 является новой веткой"},
			},
			Install: catalogInstallPlan{
				Method:      "manual",
				Notes:       []string{"На первом этапе Marketplace только обнаруживает legacy nfqws.", "Автоматическую установку добавим после отдельной проверки официального feed."},
				PreviewOnly: true,
			},
		},
		{
			ID:           "nfqws2",
			Kind:         "integration",
			Name:         "nfqws2",
			Category:     "DPI / Bypass",
			Description:  "Новая версия nfqws для NFQUEUE/raw sockets с Entware-пакетом и отдельным web UI.",
			ProjectURL:   "https://github.com/nfqws/nfqws2-keenetic",
			Source:       "official",
			Capabilities: []string{"detect", "version", "service-status", "open-ui", "install-preview", "conflict-detection"},
			Detection: catalogDetection{
				Packages: []string{"nfqws2-keenetic"},
				Services: []string{"/opt/etc/init.d/S51nfqws2"},
				Paths:    []string{"/opt/etc/nfqws2"},
			},
			ProcessNames:       []string{"nfqws2"},
			WebPort:            90,
			WebPortSource:      "nfqws-keenetic-web",
			WebRequiresPackage: "nfqws-keenetic-web",
			Compatibility: catalogCompatibility{
				Status: "requirements",
				Hints:  []string{"Entware", "Netfilter kernel modules", "Конфликтует с legacy nfqws-keenetic"},
			},
			Install: catalogInstallPlan{
				Method:        "opkg-feed",
				Repository:    "nfqws2-keenetic",
				RepositoryURL: "https://nfqws.github.io/nfqws2-keenetic/all",
				Packages:      []string{"nfqws2-keenetic"},
				Notes:         []string{"Перед миграцией legacy nfqws должен быть удалён.", "Web UI nfqws-keenetic-web устанавливается отдельно."},
				PreviewOnly:   true,
			},
		},
		{
			ID:           "hydraroute-neo",
			Kind:         "integration",
			Name:         "HydraRoute Neo",
			Category:     "Routing",
			Description:  "Маршрутизация по доменам/CIDR и L7 (TLS SNI / HTTP Host / QUIC) через политики или интерфейсы Keenetic.",
			ProjectURL:   "https://github.com/Ground-Zerro/HydraRoute",
			Source:       "official",
			Capabilities: []string{"detect", "version", "service-status", "open-ui", "install-preview"},
			Detection: catalogDetection{
				Packages: []string{"hrneo"},
				Services: []string{"/opt/etc/init.d/S99hrneo"},
				Paths:    []string{"/opt/etc/HydraRoute"},
			},
			ProcessNames:  []string{"hrneo"},
			WebPort:       2000,
			WebPortSource: "default",
			Compatibility: catalogCompatibility{
				Status: "requirements",
				Hints:  []string{"KeeneticOS > 4.3.6", "Entware", "Xtables-addons для Netfilter"},
			},
			Install: catalogInstallPlan{
				Method:       "official-script",
				InstallerURL: "https://git.zerrolabs.org/Ground-Zerro/release/pages/keenetic/install-neo.sh",
				Packages:     []string{"hrneo"},
				Notes:        []string{"Официальный installer HydraRoute Neo также ставит HRweb.", "Выполнение из DNS Monitor пока отключено."},
				PreviewOnly:  true,
			},
		},
	}
}

func finalizeCatalogItem(item *catalogItem, installed map[string]string, processes map[string]bool, exists func(string) bool) {
	if item.Builtin {
		item.Installed = true
		item.State = "installed"
		return
	}
	if item.Planned {
		item.State = "planned"
		return
	}

	for _, pkg := range item.Detection.Packages {
		if version, ok := installed[pkg]; ok {
			item.Installed = true
			if item.Version == "" {
				item.Version = version
			}
		}
	}
	for _, service := range item.Detection.Services {
		if exists(service) {
			item.Installed = true
			if item.Service == "" {
				item.Service = service
			}
		}
	}
	for _, p := range item.Detection.Paths {
		if exists(p) {
			item.Installed = true
		}
	}
	for _, name := range item.ProcessNames {
		if processes[strings.ToLower(name)] {
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
		item.State = "installed_external"
		item.Enabled = item.ServiceRunning
	} else {
		item.State = "available"
	}
}

func readInstalledPackages() map[string]string {
	paths := []string{
		"/opt/lib/opkg/status",
		"/opt/var/opkg/status",
	}

	for i := range paths {
		p := paths[i]
		f, err := os.Open(p)
		if err != nil {
			continue
		}

		out := parseOpkgStatus(f)
		f.Close()

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
