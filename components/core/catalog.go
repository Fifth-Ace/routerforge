package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type catalogDetection struct {
	Packages []string `json:"packages,omitempty"`
	Services []string `json:"services,omitempty"`
	Paths    []string `json:"paths,omitempty"`
}

type catalogCompatibility struct {
	Status         string   `json:"status"`
	Hints          []string `json:"hints,omitempty"`
	Targets        []string `json:"targets,omitempty"`
	TargetStatus   string   `json:"target_status,omitempty"`
	DetectedTarget string   `json:"detected_target,omitempty"`
	TargetSource   string   `json:"target_source,omitempty"`
}

type catalogPackageMetadata struct {
	Architecture       string   `json:"architecture,omitempty"`
	DownloadSizeBytes  uint64   `json:"download_size_bytes,omitempty"`
	InstalledSizeBytes uint64   `json:"installed_size_bytes,omitempty"`
	Depends            []string `json:"depends,omitempty"`
	Conflicts          []string `json:"conflicts,omitempty"`
	Source             string   `json:"source,omitempty"`
}

type catalogWebMetadata struct {
	Scheme string `json:"scheme,omitempty"`
	Port   int    `json:"port,omitempty"`
	Path   string `json:"path,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Embed  bool   `json:"embed,omitempty"`
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
	ID               string                  `json:"id"`
	Kind             string                  `json:"kind"`
	Name             string                  `json:"name"`
	Category         string                  `json:"category"`
	Description      string                  `json:"description"`
	ProjectURL       string                  `json:"project_url,omitempty"`
	Source           string                  `json:"source"`
	State            string                  `json:"state"`
	Installed        bool                    `json:"installed"`
	Enabled          bool                    `json:"enabled"`
	Managed          bool                    `json:"managed,omitempty"`
	Version          string                  `json:"version,omitempty"`
	AvailableVersion string                  `json:"available_version,omitempty"`
	VersionSource    string                  `json:"version_source,omitempty"`
	PackageMeta      *catalogPackageMetadata `json:"package_meta,omitempty"`
	Conflicts        []string                `json:"conflicts,omitempty"`
	PackageInstalled bool                    `json:"package_installed,omitempty"`
	Service          string                  `json:"service,omitempty"`
	ServiceRunning   bool                    `json:"service_running"`
	Web              *catalogWebMetadata     `json:"web,omitempty"`
	WebPort          int                     `json:"web_port,omitempty"`
	WebPortSource    string                  `json:"web_port_source,omitempty"`
	Capabilities     []string                `json:"capabilities,omitempty"`
	Detection        catalogDetection        `json:"detection,omitempty"`
	Compatibility    catalogCompatibility    `json:"compatibility"`
	Install          catalogInstallPlan      `json:"install,omitempty"`
	Update           catalogInstallPlan      `json:"update,omitempty"`
	Remove           catalogInstallPlan      `json:"remove,omitempty"`
	Publisher        catalogPublisher        `json:"publisher,omitempty"`
	Trust            catalogTrust            `json:"trust,omitempty"`
	Provenance       catalogProvenance       `json:"provenance,omitempty"`
	LifecycleTrust   catalogLifecycleTrust   `json:"lifecycle_trust,omitempty"`
	Actions          catalogActions          `json:"actions"`
	ManifestID       string                  `json:"manifest_id,omitempty"`
	ManifestSHA256   string                  `json:"manifest_sha256,omitempty"`
	ManifestSource   string                  `json:"manifest_source,omitempty"`
	RegistrySource   string                  `json:"registry_source,omitempty"`
	Presentation     map[string]any          `json:"presentation,omitempty"`

	UnmanagedGitHub *catalogUnmanagedGitHub `json:"unmanaged_github,omitempty"`
	Release         catalogRelease          `json:"release,omitempty"`
	UpdateAvailable bool                    `json:"update_available,omitempty"`

	ProcessNames         []string `json:"process_names,omitempty"`
	RunningPaths         []string `json:"running_paths,omitempty"`
	WebRequiresPackage   string   `json:"web_requires_package,omitempty"`
	WebProbeHost         string   `json:"-"`
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

var catalogCacheState struct {
	mu       sync.RWMutex
	snapshot catalogSnapshot
}

var catalogRefreshMu sync.Mutex

var (
	catalogReadInstalledPackages    = readInstalledPackages
	catalogReadProcessNames         = readProcessNames
	catalogLoadOpkgCatalog          = loadOpkgCatalog
	catalogApplyRuntimeWebDiscovery = rfApplyRuntimeWebDiscovery
)

func init() {
	storeCatalogSnapshot(buildCatalog(
		map[string]string{},
		map[string]bool{},
		func(string) bool { return false },
	))
}

func readCatalog() catalogSnapshot {
	catalogCacheState.mu.RLock()
	snapshot := catalogCacheState.snapshot
	catalogCacheState.mu.RUnlock()
	return snapshot
}

func storeCatalogSnapshot(snapshot catalogSnapshot) {
	catalogCacheState.mu.Lock()
	catalogCacheState.snapshot = snapshot
	catalogCacheState.mu.Unlock()
}

func refreshCatalog() catalogSnapshot {
	catalogRefreshMu.Lock()
	defer catalogRefreshMu.Unlock()

	installed := catalogReadInstalledPackages()
	processes := catalogReadProcessNames()
	snapshot := buildCatalog(installed, processes, pathExists)
	applyRouterForgeRegistry(&snapshot, installed, processes, pathExists)
	applyUserAppSources(&snapshot, installed, processes, pathExists)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	if packages, err := catalogLoadOpkgCatalog(ctx, false); err == nil {
		applyIntegrationPackageVersions(&snapshot, packages)
	}
	cancel()

	for i := range snapshot.Integrations {
		snapshot.Integrations[i].Actions = deriveCatalogActions(snapshot.Integrations[i])
		appSourceApplyActionPolicy(&snapshot.Integrations[i])
	}

	for i := range snapshot.Modules {
		applyCatalogTrustModel(&snapshot.Modules[i])
	}
	for i := range snapshot.Integrations {
		applyCatalogTrustModel(&snapshot.Integrations[i])
	}

	catalogApplyRuntimeWebDiscovery(&snapshot, installed)

	applyRouterForgeReleaseIndex(&snapshot)
	snapshot.InstallTestMode = marketplaceTestInstallEnabled()
	snapshot.PackageManagementEnabled = snapshot.InstallTestMode
	if snapshot.PackageManagementEnabled {
		snapshot.ReadOnly = false
		snapshot.Phase = "routerforge-package-mode"
	}

	storeCatalogSnapshot(snapshot)
	return snapshot
}

func buildCatalog(installed map[string]string, processes map[string]bool, exists func(string) bool) catalogSnapshot {
	modules := append(append(builtinModuleCatalog(), networkToolsSeedModules()...), integrationManagerSeedModules()...)
	integrations := bundledRegistryIntegrations()

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
		"routerforge-core": 1,
		"monitoring":       2,
		"dns":              3,
		"admin":            4,
		"network-tools":    5,
		"nfqws-manager":    6,
		"profiling":        90,
		// Legacy logical IDs remain sortable while cached pre-consolidation
		// registries are being replaced by the rolling Dev registry.
		"system": 40, "thermal": 41, "storage": 42, "network": 43,
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
			Description: "Локальная платформа RouterForge: web shell, auth, Центр приложений, registry, module routing, settings и безопасный package lifecycle.",
			Source:      "builtin", Builtin: true, Enabled: true,
			Capabilities:  []string{"web-shell", "auth", "app-center", "registry", "module-routing", "settings", "package-lifecycle"},
			Compatibility: catalogCompatibility{Status: "built-in"},
		},
	}
}

func integrationManagerSeedModules() []catalogItem {
	return []catalogItem{
		{
			ID: "nfqws-manager", Kind: "module", Name: "RouterForge NFQWS Manager", Category: "Integrations",
			Description: "Installable RouterForge extension for safe management of an existing nfqws2-keenetic runtime.",
			ProjectURL:  "https://github.com/Fifth-Ace/routerforge",
			Source:      "routerforge-official", Managed: true, PackageAuthoritative: true,
			Publisher: catalogPublisher{ID: "routerforge", Name: "RouterForge", URL: "https://github.com/Fifth-Ace/routerforge"},
			Trust: catalogTrust{
				Status: "official", ReviewedBy: "routerforge",
				Note: "Official RouterForge integration-manager module. It never installs or upgrades nfqws2 itself.",
			},
			Capabilities: []string{"integration-manager", "nfqws2-status", "nfqws2-config", "nfqws2-lists", "nfqws2-log", "guarded-reload", "guarded-restart"},
			Detection: catalogDetection{
				Packages: []string{"routerforge-nfqws-manager"},
				Services: []string{"/opt/etc/init.d/S96routerforge-nfqws-manager"},
			},
			ProcessNames: []string{"routerforge-nfqws-manager"},
			Compatibility: catalogCompatibility{
				Status:  "requirements",
				Hints:   []string{"RouterForge Core", "Entware", "installed nfqws2-keenetic"},
				Targets: []string{"aarch64-3.10"},
			},
			Install: catalogInstallPlan{
				Method: "routerforge-release", Repository: "routerforge-dev", Packages: []string{"routerforge-nfqws-manager"},
				Notes: []string{"Installs only the RouterForge manager. nfqws2 remains an independent external package."},
			},
			Update: catalogInstallPlan{
				Method: "routerforge-release", Repository: "routerforge-dev", Packages: []string{"routerforge-nfqws-manager"},
				Notes: []string{"Updates only the RouterForge manager module."},
			},
			Remove: catalogInstallPlan{
				Method: "opkg", Packages: []string{"routerforge-nfqws-manager"},
				Notes: []string{"Removes only the RouterForge manager. nfqws2 configuration and package are untouched."},
			},
			Presentation: map[string]any{
				"dashboard": map[string]any{"enabled": false, "priority": 60},
				"integration": map[string]any{
					"enabled":    true,
					"label":      "NFQWS / NFQWS2",
					"href":       "/integrations?open=nfqws-manager",
					"order":      10,
					"target_ids": []string{"nfqws2"},
				},
			},
		},
	}
}

func networkToolsSeedModules() []catalogItem {
	return []catalogItem{
		networkToolsSeedModule("network-tools", "Network Tools", "Network Tools",
			"Network Doctor, traceroute, Route Inspector, Flow Explorer and bounded active probes.",
			"routerforge-network-tools", "/opt/etc/init.d/S97routerforge-network-tools", "routerforge-network-tools",
			"/network-tools", 50,
			[]string{"network-doctor", "traceroute", "route-inspector", "flow-explorer", "active-probes", "interfaces", "routes", "read-only"}),
	}
}
func networkToolsSeedModule(
	id, name, category, description, pkg, service, process, href string,
	order int,
	capabilities []string,
) catalogItem {
	return catalogItem{
		ID: id, Kind: "module", Name: name, Category: category, Description: description,
		ProjectURL: "https://github.com/Fifth-Ace/routerforge",
		Source:     "routerforge-official", Managed: true, PackageAuthoritative: true,
		Publisher: catalogPublisher{ID: "routerforge", Name: "RouterForge", URL: "https://github.com/Fifth-Ace/routerforge"},
		Trust: catalogTrust{
			Status: "official", ReviewedBy: "routerforge",
			Note: "Official RouterForge Network Tools module on the rolling ARM64 Dev channel.",
		},
		Capabilities: capabilities,
		Detection:    catalogDetection{Packages: []string{pkg}, Services: []string{service}},
		ProcessNames: []string{process},
		Compatibility: catalogCompatibility{
			Status:  "requirements",
			Hints:   []string{"RouterForge Core", "Entware", "Keenetic / Netcraze ARM64"},
			Targets: []string{"aarch64-3.10"},
		},
		Install: catalogInstallPlan{
			Method: "routerforge-release", Repository: "routerforge-dev", Packages: []string{pkg},
			Notes: []string{
				"Dev App Center downloads the exact package from the RouterForge release index and verifies SHA256 before opkg install.",
				"Current module API is read-only; mutation endpoints remain disabled.",
			},
		},
		Update: catalogInstallPlan{
			Method: "routerforge-release", Repository: "routerforge-dev", Packages: []string{pkg},
			Notes: []string{"Dev update uses the exact release-index asset and checksum."},
		},
		Remove: catalogInstallPlan{
			Method: "opkg", Packages: []string{pkg},
			Notes: []string{"Removes only the selected optional RouterForge module; Core remains installed."},
		},
		Presentation: map[string]any{
			"dashboard":  map[string]any{"enabled": true, "priority": order},
			"navigation": map[string]any{"label": name, "href": href, "order": order},
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

func bundledRegistryIntegrations() []catalogItem {
	doc, err := parseRouterForgeRegistry(bundledRouterForgeRegistry)
	if err != nil {
		panic("invalid bundled RouterForge integration registry: " + err.Error())
	}

	integrations := make([]catalogItem, 0, len(doc.Entries))
	for _, item := range doc.Entries {
		if item.Kind != "integration" || item.Builtin {
			continue
		}
		item.RegistrySource = "bundled"
		integrations = append(integrations, item)
	}
	return integrations
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
	runningPaths := item.RunningPaths
	if len(runningPaths) == 0 && item.Managed && item.PackageAuthoritative && len(item.ProcessNames) == 0 {
		// Registry v1 compatibility: early official modules encoded enable
		// markers in detection.paths before running_paths was populated.
		for _, p := range item.Detection.Paths {
			if strings.HasSuffix(strings.ToLower(p), ".enabled") {
				runningPaths = append(runningPaths, p)
			}
		}
	}
	for _, runningPath := range runningPaths {
		if exists(runningPath) {
			item.ServiceRunning = true
			break
		}
	}

	if item.WebRequiresPackage != "" {
		if _, ok := installed[item.WebRequiresPackage]; !ok {
			item.WebPort = 0
			item.Web = nil
		}
	}
	normalizeCatalogWebMetadata(item)

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

func normalizeCatalogWebMetadata(item *catalogItem) {
	if item == nil || item.Web != nil || item.WebPort <= 0 {
		return
	}
	item.Web = &catalogWebMetadata{
		Scheme: "http",
		Port:   item.WebPort,
		Path:   "/",
		Mode:   "probe-required",
		Embed:  true,
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
	var pkg, version, status string

	commit := func() {
		if pkg != "" && opkgStatusInstalled(status) {
			out[pkg] = version
		}
		pkg, version, status = "", "", ""
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
		if strings.HasPrefix(line, "Status:") {
			status = strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
		}
	}
	commit()
	return out
}

func opkgStatusInstalled(status string) bool {
	fields := strings.Fields(status)
	return len(fields) >= 3 && fields[0] == "install" && fields[len(fields)-1] == "installed"
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
