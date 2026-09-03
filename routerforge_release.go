package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var releaseChannel = "beta"

const (
	routerForgeReleaseSyncInterval = time.Hour
	routerForgeReleaseMaxBytes     = 512 << 10
	routerForgeCanonicalRepository = "Fifth-Ace/routerforge"
	routerForgeLegacyRepository    = "Fifth-Ace/dns-monitor"
)

type catalogRelease struct {
	Channel        string `json:"channel,omitempty"`
	Version        string `json:"version,omitempty"`
	Package        string `json:"package,omitempty"`
	Asset          string `json:"asset,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	URL            string `json:"url,omitempty"`
	CanonicalURL   string `json:"canonical_url,omitempty"`
	MinCoreVersion string `json:"min_core_version,omitempty"`
}

type routerForgeReleaseIndex struct {
	SchemaVersion int              `json:"schema_version"`
	Channel       string           `json:"channel"`
	GeneratedAt   string           `json:"generated_at,omitempty"`
	Commit        string           `json:"commit,omitempty"`
	Components    []catalogRelease `json:"components"`
}

type routerForgeReleaseStatus struct {
	Channel  string `json:"channel"`
	URL      string `json:"url"`
	Source   string `json:"source"`
	Online   bool   `json:"online"`
	LastSync string `json:"last_sync,omitempty"`
	Error    string `json:"error,omitempty"`
}

var routerForgeReleaseState struct {
	mu          sync.Mutex
	initialized bool
	refreshing  bool
	lastAttempt time.Time
	doc         routerForgeReleaseIndex
	status      routerForgeReleaseStatus
}

func normalizedReleaseChannel() string {
	switch strings.ToLower(strings.TrimSpace(releaseChannel)) {
	case "stable":
		return "stable"
	case "beta":
		return "beta"
	default:
		return "beta"
	}
}

func routerForgeReleaseIndexURLs() []string {
	channel := normalizedReleaseChannel()
	repositories := []string{
		routerForgeCanonicalRepository,
		routerForgeLegacyRepository,
	}
	urls := make([]string, 0, len(repositories))
	for _, repository := range repositories {
		urls = append(urls, fmt.Sprintf(
			"https://github.com/%s/releases/download/routerforge-%s/routerforge-%s-index.json",
			repository, channel, channel,
		))
	}
	return urls
}

func routerForgeReleaseIndexURL() string {
	return routerForgeReleaseIndexURLs()[0]
}

func validRouterForgeReleaseURL(value string) bool {
	for _, repository := range []string{routerForgeCanonicalRepository, routerForgeLegacyRepository} {
		if strings.HasPrefix(value, "https://github.com/"+repository+"/releases/download/") {
			return true
		}
	}
	return false
}

func routerForgeReleaseDownloadURLs(release catalogRelease) []string {
	urls := make([]string, 0, 2)
	for _, candidate := range []string{release.CanonicalURL, release.URL} {
		if candidate == "" || !validRouterForgeReleaseURL(candidate) {
			continue
		}
		duplicate := false
		for _, existing := range urls {
			if existing == candidate {
				duplicate = true
				break
			}
		}
		if !duplicate {
			urls = append(urls, candidate)
		}
	}
	return urls
}

func routerForgeReleaseCachePath() string {
	return "/opt/var/cache/routerforge/release-index-" + normalizedReleaseChannel() + ".json"
}

func routerForgeReleaseSnapshot() (routerForgeReleaseIndex, routerForgeReleaseStatus) {
	routerForgeReleaseState.mu.Lock()
	defer routerForgeReleaseState.mu.Unlock()

	if !routerForgeReleaseState.initialized {
		channel := normalizedReleaseChannel()
		routerForgeReleaseState.doc = routerForgeReleaseIndex{
			SchemaVersion: 1,
			Channel:       channel,
			Components:    []catalogRelease{},
		}
		routerForgeReleaseState.status = routerForgeReleaseStatus{
			Channel: channel,
			URL:     routerForgeReleaseIndexURL(),
			Source:  "none",
			Online:  false,
		}

		if data, err := os.ReadFile(routerForgeReleaseCachePath()); err == nil {
			if cached, parseErr := parseRouterForgeReleaseIndex(data); parseErr == nil {
				routerForgeReleaseState.doc = cached
				routerForgeReleaseState.status.Source = "cache"
			}
		}
		routerForgeReleaseState.initialized = true
	}

	now := time.Now()
	if !routerForgeReleaseState.refreshing &&
		(routerForgeReleaseState.lastAttempt.IsZero() ||
			now.Sub(routerForgeReleaseState.lastAttempt) >= routerForgeReleaseSyncInterval) {
		routerForgeReleaseState.refreshing = true
		routerForgeReleaseState.lastAttempt = now
		go refreshRouterForgeReleaseIndex()
	}

	return routerForgeReleaseState.doc, routerForgeReleaseState.status
}

func forceRefreshRouterForgeReleaseIndex() routerForgeReleaseStatus {
	routerForgeReleaseState.mu.Lock()
	if !routerForgeReleaseState.initialized {
		routerForgeReleaseState.mu.Unlock()
		_, _ = routerForgeReleaseSnapshot()
		return waitRouterForgeReleaseRefresh(10 * time.Second)
	}
	if routerForgeReleaseState.refreshing {
		routerForgeReleaseState.mu.Unlock()
		return waitRouterForgeReleaseRefresh(10 * time.Second)
	}
	routerForgeReleaseState.refreshing = true
	routerForgeReleaseState.lastAttempt = time.Now()
	routerForgeReleaseState.mu.Unlock()

	refreshRouterForgeReleaseIndex()
	routerForgeReleaseState.mu.Lock()
	status := routerForgeReleaseState.status
	routerForgeReleaseState.mu.Unlock()
	return status
}

func waitRouterForgeReleaseRefresh(timeout time.Duration) routerForgeReleaseStatus {
	deadline := time.Now().Add(timeout)
	for {
		routerForgeReleaseState.mu.Lock()
		refreshing := routerForgeReleaseState.refreshing
		status := routerForgeReleaseState.status
		routerForgeReleaseState.mu.Unlock()
		if !refreshing || time.Now().After(deadline) {
			return status
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func fetchRouterForgeReleaseIndexURL(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "RouterForge/"+version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release index HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, routerForgeReleaseMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > routerForgeReleaseMaxBytes {
		return nil, fmt.Errorf("release index exceeds %d bytes", routerForgeReleaseMaxBytes)
	}
	return data, nil
}

func refreshRouterForgeReleaseIndex() {
	client := &http.Client{Timeout: 8 * time.Second}
	var (
		data []byte
		url  string
		err  error
	)
	var attempts []string
	for _, candidate := range routerForgeReleaseIndexURLs() {
		data, err = fetchRouterForgeReleaseIndexURL(client, candidate)
		if err == nil {
			url = candidate
			break
		}
		attempts = append(attempts, candidate+": "+err.Error())
	}
	if url == "" {
		err = fmt.Errorf("release index unavailable: %s", strings.Join(attempts, "; "))
	}

	var doc routerForgeReleaseIndex
	if err == nil {
		doc, err = parseRouterForgeReleaseIndex(data)
	}
	if err == nil {
		cache := routerForgeReleaseCachePath()
		if mkErr := os.MkdirAll(filepath.Dir(cache), 0755); mkErr == nil {
			tmp := cache + ".tmp"
			if writeErr := os.WriteFile(tmp, data, 0644); writeErr == nil {
				_ = os.Rename(tmp, cache)
			} else {
				_ = os.Remove(tmp)
			}
		}
	}

	routerForgeReleaseState.mu.Lock()
	defer routerForgeReleaseState.mu.Unlock()
	routerForgeReleaseState.refreshing = false
	if err != nil {
		routerForgeReleaseState.status.Online = false
		routerForgeReleaseState.status.Error = err.Error()
		return
	}

	routerForgeReleaseState.doc = doc
	routerForgeReleaseState.status = routerForgeReleaseStatus{
		Channel:  doc.Channel,
		URL:      url,
		Source:   "remote",
		Online:   true,
		LastSync: time.Now().UTC().Format(time.RFC3339),
	}
}

func parseRouterForgeReleaseIndex(data []byte) (routerForgeReleaseIndex, error) {
	var doc routerForgeReleaseIndex
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, err
	}
	if doc.SchemaVersion != 1 {
		return doc, fmt.Errorf("unsupported release schema_version %d", doc.SchemaVersion)
	}
	if doc.Channel != normalizedReleaseChannel() {
		return doc, fmt.Errorf("unexpected release channel %q", doc.Channel)
	}
	if len(doc.Components) > 128 {
		return doc, fmt.Errorf("too many release components")
	}

	seen := map[string]struct{}{}
	for i := range doc.Components {
		release := &doc.Components[i]
		release.Channel = doc.Channel
		if !safeCatalogID(release.Package) || release.Version == "" {
			return doc, fmt.Errorf("invalid release package/version")
		}
		if release.Asset == "" || strings.Contains(release.Asset, "/") || strings.Contains(release.Asset, "..") {
			return doc, fmt.Errorf("%s: invalid asset", release.Package)
		}
		if len(release.SHA256) != 64 {
			return doc, fmt.Errorf("%s: invalid SHA256 length", release.Package)
		}
		if _, err := hex.DecodeString(release.SHA256); err != nil {
			return doc, fmt.Errorf("%s: invalid SHA256", release.Package)
		}
		if !validRouterForgeReleaseURL(release.URL) || !strings.HasSuffix(release.URL, "/"+release.Asset) {
			return doc, fmt.Errorf("%s: invalid release URL", release.Package)
		}
		if release.CanonicalURL != "" && (!validRouterForgeReleaseURL(release.CanonicalURL) || !strings.HasSuffix(release.CanonicalURL, "/"+release.Asset)) {
			return doc, fmt.Errorf("%s: invalid canonical release URL", release.Package)
		}
		if _, exists := seen[release.Package]; exists {
			return doc, fmt.Errorf("duplicate release package %s", release.Package)
		}
		seen[release.Package] = struct{}{}
	}
	return doc, nil
}

func applyRouterForgeReleaseIndex(snapshot *catalogSnapshot) {
	doc, status := routerForgeReleaseSnapshot()
	snapshot.Release = status

	byPackage := make(map[string]catalogRelease, len(doc.Components))
	for _, release := range doc.Components {
		byPackage[release.Package] = release
	}

	for i := range snapshot.Modules {
		item := &snapshot.Modules[i]
		pkg := routerForgePackageForItem(*item)
		if pkg == "" {
			continue
		}
		release, ok := byPackage[pkg]
		if !ok {
			continue
		}

		item.Release = release
		item.UpdateAvailable = item.Installed && item.Version != "" && item.Version != release.Version

		if item.ID == "routerforge-core" {
			item.Update = catalogInstallPlan{
				Method:     "routerforge-release",
				Repository: "routerforge-" + release.Channel,
				Packages:   []string{release.Package},
				Notes: []string{
					"Core обновляется отдельно от модулей.",
					"Asset и SHA256 берутся из release index выбранного канала.",
				},
			}
			continue
		}

		if item.Publisher.ID == "routerforge" || item.Managed {
			if len(item.Install.Packages) == 0 {
				item.Install.Packages = []string{release.Package}
			}
			if len(item.Update.Packages) == 0 {
				item.Update.Packages = []string{release.Package}
			}
		}
	}

	for i := range snapshot.Modules {
		snapshot.Modules[i].Actions = deriveCatalogActions(snapshot.Modules[i])
	}
	for i := range snapshot.Integrations {
		snapshot.Integrations[i].Actions = deriveCatalogActions(snapshot.Integrations[i])
	}
}

func routerForgePackageForItem(item catalogItem) string {
	if item.ID == "routerforge-core" {
		return "routerforge-core"
	}
	for _, pkg := range item.Detection.Packages {
		if strings.HasPrefix(pkg, "routerforge-") {
			return pkg
		}
	}
	for _, pkg := range item.Install.Packages {
		if strings.HasPrefix(pkg, "routerforge-") {
			return pkg
		}
	}
	return ""
}
