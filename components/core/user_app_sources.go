package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	appSourcesSchemaVersion    = 1
	appSourcesAgreementVersion = "2026-09-12"
	appSourceMaxBytes          = 2 << 20
	appSourceMaxSources        = 16
	appSourceMaxEntries        = 256
	appSourceRiskConfirm       = "UNVERIFIED"
)

var (
	appSourcesConfigPath = "/opt/etc/routerforge/app-sources.json"
	appSourcesCacheDir   = "/opt/var/cache/routerforge/app-sources"
	appSourcesMu         sync.Mutex
	appSourceFetchBytes  = fetchAppSourceBytes
	appSourceResolve     = resolveAppSourceRemote
)

//go:embed legal/user-agreement-ru.md
var appSourceAgreementRU string

type appSourcesConfig struct {
	SchemaVersion     int               `json:"schema_version"`
	AllowUnverified   bool              `json:"allow_unverified"`
	AgreementVersion  string            `json:"agreement_version,omitempty"`
	AgreementAccepted string            `json:"agreement_accepted_at,omitempty"`
	Sources           []appSourceRecord `json:"sources"`
}

type appSourceRecord struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	ResolvedURL string `json:"resolved_url,omitempty"`
	RegistryID  string `json:"registry_id,omitempty"`
	Revision    string `json:"revision,omitempty"`
	Trust       string `json:"trust"`
	Enabled     bool   `json:"enabled"`
	ReadOnly    bool   `json:"read_only,omitempty"`
	Online      bool   `json:"online"`
	Cached      bool   `json:"cached,omitempty"`
	EntryCount  int    `json:"entry_count"`
	AddedAt     string `json:"added_at,omitempty"`
	LastSync    string `json:"last_sync,omitempty"`
	Error       string `json:"error,omitempty"`
}

type appSourceCache struct {
	SchemaVersion  int           `json:"schema_version"`
	SourceID       string        `json:"source_id"`
	Kind           string        `json:"kind"`
	RegistryID     string        `json:"registry_id"`
	Name           string        `json:"name"`
	Revision       string        `json:"revision,omitempty"`
	ResolvedURL    string        `json:"resolved_url"`
	ManifestSHA256 string        `json:"manifest_sha256"`
	Entries        []catalogItem `json:"entries"`
}

type thirdPartyRegistryDocument struct {
	SchemaVersion int              `json:"schema_version"`
	RegistryID    string           `json:"registry_id"`
	Name          string           `json:"name,omitempty"`
	Brand         string           `json:"brand,omitempty"`
	Revision      string           `json:"revision,omitempty"`
	Publisher     catalogPublisher `json:"publisher,omitempty"`
	Entries       []catalogItem    `json:"entries"`
}

type thirdPartySingleDocument struct {
	SchemaVersion int         `json:"schema_version"`
	App           catalogItem `json:"app"`
}

type appSourcePreviewEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category,omitempty"`
	Publisher string `json:"publisher,omitempty"`
	Package   string `json:"package,omitempty"`
}

type appSourcePreview struct {
	Kind           string                  `json:"kind"`
	Name           string                  `json:"name"`
	RegistryID     string                  `json:"registry_id"`
	Revision       string                  `json:"revision,omitempty"`
	ResolvedURL    string                  `json:"resolved_url"`
	Trust          string                  `json:"trust"`
	Fingerprint    string                  `json:"fingerprint"`
	EntryCount     int                     `json:"entry_count"`
	Entries        []appSourcePreviewEntry `json:"entries"`
	InstallLimited bool                    `json:"install_limited"`
}

type appSourceMutationRequest struct {
	URL  string `json:"url"`
	Kind string `json:"kind"`
}

type appSourceSecurityRequest struct {
	AllowUnverified  bool   `json:"allow_unverified"`
	Accepted         bool   `json:"accepted"`
	AgreementVersion string `json:"agreement_version"`
}

type appSourceToggleRequest struct {
	Enabled bool `json:"enabled"`
}

func defaultAppSourcesConfig() appSourcesConfig {
	return appSourcesConfig{SchemaVersion: appSourcesSchemaVersion, Sources: []appSourceRecord{}}
}

func loadAppSourcesConfig() (appSourcesConfig, error) {
	appSourcesMu.Lock()
	defer appSourcesMu.Unlock()
	return loadAppSourcesConfigUnlocked()
}

func loadAppSourcesConfigUnlocked() (appSourcesConfig, error) {
	cfg := defaultAppSourcesConfig()
	data, err := os.ReadFile(appSourcesConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultAppSourcesConfig(), fmt.Errorf("parse app sources config: %w", err)
	}
	if cfg.SchemaVersion != appSourcesSchemaVersion {
		return defaultAppSourcesConfig(), fmt.Errorf("unsupported app sources schema_version %d", cfg.SchemaVersion)
	}
	if cfg.Sources == nil {
		cfg.Sources = []appSourceRecord{}
	}
	return cfg, nil
}

func saveAppSourcesConfigUnlocked(cfg appSourcesConfig) error {
	cfg.SchemaVersion = appSourcesSchemaVersion
	if cfg.Sources == nil {
		cfg.Sources = []appSourceRecord{}
	}
	if err := os.MkdirAll(filepath.Dir(appSourcesConfigPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := appSourcesConfigPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, appSourcesConfigPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func appSourceCachePath(id string) string { return filepath.Join(appSourcesCacheDir, id+".json") }

func saveAppSourceCache(cache appSourceCache) error {
	if !strings.HasPrefix(cache.SourceID, "src-") {
		return fmt.Errorf("invalid source id")
	}
	if err := os.MkdirAll(appSourcesCacheDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := appSourceCachePath(cache.SourceID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func loadAppSourceCache(id string) (appSourceCache, error) {
	if !strings.HasPrefix(id, "src-") {
		return appSourceCache{}, fmt.Errorf("invalid source id")
	}
	data, err := os.ReadFile(appSourceCachePath(id))
	if err != nil {
		return appSourceCache{}, err
	}
	var cache appSourceCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return appSourceCache{}, err
	}
	if cache.SchemaVersion != appSourcesSchemaVersion || cache.SourceID != id {
		return appSourceCache{}, fmt.Errorf("invalid source cache metadata")
	}
	return cache, nil
}

func appSourceID(rawURL string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(rawURL)))
	return "src-" + hex.EncodeToString(sum[:6])
}

func validAppSourceKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "auto":
		return "auto"
	case "repository", "repo":
		return "repository"
	case "app", "application", "single":
		return "app"
	default:
		return ""
	}
}

func validateAppSourceURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid source URL: %w", err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("source URL must use https")
	}
	if u.User != nil {
		return nil, fmt.Errorf("credentials in source URL are not allowed")
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("source URL hostname is required")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return nil, fmt.Errorf("local source host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && !appSourcePublicIP(ip) {
		return nil, fmt.Errorf("private, loopback or link-local source address is not allowed")
	}
	return u, nil
}

func appSourcePublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	return !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() && !ip.IsUnspecified() && !ip.IsMulticast()
}

func appSourceDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	if parsed := net.ParseIP(host); parsed != nil {
		ips = []net.IP{parsed}
	} else {
		addrs, resolveErr := net.DefaultResolver.LookupIPAddr(ctx, host)
		if resolveErr != nil {
			return nil, resolveErr
		}
		for _, addr := range addrs {
			ips = append(ips, addr.IP)
		}
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	for _, ip := range ips {
		if !appSourcePublicIP(ip) {
			continue
		}
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		err = dialErr
	}
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("source host resolved only to disallowed addresses")
}

func newAppSourceHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:               nil,
		DialContext:         appSourceDialContext,
		TLSHandshakeTimeout: 5 * time.Second,
		IdleConnTimeout:     20 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   8 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many source redirects")
			}
			_, err := validateAppSourceURL(req.URL.String())
			return err
		},
	}
}

func fetchAppSourceBytes(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := validateAppSourceURL(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "RouterForge-AppCenter/1")
	resp, err := newAppSourceHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("source HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, appSourceMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > appSourceMaxBytes {
		return nil, fmt.Errorf("source document exceeds %d bytes", appSourceMaxBytes)
	}
	return data, nil
}

func githubRepositoryParts(raw string) (string, string, bool) {
	u, err := validateAppSourceURL(raw)
	if err != nil || !strings.EqualFold(u.Hostname(), "github.com") {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", false
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSuffix(strings.TrimSpace(parts[1]), ".git")
	if owner == "" || repo == "" {
		return "", "", false
	}
	return owner, repo, true
}

func githubPathEscape(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func fetchGitHubRepositoryFile(ctx context.Context, owner, repo, path, branch string) ([]byte, string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		url.PathEscape(owner), url.PathEscape(repo), githubPathEscape(path), url.QueryEscape(branch))
	data, err := appSourceFetchBytes(ctx, apiURL)
	if err != nil {
		return nil, "", err
	}
	var response struct {
		Content     string `json:"content"`
		Encoding    string `json:"encoding"`
		DownloadURL string `json:"download_url"`
		Type        string `json:"type"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, "", err
	}
	if response.Type != "file" || response.Encoding != "base64" {
		return nil, "", fmt.Errorf("GitHub manifest is not a base64 file")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(response.Content, "\n", ""))
	if err != nil {
		return nil, "", err
	}
	if len(decoded) > appSourceMaxBytes {
		return nil, "", fmt.Errorf("GitHub manifest exceeds size limit")
	}
	resolved := response.DownloadURL
	if resolved == "" {
		resolved = apiURL
	}
	return decoded, resolved, nil
}

func resolveGitHubSource(ctx context.Context, rawURL, kind string) (appSourceCache, error) {
	owner, repo, ok := githubRepositoryParts(rawURL)
	if !ok {
		return appSourceCache{}, fmt.Errorf("not a GitHub repository URL")
	}
	metaURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	metaData, err := appSourceFetchBytes(ctx, metaURL)
	if err != nil {
		return appSourceCache{}, fmt.Errorf("GitHub repository metadata: %w", err)
	}
	var meta struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(metaData, &meta); err != nil || strings.TrimSpace(meta.DefaultBranch) == "" {
		return appSourceCache{}, fmt.Errorf("GitHub repository default branch unavailable")
	}
	type candidate struct{ kind, path string }
	var candidates []candidate
	if kind == "auto" || kind == "repository" {
		candidates = append(candidates,
			candidate{"repository", ".routerforge/index.json"},
			candidate{"repository", "routerforge-index.json"})
	}
	if kind == "auto" || kind == "app" {
		candidates = append(candidates,
			candidate{"app", ".routerforge/manifest.json"},
			candidate{"app", "routerforge.json"})
	}
	var lastErr error
	for _, candidate := range candidates {
		data, resolved, fetchErr := fetchGitHubRepositoryFile(ctx, owner, repo, candidate.path, meta.DefaultBranch)
		if fetchErr != nil {
			lastErr = fetchErr
			continue
		}
		cache, parseErr := parseThirdPartySourceDocument(data, resolved, candidate.kind)
		if parseErr == nil {
			return cache, nil
		}
		lastErr = parseErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("RouterForge manifest not found")
	}
	return appSourceCache{}, fmt.Errorf("GitHub source: %w", lastErr)
}

func resolveAppSourceRemote(ctx context.Context, rawURL, kind string) (appSourceCache, error) {
	kind = validAppSourceKind(kind)
	if kind == "" {
		return appSourceCache{}, fmt.Errorf("invalid source kind")
	}
	if _, _, ok := githubRepositoryParts(rawURL); ok {
		return resolveGitHubSource(ctx, rawURL, kind)
	}
	data, err := appSourceFetchBytes(ctx, rawURL)
	if err != nil {
		return appSourceCache{}, err
	}
	return parseThirdPartySourceDocument(data, rawURL, kind)
}

func parseThirdPartySourceDocument(data []byte, resolvedURL, requestedKind string) (appSourceCache, error) {
	requestedKind = validAppSourceKind(requestedKind)
	if requestedKind == "" {
		return appSourceCache{}, fmt.Errorf("invalid source kind")
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return appSourceCache{}, fmt.Errorf("parse source JSON: %w", err)
	}

	var kind, registryID, name, revision string
	var entries []catalogItem

	if _, ok := probe["entries"]; ok {
		if requestedKind == "app" {
			return appSourceCache{}, fmt.Errorf("repository index supplied where single app was requested")
		}
		var doc thirdPartyRegistryDocument
		if err := json.Unmarshal(data, &doc); err != nil {
			return appSourceCache{}, err
		}
		if doc.SchemaVersion != 1 {
			return appSourceCache{}, fmt.Errorf("unsupported schema_version %d", doc.SchemaVersion)
		}
		registryID = strings.TrimSpace(doc.RegistryID)
		if !safeCatalogID(registryID) || reservedThirdPartyRegistryID(registryID) {
			return appSourceCache{}, fmt.Errorf("invalid or reserved registry_id %q", registryID)
		}
		if len(doc.Entries) < 1 || len(doc.Entries) > appSourceMaxEntries {
			return appSourceCache{}, fmt.Errorf("registry entries must be between 1 and %d", appSourceMaxEntries)
		}
		kind = "repository"
		name = strings.TrimSpace(doc.Name)
		if name == "" {
			name = strings.TrimSpace(doc.Brand)
		}
		if name == "" {
			name = registryID
		}
		revision = strings.TrimSpace(doc.Revision)
		entries = append([]catalogItem(nil), doc.Entries...)
	} else if _, ok := probe["app"]; ok {
		if requestedKind == "repository" {
			return appSourceCache{}, fmt.Errorf("single app supplied where repository was requested")
		}
		var doc thirdPartySingleDocument
		if err := json.Unmarshal(data, &doc); err != nil {
			return appSourceCache{}, err
		}
		if doc.SchemaVersion != 1 {
			return appSourceCache{}, fmt.Errorf("unsupported schema_version %d", doc.SchemaVersion)
		}
		kind = "app"
		registryID = "single"
		name = strings.TrimSpace(doc.App.Name)
		entries = []catalogItem{doc.App}
	} else {
		return appSourceCache{}, fmt.Errorf("source must contain either entries or app")
	}

	seen := map[string]struct{}{}
	for i := range entries {
		if err := validateThirdPartyCatalogItem(entries[i]); err != nil {
			return appSourceCache{}, fmt.Errorf("entry %d: %w", i+1, err)
		}
		if _, exists := seen[entries[i].ID]; exists {
			return appSourceCache{}, fmt.Errorf("duplicate app id %q", entries[i].ID)
		}
		seen[entries[i].ID] = struct{}{}
		entries[i].Trust = catalogTrust{
			Status: "unverified",
			Note:   "User-added source; publisher identity has not been verified by RouterForge.",
		}
		entries[i].Builtin = false
		entries[i].Managed = false
		if len(entries[i].Detection.Packages) == 0 &&
			entries[i].Install.Method == "opkg" &&
			len(entries[i].Install.Packages) == 1 {
			entries[i].Detection.Packages = append([]string(nil), entries[i].Install.Packages...)
		}
	}

	sum := sha256.Sum256(data)
	return appSourceCache{
		SchemaVersion:  appSourcesSchemaVersion,
		Kind:           kind,
		RegistryID:     registryID,
		Name:           name,
		Revision:       revision,
		ResolvedURL:    resolvedURL,
		ManifestSHA256: hex.EncodeToString(sum[:]),
		Entries:        entries,
	}, nil
}

func reservedThirdPartyRegistryID(id string) bool {
	value := strings.ToLower(strings.TrimSpace(id))
	return value == "routerforge" ||
		value == "routerforge-community" ||
		strings.HasPrefix(value, "routerforge-")
}

func validateThirdPartyCatalogItem(item catalogItem) error {
	if !safeCatalogID(item.ID) {
		return fmt.Errorf("invalid app id %q", item.ID)
	}
	if item.Kind != "integration" {
		return fmt.Errorf("%s: R2 third-party sources support kind=integration only", item.ID)
	}
	if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Publisher.Name) == "" {
		return fmt.Errorf("%s: name and publisher.name are required", item.ID)
	}
	if strings.EqualFold(item.Publisher.ID, "routerforge") {
		return fmt.Errorf("%s: reserved RouterForge publisher namespace", item.ID)
	}
	if item.Builtin || item.Managed {
		return fmt.Errorf("%s: third-party app cannot declare builtin/managed", item.ID)
	}
	if status := strings.ToLower(strings.TrimSpace(item.Trust.Status)); status != "" && status != "unverified" {
		return fmt.Errorf("%s: third-party source cannot self-declare trust=%q", item.ID, status)
	}
	if err := validateCatalogWebMetadata(item.Web); err != nil {
		return fmt.Errorf("%s: %w", item.ID, err)
	}
	for _, plan := range []catalogInstallPlan{item.Install, item.Update, item.Remove} {
		if err := validateThirdPartyCatalogPlan(plan); err != nil {
			return fmt.Errorf("%s: %w", item.ID, err)
		}
	}
	for _, pkg := range append(append([]string(nil), item.Detection.Packages...), item.Conflicts...) {
		if !safeCatalogPackageName(pkg) || pkg == "opkg" || strings.HasPrefix(pkg, "routerforge-") {
			return fmt.Errorf("%s: unsafe package %q", item.ID, pkg)
		}
	}
	return nil
}

func validateThirdPartyCatalogPlan(plan catalogInstallPlan) error {
	switch plan.Method {
	case "", "manual":
		return nil
	case "opkg":
	default:
		return fmt.Errorf("unverified source lifecycle method %q is not allowed", plan.Method)
	}
	if len(plan.Steps) > 0 {
		return fmt.Errorf("unverified source cannot declare structured lifecycle steps")
	}
	for _, pkg := range plan.Packages {
		if !safeCatalogPackageName(pkg) || pkg == "opkg" || strings.HasPrefix(pkg, "routerforge-") {
			return fmt.Errorf("unsafe package %q", pkg)
		}
	}
	return nil
}

func previewFromAppSourceCache(cache appSourceCache) appSourcePreview {
	entries := make([]appSourcePreviewEntry, 0, len(cache.Entries))
	for _, item := range cache.Entries {
		pkg := ""
		if len(item.Detection.Packages) > 0 {
			pkg = item.Detection.Packages[0]
		} else if len(item.Install.Packages) > 0 {
			pkg = item.Install.Packages[0]
		}
		entries = append(entries, appSourcePreviewEntry{
			ID:        item.ID,
			Name:      item.Name,
			Category:  item.Category,
			Publisher: item.Publisher.Name,
			Package:   pkg,
		})
	}
	return appSourcePreview{
		Kind:           cache.Kind,
		Name:           cache.Name,
		RegistryID:     cache.RegistryID,
		Revision:       cache.Revision,
		ResolvedURL:    cache.ResolvedURL,
		Trust:          "unsigned",
		Fingerprint:    cache.ManifestSHA256,
		EntryCount:     len(cache.Entries),
		Entries:        entries,
		InstallLimited: true,
	}
}

func normalizeUserSourceItem(source appSourceRecord, cache appSourceCache, original catalogItem) catalogItem {
	item := original
	manifestID := item.ID
	item.ID = source.ID + ":" + manifestID
	item.ManifestID = manifestID
	item.ManifestSHA256 = cache.ManifestSHA256
	item.ManifestSource = cache.ResolvedURL
	item.RegistrySource = source.ID
	item.Source = "user-source"
	item.Builtin = false
	item.Managed = false
	item.Trust = catalogTrust{
		Status: "unverified",
		Note:   "User-added source. Installation requires explicit unsafe-source permission.",
	}
	return item
}

func applyUserAppSources(snapshot *catalogSnapshot, installed map[string]string, processes map[string]bool, exists func(string) bool) {
	cfg, err := loadAppSourcesConfig()
	if err != nil {
		return
	}
	for _, source := range cfg.Sources {
		if !source.Enabled {
			continue
		}
		cache, cacheErr := loadAppSourceCache(source.ID)
		if cacheErr != nil {
			continue
		}
		for _, original := range cache.Entries {
			item := normalizeUserSourceItem(source, cache, original)
			resetCatalogRuntime(&item)
			finalizeCatalogItem(&item, installed, processes, exists)
			snapshot.Integrations = append(snapshot.Integrations, item)
		}
	}
}

func appSourceApplyActionPolicy(item *catalogItem) {
	if item == nil || !strings.HasPrefix(item.RegistrySource, "src-") {
		return
	}
	if strings.ToLower(item.Trust.Status) != "unverified" {
		return
	}
	cfg, err := loadAppSourcesConfig()
	if err != nil || !cfg.AllowUnverified {
		item.Actions.Install = false
		item.Actions.Update = false
		if item.Actions.Reason == "" || strings.Contains(item.Actions.Reason, "No verified manifest") {
			item.Actions.Reason = "Installation from unverified sources is disabled. Open App Center Sources to enable it."
		}
	}
}

func appSourceActionBlockReason(item catalogItem, action, confirm string) string {
	if !strings.HasPrefix(item.RegistrySource, "src-") {
		return ""
	}
	if action == "remove" {
		return ""
	}
	if strings.ToLower(item.Trust.Status) != "unverified" {
		return "unsupported third-party trust state"
	}
	cfg, err := loadAppSourcesConfig()
	if err != nil || !cfg.AllowUnverified {
		return "installation from unverified sources is disabled"
	}
	if confirm != appSourceRiskConfirm {
		return "explicit unverified-source confirmation is required"
	}
	return ""
}

func userAppSourcePackageNames() map[string]struct{} {
	out := map[string]struct{}{}
	cfg, err := loadAppSourcesConfig()
	if err != nil {
		return out
	}
	for _, source := range cfg.Sources {
		if !source.Enabled {
			continue
		}
		cache, cacheErr := loadAppSourceCache(source.ID)
		if cacheErr != nil {
			continue
		}
		for _, item := range cache.Entries {
			for _, pkg := range item.Detection.Packages {
				if safeCatalogPackageName(pkg) {
					out[pkg] = struct{}{}
				}
			}
			for _, pkg := range item.Install.Packages {
				if safeCatalogPackageName(pkg) {
					out[pkg] = struct{}{}
				}
			}
		}
	}
	return out
}

func listAppSources() (map[string]any, error) {
	cfg, err := loadAppSourcesConfig()
	if err != nil {
		return nil, err
	}
	doc, officialStatus := routerForgeRegistrySnapshot()
	official := appSourceRecord{
		ID:          "routerforge-official",
		Kind:        "repository",
		Name:        "RouterForge Official",
		URL:         officialStatus.URL,
		ResolvedURL: officialStatus.URL,
		RegistryID:  doc.RegistryID,
		Revision:    doc.Revision,
		Trust:       "official",
		Enabled:     true,
		ReadOnly:    true,
		Online:      officialStatus.Online,
		Cached:      officialStatus.Source == "cache" || officialStatus.Source == "bundled",
		EntryCount:  len(doc.Entries),
		LastSync:    officialStatus.LastSync,
		Error:       officialStatus.Error,
	}
	sources := make([]appSourceRecord, 0, len(cfg.Sources)+1)
	sources = append(sources, official)
	for _, source := range cfg.Sources {
		if _, cacheErr := loadAppSourceCache(source.ID); cacheErr == nil {
			source.Cached = true
		}
		source.Online = source.Error == "" && source.LastSync != ""
		sources = append(sources, source)
	}
	return map[string]any{
		"schema_version":        cfg.SchemaVersion,
		"allow_unverified":      cfg.AllowUnverified,
		"agreement_version":     appSourcesAgreementVersion,
		"agreement_accepted_at": cfg.AgreementAccepted,
		"sources":               sources,
	}, nil
}

func addAppSource(ctx context.Context, request appSourceMutationRequest) (appSourceRecord, appSourcePreview, error) {
	kind := validAppSourceKind(request.Kind)
	if kind == "" {
		return appSourceRecord{}, appSourcePreview{}, fmt.Errorf("invalid source kind")
	}
	rawURL := strings.TrimSpace(request.URL)
	if _, err := validateAppSourceURL(rawURL); err != nil {
		return appSourceRecord{}, appSourcePreview{}, err
	}
	cache, err := appSourceResolve(ctx, rawURL, kind)
	if err != nil {
		return appSourceRecord{}, appSourcePreview{}, err
	}

	appSourcesMu.Lock()
	defer appSourcesMu.Unlock()
	cfg, err := loadAppSourcesConfigUnlocked()
	if err != nil {
		return appSourceRecord{}, appSourcePreview{}, err
	}
	if len(cfg.Sources) >= appSourceMaxSources {
		return appSourceRecord{}, appSourcePreview{}, fmt.Errorf("maximum source count is %d", appSourceMaxSources)
	}
	id := appSourceID(rawURL)
	for _, existing := range cfg.Sources {
		if existing.ID == id || strings.EqualFold(existing.URL, rawURL) {
			return appSourceRecord{}, appSourcePreview{}, fmt.Errorf("source is already configured")
		}
	}
	cache.SourceID = id
	now := time.Now().UTC().Format(time.RFC3339)
	source := appSourceRecord{
		ID:          id,
		Kind:        cache.Kind,
		Name:        cache.Name,
		URL:         rawURL,
		ResolvedURL: cache.ResolvedURL,
		RegistryID:  cache.RegistryID,
		Revision:    cache.Revision,
		Trust:       "unsigned",
		Enabled:     true,
		Online:      true,
		Cached:      true,
		EntryCount:  len(cache.Entries),
		AddedAt:     now,
		LastSync:    now,
	}
	if err := saveAppSourceCache(cache); err != nil {
		return appSourceRecord{}, appSourcePreview{}, err
	}
	cfg.Sources = append(cfg.Sources, source)
	if err := saveAppSourcesConfigUnlocked(cfg); err != nil {
		_ = os.Remove(appSourceCachePath(id))
		return appSourceRecord{}, appSourcePreview{}, err
	}
	return source, previewFromAppSourceCache(cache), nil
}

func refreshOneAppSource(ctx context.Context, id string) (appSourceRecord, error) {
	cfg, err := loadAppSourcesConfig()
	if err != nil {
		return appSourceRecord{}, err
	}
	var source appSourceRecord
	found := false
	for _, candidate := range cfg.Sources {
		if candidate.ID == id {
			source = candidate
			found = true
			break
		}
	}
	if !found {
		return appSourceRecord{}, fmt.Errorf("source not found")
	}

	cache, resolveErr := appSourceResolve(ctx, source.URL, source.Kind)

	appSourcesMu.Lock()
	defer appSourcesMu.Unlock()
	cfg, err = loadAppSourcesConfigUnlocked()
	if err != nil {
		return source, err
	}
	index := -1
	for i := range cfg.Sources {
		if cfg.Sources[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return source, fmt.Errorf("source disappeared during refresh")
	}

	if resolveErr != nil {
		cfg.Sources[index].Error = resolveErr.Error()
		cfg.Sources[index].Online = false
		cfg.Sources[index].Cached = true
		_ = saveAppSourcesConfigUnlocked(cfg)
		return cfg.Sources[index], resolveErr
	}
	cache.SourceID = id
	if err := saveAppSourceCache(cache); err != nil {
		return cfg.Sources[index], err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	cfg.Sources[index].Kind = cache.Kind
	cfg.Sources[index].Name = cache.Name
	cfg.Sources[index].ResolvedURL = cache.ResolvedURL
	cfg.Sources[index].RegistryID = cache.RegistryID
	cfg.Sources[index].Revision = cache.Revision
	cfg.Sources[index].EntryCount = len(cache.Entries)
	cfg.Sources[index].LastSync = now
	cfg.Sources[index].Error = ""
	cfg.Sources[index].Online = true
	cfg.Sources[index].Cached = true
	if err := saveAppSourcesConfigUnlocked(cfg); err != nil {
		return cfg.Sources[index], err
	}
	return cfg.Sources[index], nil
}

func forceRefreshUserAppSources() {
	cfg, err := loadAppSourcesConfig()
	if err != nil {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, source := range cfg.Sources {
		if !source.Enabled {
			continue
		}
		sourceID := source.ID
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = refreshOneAppSource(ctx, sourceID)
		}()
	}
	wg.Wait()
}

func setAppSourceEnabled(id string, enabled bool) (appSourceRecord, error) {
	appSourcesMu.Lock()
	defer appSourcesMu.Unlock()
	cfg, err := loadAppSourcesConfigUnlocked()
	if err != nil {
		return appSourceRecord{}, err
	}
	for i := range cfg.Sources {
		if cfg.Sources[i].ID != id {
			continue
		}
		cfg.Sources[i].Enabled = enabled
		if err := saveAppSourcesConfigUnlocked(cfg); err != nil {
			return appSourceRecord{}, err
		}
		return cfg.Sources[i], nil
	}
	return appSourceRecord{}, fmt.Errorf("source not found")
}

func removeAppSource(id string) error {
	if !strings.HasPrefix(id, "src-") {
		return fmt.Errorf("built-in source cannot be removed")
	}
	appSourcesMu.Lock()
	defer appSourcesMu.Unlock()
	cfg, err := loadAppSourcesConfigUnlocked()
	if err != nil {
		return err
	}
	next := make([]appSourceRecord, 0, len(cfg.Sources))
	found := false
	for _, source := range cfg.Sources {
		if source.ID == id {
			found = true
			continue
		}
		next = append(next, source)
	}
	if !found {
		return fmt.Errorf("source not found")
	}
	cfg.Sources = next
	if err := saveAppSourcesConfigUnlocked(cfg); err != nil {
		return err
	}
	_ = os.Remove(appSourceCachePath(id))
	return nil
}

func setAppSourceSecurity(request appSourceSecurityRequest) (appSourcesConfig, error) {
	appSourcesMu.Lock()
	defer appSourcesMu.Unlock()
	cfg, err := loadAppSourcesConfigUnlocked()
	if err != nil {
		return cfg, err
	}
	if request.AllowUnverified {
		if !request.Accepted || request.AgreementVersion != appSourcesAgreementVersion {
			return cfg, fmt.Errorf("current agreement and risk warning must be accepted")
		}
		cfg.AllowUnverified = true
		cfg.AgreementVersion = appSourcesAgreementVersion
		cfg.AgreementAccepted = time.Now().UTC().Format(time.RFC3339)
	} else {
		cfg.AllowUnverified = false
	}
	if err := saveAppSourcesConfigUnlocked(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func registerAppSourceHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/apps/sources", handleAppSources)
	mux.HandleFunc("/api/apps/sources/preview", handleAppSourcePreview)
	mux.HandleFunc("/api/apps/sources/security", handleAppSourceSecurity)
	mux.HandleFunc("/api/apps/sources/", handleAppSourcePath)
	mux.HandleFunc("/api/apps/legal", handleAppLegal)
}

func handleAppSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		state, err := listAppSources()
		if err != nil {
			writeCatalogJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeCatalogJSON(w, http.StatusOK, state)
	case http.MethodPost:
		if !sameOriginRequest(r) {
			writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin source mutation rejected"})
			return
		}
		var request appSourceMutationRequest
		if err := decodeSmallJSON(w, r, &request); err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		source, preview, err := addAppSource(ctx, request)
		if err != nil {
			writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeCatalogJSON(w, http.StatusCreated, map[string]any{
			"source":  source,
			"preview": preview,
			"catalog": refreshCatalog(),
		})
	default:
		w.Header().Set("Allow", "GET, POST")
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET or POST required"})
	}
}

func handleAppSourcePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if !sameOriginRequest(r) {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin source preview rejected"})
		return
	}
	var request appSourceMutationRequest
	if err := decodeSmallJSON(w, r, &request); err != nil {
		return
	}
	kind := validAppSourceKind(request.Kind)
	if kind == "" {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid source kind"})
		return
	}
	if _, err := validateAppSourceURL(request.URL); err != nil {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	cache, err := appSourceResolve(ctx, strings.TrimSpace(request.URL), kind)
	if err != nil {
		writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeCatalogJSON(w, http.StatusOK, previewFromAppSourceCache(cache))
}

func handleAppSourceSecurity(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := loadAppSourcesConfig()
		if err != nil {
			writeCatalogJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeCatalogJSON(w, http.StatusOK, map[string]any{
			"allow_unverified":      cfg.AllowUnverified,
			"agreement_version":     appSourcesAgreementVersion,
			"agreement_accepted_at": cfg.AgreementAccepted,
		})
	case http.MethodPost:
		if !sameOriginRequest(r) {
			writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin source security mutation rejected"})
			return
		}
		var request appSourceSecurityRequest
		if err := decodeSmallJSON(w, r, &request); err != nil {
			return
		}
		cfg, err := setAppSourceSecurity(request)
		if err != nil {
			writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeCatalogJSON(w, http.StatusOK, map[string]any{
			"allow_unverified":      cfg.AllowUnverified,
			"agreement_version":     appSourcesAgreementVersion,
			"agreement_accepted_at": cfg.AgreementAccepted,
			"catalog":               refreshCatalog(),
		})
	default:
		w.Header().Set("Allow", "GET, POST")
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET or POST required"})
	}
}

func handleAppSourcePath(w http.ResponseWriter, r *http.Request) {
	suffix := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/apps/sources/"), "/")
	if suffix == "" {
		writeCatalogJSON(w, http.StatusNotFound, map[string]any{"error": "source id required"})
		return
	}
	parts := strings.Split(suffix, "/")
	id := parts[0]
	if !strings.HasPrefix(id, "src-") {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "built-in source is read-only"})
		return
	}
	if !sameOriginRequest(r) && r.Method != http.MethodGet {
		writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "cross-origin source mutation rejected"})
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := removeAppSource(id); err != nil {
			writeCatalogJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
			return
		}
		writeCatalogJSON(w, http.StatusOK, map[string]any{"ok": true, "catalog": refreshCatalog()})
		return
	}

	if len(parts) != 2 || r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, DELETE")
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "unsupported source operation"})
		return
	}

	switch parts[1] {
	case "refresh":
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		source, err := refreshOneAppSource(ctx, id)
		if err != nil {
			writeCatalogJSON(w, http.StatusBadGateway, map[string]any{
				"error":  err.Error(),
				"source": source,
				"cached": source.Cached,
			})
			return
		}
		writeCatalogJSON(w, http.StatusOK, map[string]any{
			"source":  source,
			"catalog": refreshCatalog(),
		})
	case "toggle":
		var request appSourceToggleRequest
		if err := decodeSmallJSON(w, r, &request); err != nil {
			return
		}
		source, err := setAppSourceEnabled(id, request.Enabled)
		if err != nil {
			writeCatalogJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
			return
		}
		writeCatalogJSON(w, http.StatusOK, map[string]any{
			"source":  source,
			"catalog": refreshCatalog(),
		})
	default:
		writeCatalogJSON(w, http.StatusNotFound, map[string]any{"error": "unknown source operation"})
	}
}

func handleAppLegal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required"})
		return
	}
	writeCatalogJSON(w, http.StatusOK, map[string]any{
		"agreement_version": appSourcesAgreementVersion,
		"license":           "MIT",
		"as_is":             true,
		"text":              appSourceAgreementRU,
	})
}
