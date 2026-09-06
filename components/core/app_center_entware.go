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

type entwareActionRequest struct {
	Package string `json:"package"`
	Action  string `json:"action"`
	Confirm string `json:"confirm,omitempty"`
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

	mux.Handle("/api/apps/entware", list)
	mux.Handle("/api/apps/entware/refresh", refresh)
	mux.Handle("/api/apps/entware/action", action)

	// Compatibility aliases: legacy catalog namespace stays available for one
	// transition cycle while the user-facing product is renamed to App Center.
	mux.Handle("/api/catalog/entware", list)
	mux.Handle("/api/catalog/entware/refresh", refresh)
	mux.Handle("/api/catalog/entware/action", action)
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
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))

	if !safeCatalogPackageName(request.Package) {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid package name"})
		return
	}
	if entwarePackageProtected(request.Package) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{
			"error": "RouterForge packages must be managed through the RouterForge tab",
		})
		return
	}

	var verb string
	switch request.Action {
	case "install":
		verb = "install"
	case "update", "upgrade":
		verb = "upgrade"
		request.Action = "update"
	case "remove":
		verb = "remove"
		if request.Confirm != request.Package {
			writeCatalogJSON(w, http.StatusBadRequest, map[string]any{
				"error": "typed removal confirmation does not match package name",
			})
			return
		}
	default:
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported Entware action"})
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

	installed := readInstalledPackages()
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
	writeCatalogJSON(w, http.StatusOK, result)
}

func loadEntwareCatalog(ctx context.Context, force bool) ([]entwarePackage, error) {
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

	available := parseEntwareList(string(listOutput))
	installed := readInstalledPackages()

	upgradable := map[string]string{}
	if output, upgradeErr := exec.CommandContext(ctx, opkg, "list-upgradable").Output(); upgradeErr == nil {
		for name, item := range parseEntwareList(string(output)) {
			upgradable[name] = item.AvailableVersion
		}
	}

	itemsByName := make(map[string]entwarePackage, len(available)+len(installed))
	for name, item := range available {
		item.Source = "entware"
		itemsByName[name] = item
	}
	for name, installedVersion := range installed {
		item := itemsByName[name]
		item.Name = name
		item.Source = "entware"
		item.Installed = true
		item.InstalledVersion = installedVersion
		if version, ok := upgradable[name]; ok {
			item.Upgradable = true
			if version != "" {
				item.AvailableVersion = version
			}
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
			Source:           "entware",
		}
		if len(parts) == 3 {
			item.Description = strings.TrimSpace(parts[2])
		}
		out[name] = item
	}
	return out
}

func invalidateEntwareCatalog() {
	entwareCache.Lock()
	entwareCache.at = time.Time{}
	entwareCache.items = nil
	entwareCache.Unlock()
}

func entwarePackageProtected(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "opkg" || strings.HasPrefix(name, "routerforge-")
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
