package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	routerForgeRegistrySyncInterval = time.Hour
	routerForgeRegistryMaxBytes     = 2 << 20
)

func routerForgeRegistryRemoteURL() string {
	ref := "dev"
	if normalizedReleaseChannel() == "stable" {
		ref = "main"
	}
	return fmt.Sprintf("https://raw.githubusercontent.com/Fifth-Ace/dns-monitor/%s/marketplace/registry/index.json", ref)
}

func routerForgeRegistryCacheFile() string {
	return "/opt/var/cache/routerforge/marketplace-index-" + normalizedReleaseChannel() + ".json"
}

//go:embed marketplace/registry/index.json
var bundledRouterForgeRegistry []byte

type catalogPublisher struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type catalogTrust struct {
	Status     string `json:"status,omitempty"`
	ReviewedBy string `json:"reviewed_by,omitempty"`
	Note       string `json:"note,omitempty"`
}

type catalogActions struct {
	Install bool   `json:"install"`
	Update  bool   `json:"update"`
	Remove  bool   `json:"remove"`
	Reason  string `json:"reason,omitempty"`
}

type catalogLifecycleStep struct {
	Type          string   `json:"type"`
	Packages      []string `json:"packages,omitempty"`
	Args          []string `json:"args,omitempty"`
	Path          string   `json:"path,omitempty"`
	Content       string   `json:"content,omitempty"`
	IgnoreFailure bool     `json:"ignore_failure,omitempty"`
}

type routerForgeRegistryDocument struct {
	SchemaVersion int           `json:"schema_version"`
	RegistryID    string        `json:"registry_id"`
	Brand         string        `json:"brand"`
	Revision      string        `json:"revision"`
	Entries       []catalogItem `json:"entries"`
}

type routerForgeRegistryStatus struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	Source        string `json:"source"`
	Online        bool   `json:"online"`
	SchemaVersion int    `json:"schema_version"`
	Revision      string `json:"revision,omitempty"`
	LastSync      string `json:"last_sync,omitempty"`
	Error         string `json:"error,omitempty"`
}

var routerForgeRegistryState struct {
	mu          sync.Mutex
	initialized bool
	refreshing  bool
	lastAttempt time.Time
	doc         routerForgeRegistryDocument
	status      routerForgeRegistryStatus
}

func routerForgeRegistrySnapshot() (routerForgeRegistryDocument, routerForgeRegistryStatus) {
	routerForgeRegistryState.mu.Lock()
	defer routerForgeRegistryState.mu.Unlock()

	if !routerForgeRegistryState.initialized {
		doc, err := parseRouterForgeRegistry(bundledRouterForgeRegistry)
		if err != nil {
			panic("invalid bundled RouterForge registry: " + err.Error())
		}
		routerForgeRegistryState.doc = doc
		routerForgeRegistryState.status = routerForgeRegistryStatus{
			ID:            doc.RegistryID,
			URL:           routerForgeRegistryRemoteURL(),
			Source:        "bundled",
			Online:        false,
			SchemaVersion: doc.SchemaVersion,
			Revision:      doc.Revision,
		}

		if data, err := os.ReadFile(routerForgeRegistryCacheFile()); err == nil {
			if cached, parseErr := parseRouterForgeRegistry(data); parseErr == nil {
				routerForgeRegistryState.doc = cached
				routerForgeRegistryState.status.Source = "cache"
				routerForgeRegistryState.status.SchemaVersion = cached.SchemaVersion
				routerForgeRegistryState.status.Revision = cached.Revision
			}
		}
		routerForgeRegistryState.initialized = true
	}

	now := time.Now()
	if !routerForgeRegistryState.refreshing && (routerForgeRegistryState.lastAttempt.IsZero() || now.Sub(routerForgeRegistryState.lastAttempt) >= routerForgeRegistrySyncInterval) {
		routerForgeRegistryState.refreshing = true
		routerForgeRegistryState.lastAttempt = now
		go refreshRouterForgeRegistry()
	}

	return routerForgeRegistryState.doc, routerForgeRegistryState.status
}


func forceRefreshRouterForgeRegistry() routerForgeRegistryStatus {
	routerForgeRegistryState.mu.Lock()
	if !routerForgeRegistryState.initialized {
		routerForgeRegistryState.mu.Unlock()
		_, _ = routerForgeRegistrySnapshot()
		return waitRouterForgeRegistryRefresh(8 * time.Second)
	}
	if routerForgeRegistryState.refreshing {
		routerForgeRegistryState.mu.Unlock()
		return waitRouterForgeRegistryRefresh(8 * time.Second)
	}
	routerForgeRegistryState.refreshing = true
	routerForgeRegistryState.lastAttempt = time.Now()
	routerForgeRegistryState.mu.Unlock()

	refreshRouterForgeRegistry()
	routerForgeRegistryState.mu.Lock()
	status := routerForgeRegistryState.status
	routerForgeRegistryState.mu.Unlock()
	return status
}

func waitRouterForgeRegistryRefresh(timeout time.Duration) routerForgeRegistryStatus {
	deadline := time.Now().Add(timeout)
	for {
		routerForgeRegistryState.mu.Lock()
		refreshing := routerForgeRegistryState.refreshing
		status := routerForgeRegistryState.status
		routerForgeRegistryState.mu.Unlock()
		if !refreshing || time.Now().After(deadline) {
			return status
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func refreshRouterForgeRegistry() {
	ctxClient := &http.Client{Timeout: 6 * time.Second}
	req, err := http.NewRequest(http.MethodGet, routerForgeRegistryRemoteURL(), nil)
	if err == nil {
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "RouterForge/"+version)
	}

	var data []byte
	if err == nil {
		var resp *http.Response
		resp, err = ctxClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("registry HTTP %d", resp.StatusCode)
			} else {
				data, err = io.ReadAll(io.LimitReader(resp.Body, routerForgeRegistryMaxBytes+1))
				if err == nil && len(data) > routerForgeRegistryMaxBytes {
					err = fmt.Errorf("registry exceeds %d bytes", routerForgeRegistryMaxBytes)
				}
			}
		}
	}

	var doc routerForgeRegistryDocument
	if err == nil {
		doc, err = parseRouterForgeRegistry(data)
	}
	if err == nil {
		if mkErr := os.MkdirAll(filepath.Dir(routerForgeRegistryCacheFile()), 0755); mkErr == nil {
			tmp := routerForgeRegistryCacheFile() + ".tmp"
			if writeErr := os.WriteFile(tmp, data, 0644); writeErr == nil {
				_ = os.Rename(tmp, routerForgeRegistryCacheFile())
			} else {
				_ = os.Remove(tmp)
			}
		}
	}

	routerForgeRegistryState.mu.Lock()
	defer routerForgeRegistryState.mu.Unlock()
	routerForgeRegistryState.refreshing = false
	if err != nil {
		routerForgeRegistryState.status.Online = false
		routerForgeRegistryState.status.Error = err.Error()
		return
	}
	routerForgeRegistryState.doc = doc
	routerForgeRegistryState.status = routerForgeRegistryStatus{
		ID:            doc.RegistryID,
		URL:           routerForgeRegistryRemoteURL(),
		Source:        "remote",
		Online:        true,
		SchemaVersion: doc.SchemaVersion,
		Revision:      doc.Revision,
		LastSync:      time.Now().UTC().Format(time.RFC3339),
	}
}

func parseRouterForgeRegistry(data []byte) (routerForgeRegistryDocument, error) {
	var doc routerForgeRegistryDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, err
	}
	if err := validateRouterForgeRegistry(doc); err != nil {
		return doc, err
	}
	return doc, nil
}

func validateRouterForgeRegistry(doc routerForgeRegistryDocument) error {
	if doc.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema_version %d", doc.SchemaVersion)
	}
	if doc.RegistryID != "routerforge-community" {
		return fmt.Errorf("unexpected registry_id %q", doc.RegistryID)
	}
	if doc.Brand != "RouterForge" {
		return fmt.Errorf("unexpected brand %q", doc.Brand)
	}
	if len(doc.Entries) > 512 {
		return fmt.Errorf("too many registry entries")
	}

	seen := make(map[string]struct{}, len(doc.Entries))
	for _, item := range doc.Entries {
		if !safeCatalogID(item.ID) {
			return fmt.Errorf("invalid registry id %q", item.ID)
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("duplicate registry id %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.Kind != "module" && item.Kind != "integration" {
			return fmt.Errorf("%s: invalid kind %q", item.ID, item.Kind)
		}
		if item.Name == "" || item.Publisher.Name == "" {
			return fmt.Errorf("%s: name/publisher required", item.ID)
		}
		if !validTrustStatus(item.Trust.Status) {
			return fmt.Errorf("%s: invalid trust status %q", item.ID, item.Trust.Status)
		}
		if item.Trust.Status == "official" && item.Publisher.ID != "routerforge" {
			return fmt.Errorf("%s: only RouterForge publisher may be official", item.ID)
		}
		for _, plan := range []catalogInstallPlan{item.Install, item.Update, item.Remove} {
			if err := validateCatalogPlan(plan); err != nil {
				return fmt.Errorf("%s: %w", item.ID, err)
			}
		}
	}
	return nil
}

func validateCatalogPlan(plan catalogInstallPlan) error {
	if plan.Method == "" {
		return nil
	}
	switch plan.Method {
	case "routerforge-release", "opkg", "structured", "manual", "official-script", "release-deploy":
	default:
		return fmt.Errorf("unsupported lifecycle method %q", plan.Method)
	}
	for _, pkg := range plan.Packages {
		if !safeCatalogPackageName(pkg) {
			return fmt.Errorf("unsafe package name %q", pkg)
		}
	}
	for _, step := range plan.Steps {
		switch step.Type {
		case "opkg-update", "opkg-install", "opkg-upgrade", "opkg-remove", "write-opkg-feed":
		default:
			return fmt.Errorf("unsupported lifecycle step %q", step.Type)
		}
		for _, pkg := range step.Packages {
			if !safeCatalogPackageName(pkg) {
				return fmt.Errorf("unsafe step package %q", pkg)
			}
		}
		if step.Type == "write-opkg-feed" {
			if !strings.HasPrefix(step.Path, "/opt/etc/opkg/") || strings.Contains(step.Path, "..") {
				return fmt.Errorf("unsafe opkg feed path %q", step.Path)
			}
			if !strings.HasPrefix(strings.TrimSpace(step.Content), "src/gz ") || !strings.Contains(step.Content, "https://") {
				return fmt.Errorf("invalid opkg feed content")
			}
		}
	}
	return nil
}

func safeCatalogID(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

func validTrustStatus(status string) bool {
	switch strings.ToLower(status) {
	case "official", "verified", "unverified", "changed", "blocked", "deprecated":
		return true
	default:
		return false
	}
}

func applyRouterForgeRegistry(snapshot *catalogSnapshot, installed map[string]string, processes map[string]bool, exists func(string) bool) {
	doc, status := routerForgeRegistrySnapshot()
	snapshot.Brand = "RouterForge"
	snapshot.Registry = status

	for i := range snapshot.Modules {
		item := &snapshot.Modules[i]
		if item.Builtin || item.Managed {
			item.Publisher = catalogPublisher{ID: "routerforge", Name: "RouterForge", URL: "https://github.com/Fifth-Ace/dns-monitor"}
			item.Trust = catalogTrust{Status: "official", ReviewedBy: "routerforge", Note: "Встроенный или официальный модуль RouterForge."}
			item.RegistrySource = "builtin-fallback"
		}
	}
	for i := range snapshot.Integrations {
		item := &snapshot.Integrations[i]
		item.Publisher = catalogPublisher{ID: item.Source, Name: "Upstream project", URL: item.ProjectURL}
		item.Trust = catalogTrust{Status: "unverified", Note: "Legacy catalog entry: manifest ещё не перенесён в RouterForge Registry."}
		item.RegistrySource = "legacy-fallback"
	}

	for _, incoming := range doc.Entries {
		if incoming.ID == "" {
			continue
		}
		if target := findCatalogItem(snapshot, incoming.ID, incoming.Kind); target != nil {
			mergeRegistryCatalogItem(target, incoming)
			resetCatalogRuntime(target)
			finalizeCatalogItem(target, installed, processes, exists)
			continue
		}
		if incoming.Builtin {
			continue
		}
		incoming.RegistrySource = status.Source
		resetCatalogRuntime(&incoming)
		finalizeCatalogItem(&incoming, installed, processes, exists)
		if incoming.Kind == "module" {
			snapshot.Modules = append(snapshot.Modules, incoming)
		} else {
			snapshot.Integrations = append(snapshot.Integrations, incoming)
		}
	}

	for i := range snapshot.Modules {
		snapshot.Modules[i].Actions = deriveCatalogActions(snapshot.Modules[i])
	}
	for i := range snapshot.Integrations {
		snapshot.Integrations[i].Actions = deriveCatalogActions(snapshot.Integrations[i])
	}

	sort.SliceStable(snapshot.Modules, func(i, j int) bool {
		return moduleOrder(snapshot.Modules[i].ID) < moduleOrder(snapshot.Modules[j].ID)
	})
	sort.SliceStable(snapshot.Integrations, func(i, j int) bool {
		if snapshot.Integrations[i].Category != snapshot.Integrations[j].Category {
			return snapshot.Integrations[i].Category < snapshot.Integrations[j].Category
		}
		return snapshot.Integrations[i].Name < snapshot.Integrations[j].Name
	})
}

func findCatalogItem(snapshot *catalogSnapshot, id, kind string) *catalogItem {
	if kind == "module" {
		for i := range snapshot.Modules {
			if snapshot.Modules[i].ID == id {
				return &snapshot.Modules[i]
			}
		}
		return nil
	}
	for i := range snapshot.Integrations {
		if snapshot.Integrations[i].ID == id {
			return &snapshot.Integrations[i]
		}
	}
	return nil
}

func mergeRegistryCatalogItem(dst *catalogItem, src catalogItem) {
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.Category != "" {
		dst.Category = src.Category
	}
	if src.Description != "" {
		dst.Description = src.Description
	}
	if src.ProjectURL != "" {
		dst.ProjectURL = src.ProjectURL
	}
	if src.Source != "" {
		dst.Source = src.Source
	}
	if len(src.Capabilities) > 0 {
		dst.Capabilities = append([]string(nil), src.Capabilities...)
	}
	if len(src.Detection.Packages)+len(src.Detection.Services)+len(src.Detection.Paths) > 0 {
		dst.Detection = src.Detection
	}
	if src.Compatibility.Status != "" {
		dst.Compatibility = src.Compatibility
	}
	if len(src.ProcessNames) > 0 {
		dst.ProcessNames = append([]string(nil), src.ProcessNames...)
	}
	if len(src.RunningPaths) > 0 {
		dst.RunningPaths = append([]string(nil), src.RunningPaths...)
	}
	if src.WebRequiresPackage != "" {
		dst.WebRequiresPackage = src.WebRequiresPackage
	}
	if src.WebPort > 0 {
		dst.WebPort = src.WebPort
		dst.WebPortSource = src.WebPortSource
	}
	if src.Install.Method != "" {
		dst.Install = src.Install
	}
	if src.Update.Method != "" {
		dst.Update = src.Update
	}
	if src.Remove.Method != "" {
		dst.Remove = src.Remove
	}
	dst.Publisher = src.Publisher
	dst.Trust = src.Trust
	dst.ManifestID = src.ManifestID
	dst.ManifestSHA256 = src.ManifestSHA256
	dst.ManifestSource = src.ManifestSource
	dst.RegistrySource = src.RegistrySource
	if src.Presentation != nil {
		dst.Presentation = src.Presentation
	}
	dst.PackageAuthoritative = src.PackageAuthoritative
}

func resetCatalogRuntime(item *catalogItem) {
	if item.Builtin {
		return
	}
	item.State = ""
	item.Installed = false
	item.Enabled = false
	item.Version = ""
	item.Service = ""
	item.ServiceRunning = false
}

func deriveCatalogActions(item catalogItem) catalogActions {
	if item.Builtin {
		if item.ID == "routerforge-core" && item.UpdateAvailable && executableCatalogPlan(item.Update) {
			return catalogActions{Update: true}
		}
		return catalogActions{Reason: "Встроенный компонент RouterForge."}
	}
	status := strings.ToLower(item.Trust.Status)
	if status != "official" && status != "verified" {
		reason := "Автоматические действия доступны только для OFFICIAL / VERIFIED manifest."
		if status == "blocked" {
			reason = "Manifest заблокирован RouterForge Registry."
		} else if status == "changed" {
			reason = "Manifest изменён после последнего approval и требует повторной проверки."
		} else if status == "deprecated" {
			reason = "Проект помечен как устаревший."
		}
		return catalogActions{Reason: reason}
	}

	installAllowed := !item.Installed && executableCatalogPlan(item.Install)
	updateAllowed := item.Installed && executableCatalogPlan(item.Update)
	if routerForgePackageForItem(item) != "" {
		installAllowed = installAllowed && item.Release.Version != ""
		updateAllowed = updateAllowed && item.UpdateAvailable
	}

	return catalogActions{
		Install: installAllowed,
		Update:  updateAllowed,
		Remove:  item.Installed && executableCatalogPlan(item.Remove),
	}
}

func executableCatalogPlan(plan catalogInstallPlan) bool {
	if plan.PreviewOnly {
		return false
	}
	switch plan.Method {
	case "routerforge-release", "opkg", "structured":
		return true
	default:
		return false
	}
}
