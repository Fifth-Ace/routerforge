package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	entwareCatalogTTL      = 60 * time.Second
	entwareCatalogMaxItems = 250
	entwareOutputMaxBytes  = 16000
	entwareDiskFloorBytes  = 32 << 20
)

type entwarePackage struct {
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	AvailableVersion string `json:"available_version,omitempty"`
	InstalledVersion string `json:"installed_version,omitempty"`
	Installed        bool   `json:"installed"`
	Upgradable       bool   `json:"upgradable"`
	Source           string `json:"source"`
}

type entwareCatalogResponse struct {
	GeneratedAt              time.Time        `json:"generated_at"`
	Online                   bool             `json:"online"`
	PackageManagementEnabled bool             `json:"package_management_enabled"`
	Total                    int              `json:"total"`
	InstalledCount           int              `json:"installed_count"`
	UpgradableCount          int              `json:"upgradable_count"`
	Offset                   int              `json:"offset"`
	Limit                    int              `json:"limit"`
	Items                    []entwarePackage `json:"items"`
	Error                    string           `json:"error,omitempty"`
}

type entwarePackageDetail struct {
	Name               string   `json:"name"`
	Description        string   `json:"description,omitempty"`
	Version            string   `json:"version,omitempty"`
	AvailableVersion   string   `json:"available_version,omitempty"`
	InstalledVersion   string   `json:"installed_version,omitempty"`
	Architecture       string   `json:"architecture,omitempty"`
	Status             string   `json:"status,omitempty"`
	Section            string   `json:"section,omitempty"`
	Maintainer         string   `json:"maintainer,omitempty"`
	DependsRaw         string   `json:"depends_raw,omitempty"`
	Depends            []string `json:"depends,omitempty"`
	DownloadSizeBytes  uint64   `json:"download_size_bytes,omitempty"`
	InstalledSizeBytes uint64   `json:"installed_size_bytes,omitempty"`
	Installed          bool     `json:"installed"`
	Upgradable         bool     `json:"upgradable"`
	PackageSource      string   `json:"package_source,omitempty"`
	MetadataSource     string   `json:"metadata_source"`
}

type entwareActionRequest struct {
	Package string `json:"package"`
	Action  string `json:"action"`
	Confirm string `json:"confirm,omitempty"`
}

type entwareActionPreflight struct {
	Package             string   `json:"package"`
	Action              string   `json:"action"`
	Allowed             bool     `json:"allowed"`
	Reason              string   `json:"reason,omitempty"`
	PackageManagement   bool     `json:"package_management_enabled"`
	Installed           bool     `json:"installed"`
	Upgradable          bool     `json:"upgradable"`
	InstalledVersion    string   `json:"installed_version,omitempty"`
	AvailableVersion    string   `json:"available_version,omitempty"`
	Architecture        string   `json:"architecture,omitempty"`
	Dependencies        []string `json:"dependencies,omitempty"`
	MissingDependencies []string `json:"missing_dependencies,omitempty"`
	ReverseDependencies []string `json:"reverse_dependencies,omitempty"`
	DownloadSizeBytes   uint64   `json:"download_size_bytes,omitempty"`
	InstalledSizeBytes  uint64   `json:"installed_size_bytes,omitempty"`
	OptAvailableBytes   uint64   `json:"opt_available_bytes,omitempty"`
	OptTotalBytes       uint64   `json:"opt_total_bytes,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
}

type entwareActionResult struct {
	Package     string    `json:"package"`
	Action      string    `json:"action"`
	Installed   bool      `json:"installed"`
	Version     string    `json:"version,omitempty"`
	Output      string    `json:"output,omitempty"`
	CompletedAt time.Time `json:"completed_at"`
}

var entwareCache = struct {
	sync.Mutex
	at    time.Time
	items []entwarePackage
}{}

func registerAppCenterHandlers(mux *http.ServeMux) {
	list := http.HandlerFunc(handleEntwareCatalog)
	refresh := http.HandlerFunc(handleEntwareRefresh)
	action := http.HandlerFunc(handleEntwareAction)
	detail := http.HandlerFunc(handleEntwareDetail)
	preflight := http.HandlerFunc(handleEntwarePreflight)

	mux.Handle("/api/apps/entware", list)
	mux.Handle("/api/apps/entware/refresh", refresh)
	mux.Handle("/api/apps/entware/action", action)
	mux.Handle("/api/apps/entware/detail", detail)
	mux.Handle("/api/apps/entware/preflight", preflight)

	// Compatibility aliases: legacy catalog namespace stays available for one
	// transition cycle while the user-facing product is renamed to App Center.
	mux.Handle("/api/catalog/entware", list)
	mux.Handle("/api/catalog/entware/refresh", refresh)
	mux.Handle("/api/catalog/entware/action", action)
	mux.Handle("/api/catalog/entware/detail", detail)
	mux.Handle("/api/catalog/entware/preflight", preflight)
}

func handleEntwareCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	items, err := loadEntwareCatalog(ctx, false)
	if err != nil {
		writeCatalogJSON(w, http.StatusServiceUnavailable, entwareCatalogResponse{
			GeneratedAt:              time.Now(),
			Online:                   false,
			PackageManagementEnabled: marketplaceTestInstallEnabled(),
			Items:                    []entwarePackage{},
			Error:                    err.Error(),
		})
		return
	}

	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	state := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("state")))
	offset := parseEntwareInt(r.URL.Query().Get("offset"), 0, 0, 1000000)
	limit := parseEntwareInt(r.URL.Query().Get("limit"), 100, 1, entwareCatalogMaxItems)

	filtered := make([]entwarePackage, 0, len(items))
	installedCount := 0
	upgradableCount := 0

	for _, item := range items {
		if item.Installed {
			installedCount++
		}
		if item.Upgradable {
			upgradableCount++
		}

		switch state {
		case "installed":
			if !item.Installed {
				continue
			}
		case "available":
			if item.Installed {
				continue
			}
		case "upgradable", "updates":
			if !item.Upgradable {
				continue
			}
		}

		if query != "" {
			haystack := strings.ToLower(item.Name + " " + item.Description + " " + item.AvailableVersion + " " + item.InstalledVersion)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		filtered = append(filtered, item)
	}

	total := len(filtered)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	writeCatalogJSON(w, http.StatusOK, entwareCatalogResponse{
		GeneratedAt:              time.Now(),
		Online:                   true,
		PackageManagementEnabled: marketplaceTestInstallEnabled(),
		Total:                    total,
		InstalledCount:           installedCount,
		UpgradableCount:          upgradableCount,
		Offset:                   offset,
		Limit:                    limit,
		Items:                    append([]entwarePackage(nil), filtered[offset:end]...),
	})
}

func handleEntwareDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required"})
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("package"))
	if !safeCatalogPackageName(name) {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid package name"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	detail, found, err := loadEntwarePackageDetail(ctx, name)
	if err != nil {
		writeCatalogJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}
	if !found {
		writeCatalogJSON(w, http.StatusNotFound, map[string]any{"error": "package not found"})
		return
	}
	writeCatalogJSON(w, http.StatusOK, detail)
}

func handleEntwarePreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if !sameOriginRequest(r) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin Entware preflight rejected"})
		return
	}

	var request entwareActionRequest
	if err := decodeSmallJSON(w, r, &request); err != nil {
		return
	}
	request.Package = strings.TrimSpace(request.Package)
	request.Action = normalizeEntwareAction(request.Action)
	if !safeCatalogPackageName(request.Package) {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid package name"})
		return
	}
	if request.Action == "" {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported Entware action"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	preflight, err := buildEntwarePreflight(ctx, request.Package, request.Action)
	if err != nil {
		writeCatalogJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}
	writeCatalogJSON(w, http.StatusOK, preflight)
}

func handleEntwareRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if !sameOriginRequest(r) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin package-list refresh rejected"})
		return
	}
	if !marketplaceTestInstallEnabled() {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{
			"error":  "RouterForge package management is disabled",
			"marker": marketplaceTestInstallMarker,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	marketplaceInstallMu.Lock()
	defer marketplaceInstallMu.Unlock()

	opkg, err := opkgExecutable()
	if err != nil {
		writeCatalogJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}

	output, runErr := exec.CommandContext(ctx, opkg, "update").CombinedOutput()
	if runErr != nil {
		writeCatalogJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "opkg update failed",
			"detail": truncateCatalogInstallOutput(string(output), entwareOutputMaxBytes),
		})
		return
	}

	invalidateEntwareCatalog()
	if _, err := loadEntwareCatalog(ctx, true); err != nil {
		writeCatalogJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "package lists updated but App Center cache refresh failed",
			"detail": err.Error(),
			"output": truncateCatalogInstallOutput(string(output), entwareOutputMaxBytes),
		})
		return
	}
	refreshCatalog()

	writeCatalogJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"output": truncateCatalogInstallOutput(string(output), entwareOutputMaxBytes),
	})
}

func handleEntwareAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if !sameOriginRequest(r) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin Entware action rejected"})
		return
	}
	if !marketplaceTestInstallEnabled() {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{
			"error":  "RouterForge package management is disabled",
			"marker": marketplaceTestInstallMarker,
		})
		return
	}

	var request entwareActionRequest
	if err := decodeSmallJSON(w, r, &request); err != nil {
		return
	}

	request.Package = strings.TrimSpace(request.Package)
	request.Action = normalizeEntwareAction(request.Action)

	if !safeCatalogPackageName(request.Package) {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid package name"})
		return
	}
	if entwarePackageProtected(request.Package) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{
			"error": "RouterForge and integration-owned packages must be managed through their App Center card",
		})
		return
	}
	if request.Action == "" {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported Entware action"})
		return
	}
	if request.Action == "remove" && request.Confirm != request.Package {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{
			"error": "typed removal confirmation does not match package name",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	marketplaceInstallMu.Lock()
	defer marketplaceInstallMu.Unlock()

	preflight, err := buildEntwarePreflight(ctx, request.Package, request.Action)
	if err != nil {
		writeCatalogJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}
	if !preflight.Allowed {
		writeCatalogJSON(w, http.StatusConflict, map[string]any{
			"error":     "Entware action rejected by preflight",
			"detail":    preflight.Reason,
			"preflight": preflight,
		})
		return
	}

	opkg, err := opkgExecutable()
	if err != nil {
		writeCatalogJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}

	verb := entwareOpkgVerb(request.Action)
	output, runErr := exec.CommandContext(ctx, opkg, verb, request.Package).CombinedOutput()
	result := entwareActionResult{
		Package: request.Package,
		Action:  request.Action,
		Output:  truncateCatalogInstallOutput(string(output), entwareOutputMaxBytes),
	}
	if runErr != nil {
		writeCatalogJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "opkg " + request.Action + " failed",
			"detail": result.Output,
			"result": result,
		})
		return
	}

	installed, installedErr := loadOpkgInstalled(ctx, opkg)
	if installedErr != nil {
		writeCatalogJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "opkg action completed but installed package state could not be verified",
			"detail": installedErr.Error(),
			"result": result,
		})
		return
	}
	result.Version, result.Installed = installed[request.Package]
	result.CompletedAt = time.Now()

	if request.Action == "remove" && result.Installed {
		writeCatalogJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "opkg remove completed but package is still installed",
			"result": result,
		})
		return
	}
	if request.Action != "remove" && !result.Installed {
		writeCatalogJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "opkg action completed but package is not installed",
			"result": result,
		})
		return
	}

	invalidateEntwareCatalog()
	refreshCatalog()
	writeCatalogJSON(w, http.StatusOK, result)
}

func normalizeEntwareAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "install":
		return "install"
	case "update", "upgrade":
		return "update"
	case "remove":
		return "remove"
	default:
		return ""
	}
}

func entwareOpkgVerb(action string) string {
	switch action {
	case "install":
		return "install"
	case "update":
		return "upgrade"
	case "remove":
		return "remove"
	default:
		return ""
	}
}

func loadEntwareCatalog(ctx context.Context, force bool) ([]entwarePackage, error) {
	items, err := loadOpkgCatalog(ctx, force)
	if err != nil {
		return nil, err
	}
	return filterEntwarePackages(items), nil
}

func loadOpkgCatalog(ctx context.Context, force bool) ([]entwarePackage, error) {
	entwareCache.Lock()
	defer entwareCache.Unlock()

	if !force && len(entwareCache.items) > 0 && time.Since(entwareCache.at) < entwareCatalogTTL {
		return append([]entwarePackage(nil), entwareCache.items...), nil
	}

	opkg, err := opkgExecutable()
	if err != nil {
		return nil, err
	}

	listOutput, err := exec.CommandContext(ctx, opkg, "list").Output()
	if err != nil {
		return nil, fmt.Errorf("opkg list: %w", err)
	}

	installed, err := loadOpkgInstalled(ctx, opkg)
	if err != nil {
		return nil, err
	}

	available := parseEntwareList(string(listOutput))
	upgradable := map[string]string{}
	if output, upgradeErr := exec.CommandContext(ctx, opkg, "list-upgradable").Output(); upgradeErr == nil {
		upgradable = parseEntwareUpgradable(string(output))
	}

	itemsByName := make(map[string]entwarePackage, len(available)+len(installed))
	for name, item := range available {
		item.Source = "configured-opkg-feed"
		itemsByName[name] = item
	}
	for name, installedVersion := range installed {
		item := itemsByName[name]
		item.Name = name
		item.Source = "configured-opkg-feed"
		item.Installed = true
		item.InstalledVersion = installedVersion
		if version, ok := upgradable[name]; ok && version != "" && version != installedVersion {
			item.Upgradable = true
			item.AvailableVersion = version
		}
		itemsByName[name] = item
	}

	items := make([]entwarePackage, 0, len(itemsByName))
	for _, item := range itemsByName {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Installed != items[j].Installed {
			return items[i].Installed
		}
		return items[i].Name < items[j].Name
	})

	entwareCache.items = append([]entwarePackage(nil), items...)
	entwareCache.at = time.Now()
	return append([]entwarePackage(nil), items...), nil
}

func loadOpkgInstalled(ctx context.Context, opkg string) (map[string]string, error) {
	output, err := exec.CommandContext(ctx, opkg, "list-installed").Output()
	if err != nil {
		return nil, fmt.Errorf("opkg list-installed: %w", err)
	}
	return parseEntwareInstalled(string(output)), nil
}

func parseEntwareList(raw string) map[string]entwarePackage {
	out := map[string]entwarePackage{}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " - ", 3)
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if !safeCatalogPackageName(name) {
			continue
		}
		item := entwarePackage{
			Name:             name,
			AvailableVersion: strings.TrimSpace(parts[1]),
			Source:           "configured-opkg-feed",
		}
		if len(parts) == 3 {
			item.Description = strings.TrimSpace(parts[2])
		}
		out[name] = item
	}
	return out
}

func parseEntwareInstalled(raw string) map[string]string {
	out := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " - ", 3)
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		version := strings.TrimSpace(parts[1])
		if safeCatalogPackageName(name) && version != "" {
			out[name] = version
		}
	}
	return out
}

func parseEntwareUpgradable(raw string) map[string]string {
	out := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " - ", 3)
		if len(parts) != 3 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		version := strings.TrimSpace(parts[2])
		if !safeCatalogPackageName(name) || version == "" {
			continue
		}
		out[name] = version
	}
	return out
}

func loadEntwarePackageDetail(ctx context.Context, name string) (entwarePackageDetail, bool, error) {
	opkg, err := opkgExecutable()
	if err != nil {
		return entwarePackageDetail{}, false, err
	}

	items, err := loadOpkgCatalog(ctx, false)
	if err != nil {
		return entwarePackageDetail{}, false, err
	}
	var catalogMeta entwarePackage
	found := false
	for _, item := range items {
		if item.Name == name {
			catalogMeta = item
			found = true
			break
		}
	}

	detail := entwarePackageDetail{
		Name:             name,
		Description:      catalogMeta.Description,
		AvailableVersion: catalogMeta.AvailableVersion,
		InstalledVersion: catalogMeta.InstalledVersion,
		Installed:        catalogMeta.Installed,
		Upgradable:       catalogMeta.Upgradable,
		MetadataSource:   "opkg-info-status",
	}

	if output, infoErr := exec.CommandContext(ctx, opkg, "info", name).Output(); infoErr == nil {
		if stanza, ok := selectOpkgStanza(parseOpkgControlStanzas(string(output)), name, ""); ok {
			mergeEntwareDetailStanza(&detail, stanza, false)
			found = true
		}
	}

	if output, statusErr := exec.CommandContext(ctx, opkg, "status", name).Output(); statusErr == nil {
		if stanza, ok := selectOpkgStanza(parseOpkgControlStanzas(string(output)), name, "installed"); ok {
			mergeEntwareDetailStanza(&detail, stanza, true)
			found = true
		}
	}

	if detail.AvailableVersion == "" && !detail.Installed {
		detail.AvailableVersion = detail.Version
	}
	if detail.InstalledVersion == "" && detail.Installed {
		detail.InstalledVersion = detail.Version
	}
	return detail, found, nil
}

func mergeEntwareDetailStanza(detail *entwarePackageDetail, stanza map[string]string, installed bool) {
	if detail == nil {
		return
	}
	if value := stanza["Package"]; value != "" {
		detail.Name = value
	}
	if value := stanza["Description"]; value != "" {
		detail.Description = strings.ReplaceAll(value, "\n", " ")
	}
	if value := stanza["Version"]; value != "" {
		detail.Version = value
		if installed {
			detail.InstalledVersion = value
		}
	}
	if value := stanza["Architecture"]; value != "" {
		detail.Architecture = value
	}
	if value := stanza["Status"]; value != "" {
		detail.Status = value
		if strings.HasSuffix(strings.TrimSpace(value), " installed") {
			detail.Installed = true
		}
	}
	if value := stanza["Section"]; value != "" {
		detail.Section = value
	}
	if value := stanza["Maintainer"]; value != "" {
		detail.Maintainer = value
	}
	if value := stanza["Depends"]; value != "" {
		detail.DependsRaw = value
		detail.Depends = flattenDependencyGroups(parseDependencyGroups(value))
	}
	if value := stanza["Size"]; value != "" {
		detail.DownloadSizeBytes = parseOpkgSize(value)
	}
	if value := stanza["Installed-Size"]; value != "" {
		detail.InstalledSizeBytes = parseOpkgSize(value)
	}
	if value := stanza["Source"]; value != "" {
		detail.PackageSource = value
	}
}

func parseOpkgControlStanzas(raw string) []map[string]string {
	var out []map[string]string
	current := map[string]string{}
	lastKey := ""

	flush := func() {
		if len(current) == 0 {
			return
		}
		out = append(out, current)
		current = map[string]string{}
		lastKey = ""
	}

	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && lastKey != "" {
			current[lastKey] = strings.TrimSpace(current[lastKey] + "\n" + strings.TrimSpace(line))
			continue
		}
		index := strings.IndexByte(line, ':')
		if index <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:index])
		value := strings.TrimSpace(line[index+1:])
		current[key] = value
		lastKey = key
	}
	flush()
	return out
}

func selectOpkgStanza(stanzas []map[string]string, name, requiredStatus string) (map[string]string, bool) {
	var fallback map[string]string
	for _, stanza := range stanzas {
		if stanza["Package"] != name {
			continue
		}
		if fallback == nil {
			fallback = stanza
		}
		if requiredStatus == "" || strings.Contains(strings.ToLower(stanza["Status"]), strings.ToLower(requiredStatus)) {
			return stanza, true
		}
	}
	if fallback != nil && requiredStatus == "" {
		return fallback, true
	}
	return nil, false
}

func parseOpkgSize(raw string) uint64 {
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func parseDependencyGroups(raw string) [][]string {
	var groups [][]string
	for _, clause := range strings.Split(raw, ",") {
		var group []string
		for _, alternative := range strings.Split(clause, "|") {
			name := strings.TrimSpace(alternative)
			if index := strings.IndexByte(name, ' '); index >= 0 {
				name = name[:index]
			}
			name = strings.TrimSpace(strings.Trim(name, "()"))
			if safeCatalogPackageName(name) {
				group = append(group, name)
			}
		}
		if len(group) > 0 {
			groups = append(groups, uniqueStrings(group))
		}
	}
	return groups
}

func flattenDependencyGroups(groups [][]string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, group := range groups {
		for _, name := range group {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func uniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func missingDependencyGroups(groups [][]string, installed map[string]string) []string {
	var missing []string
	for _, group := range groups {
		satisfied := false
		for _, name := range group {
			if _, ok := installed[name]; ok {
				satisfied = true
				break
			}
		}
		if !satisfied {
			missing = append(missing, strings.Join(group, " | "))
		}
	}
	return missing
}

func reverseDependencies(stanzas []map[string]string, target string) []string {
	var out []string
	for _, stanza := range stanzas {
		name := strings.TrimSpace(stanza["Package"])
		if !safeCatalogPackageName(name) || name == target {
			continue
		}
		if !strings.HasSuffix(strings.TrimSpace(stanza["Status"]), " installed") {
			continue
		}
		for _, group := range parseDependencyGroups(stanza["Depends"]) {
			for _, dependency := range group {
				if dependency == target {
					out = append(out, name)
					goto nextStanza
				}
			}
		}
	nextStanza:
	}
	sort.Strings(out)
	return uniqueStrings(out)
}

func buildEntwarePreflight(ctx context.Context, name, action string) (entwareActionPreflight, error) {
	preflight := entwareActionPreflight{
		Package:           name,
		Action:            action,
		PackageManagement: marketplaceTestInstallEnabled(),
	}
	if !safeCatalogPackageName(name) {
		preflight.Reason = "invalid package name"
		return preflight, nil
	}
	if entwarePackageProtected(name) {
		preflight.Reason = "package is owned by RouterForge or an App Center integration"
		return preflight, nil
	}

	items, err := loadOpkgCatalog(ctx, true)
	if err != nil {
		return preflight, err
	}
	var meta entwarePackage
	found := false
	for _, item := range items {
		if item.Name == name {
			meta = item
			found = true
			break
		}
	}
	if !found {
		preflight.Reason = "package is not present in configured opkg metadata"
		return preflight, nil
	}

	detail, _, err := loadEntwarePackageDetail(ctx, name)
	if err != nil {
		return preflight, err
	}

	preflight.Installed = meta.Installed
	preflight.Upgradable = meta.Upgradable
	preflight.InstalledVersion = meta.InstalledVersion
	preflight.AvailableVersion = meta.AvailableVersion
	preflight.Architecture = detail.Architecture
	preflight.Dependencies = append([]string(nil), detail.Depends...)
	preflight.DownloadSizeBytes = detail.DownloadSizeBytes
	preflight.InstalledSizeBytes = detail.InstalledSizeBytes

	opkg, err := opkgExecutable()
	if err != nil {
		return preflight, err
	}
	installed, err := loadOpkgInstalled(ctx, opkg)
	if err != nil {
		return preflight, err
	}
	preflight.MissingDependencies = missingDependencyGroups(parseDependencyGroups(detail.DependsRaw), installed)

	if action == "remove" {
		if output, statusErr := exec.CommandContext(ctx, opkg, "status").Output(); statusErr == nil {
			preflight.ReverseDependencies = reverseDependencies(parseOpkgControlStanzas(string(output)), name)
		}
	}

	opt := readPlatformStorage("/opt")
	preflight.OptAvailableBytes = opt.AvailableBytes
	preflight.OptTotalBytes = opt.TotalBytes

	if opt.TotalBytes > 0 && opt.AvailableBytes < entwareDiskFloorBytes {
		preflight.Warnings = append(preflight.Warnings, "low free space on /opt")
	}
	estimated := detail.InstalledSizeBytes
	if estimated == 0 {
		estimated = detail.DownloadSizeBytes * 3
	}
	if estimated > 0 && opt.AvailableBytes > 0 && opt.AvailableBytes < estimated*2 {
		preflight.Warnings = append(preflight.Warnings, "package may require more free space than currently available on /opt")
	}
	if len(preflight.ReverseDependencies) > 0 {
		preflight.Warnings = append(preflight.Warnings, "installed packages depend on this package")
	}
	if len(preflight.MissingDependencies) > 0 && action != "remove" {
		preflight.Warnings = append(preflight.Warnings, "additional dependencies are not currently installed")
	}

	if !preflight.PackageManagement {
		preflight.Reason = "RouterForge package management is disabled"
		return preflight, nil
	}
	switch action {
	case "install":
		if meta.Installed {
			preflight.Reason = "package is already installed"
			return preflight, nil
		}
		if meta.AvailableVersion == "" {
			preflight.Reason = "package has no available version in configured feeds"
			return preflight, nil
		}
	case "update":
		if !meta.Installed {
			preflight.Reason = "package is not installed"
			return preflight, nil
		}
		if !meta.Upgradable {
			preflight.Reason = "no opkg upgrade is available"
			return preflight, nil
		}
	case "remove":
		if !meta.Installed {
			preflight.Reason = "package is not installed"
			return preflight, nil
		}
	default:
		preflight.Reason = "unsupported Entware action"
		return preflight, nil
	}

	preflight.Allowed = true
	return preflight, nil
}

func appCenterIntegrationPackageNames() map[string]struct{} {
	out := map[string]struct{}{}
	for _, item := range integrationCatalog() {
		for _, pkg := range item.Detection.Packages {
			name := strings.ToLower(strings.TrimSpace(pkg))
			if safeCatalogPackageName(name) {
				out[name] = struct{}{}
			}
		}
	}
	return out
}

func entwarePackageProtectedWithIntegrations(name string, integrations map[string]struct{}) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "opkg" || strings.HasPrefix(name, "routerforge-") {
		return true
	}
	_, ok := integrations[name]
	return ok
}

func filterEntwarePackages(items []entwarePackage) []entwarePackage {
	integrations := appCenterIntegrationPackageNames()
	out := make([]entwarePackage, 0, len(items))
	for _, item := range items {
		if entwarePackageProtectedWithIntegrations(item.Name, integrations) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func applyIntegrationPackageVersions(snapshot *catalogSnapshot, packages []entwarePackage) {
	byName := make(map[string]entwarePackage, len(packages))
	for _, pkg := range packages {
		byName[pkg.Name] = pkg
	}

	for i := range snapshot.Integrations {
		meta, ok := integrationPackageVersionCandidate(snapshot.Integrations[i], byName)
		if !ok {
			continue
		}
		item := &snapshot.Integrations[i]
		item.AvailableVersion = meta.AvailableVersion
		item.PackageInstalled = meta.Installed
		if item.Version == "" && meta.InstalledVersion != "" {
			item.Version = meta.InstalledVersion
		}
		item.UpdateAvailable = meta.Installed && meta.Upgradable
	}
}

func integrationPackageVersionCandidate(item catalogItem, byName map[string]entwarePackage) (entwarePackage, bool) {
	matches := make([]entwarePackage, 0, len(item.Detection.Packages))
	installed := make([]entwarePackage, 0, 1)

	for _, name := range item.Detection.Packages {
		meta, ok := byName[name]
		if !ok {
			continue
		}
		matches = append(matches, meta)
		if meta.Installed {
			installed = append(installed, meta)
		}
	}

	if len(installed) == 1 {
		return installed[0], true
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return entwarePackage{}, false
}

func invalidateEntwareCatalog() {
	entwareCache.Lock()
	entwareCache.at = time.Time{}
	entwareCache.items = nil
	entwareCache.Unlock()
}

func entwarePackageProtected(name string) bool {
	return entwarePackageProtectedWithIntegrations(name, appCenterIntegrationPackageNames())
}

func parseEntwareInt(raw string, fallback, min, max int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
