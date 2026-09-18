package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	unmanagedGitHubRoot     = "/opt/lib/routerforge/apps"
	unmanagedGitHubMaxBytes = 64 << 20
)

type catalogUnmanagedGitHubAsset struct {
	Channel      string `json:"channel,omitempty"`
	Tag          string `json:"tag,omitempty"`
	Version      string `json:"version,omitempty"`
	Prerelease   bool   `json:"prerelease,omitempty"`
	ReleaseID    int64  `json:"release_id,omitempty"`
	AssetID      int64  `json:"asset_id,omitempty"`
	Asset        string `json:"asset,omitempty"`
	URL          string `json:"url,omitempty"`
	SizeBytes    uint64 `json:"size_bytes,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	PublishedAt  string `json:"published_at,omitempty"`
}

type catalogUnmanagedGitHub struct {
	Owner            string                       `json:"owner,omitempty"`
	Repo             string                       `json:"repo,omitempty"`
	Target           string                       `json:"target,omitempty"`
	RequestedChannel string                       `json:"requested_channel,omitempty"`
	SelectedChannel  string                       `json:"selected_channel,omitempty"`
	Stable           *catalogUnmanagedGitHubAsset `json:"release,omitempty"`
	Beta             *catalogUnmanagedGitHubAsset `json:"beta,omitempty"`
	Selected         *catalogUnmanagedGitHubAsset `json:"selected,omitempty"`
	ManualReason     string                       `json:"manual_reason,omitempty"`
}

type appSourceReleaseChannelRequest struct {
	Channel string `json:"channel"`
}

type githubReleaseAssetAPI struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               uint64 `json:"size"`
}

type githubReleaseAPI struct {
	ID          int64                   `json:"id"`
	TagName     string                  `json:"tag_name"`
	Draft       bool                    `json:"draft"`
	Prerelease  bool                    `json:"prerelease"`
	PublishedAt string                  `json:"published_at"`
	Assets      []githubReleaseAssetAPI `json:"assets"`
}

type unmanagedGitHubInstallMetadata struct {
	SchemaVersion int    `json:"schema_version"`
	SourceID      string `json:"source_id"`
	AppID         string `json:"app_id"`
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	Channel       string `json:"channel"`
	Tag           string `json:"tag"`
	Version       string `json:"version"`
	Asset         string `json:"asset"`
	URL           string `json:"url"`
	SHA256        string `json:"sha256"`
	SizeBytes     uint64 `json:"size_bytes"`
	InstalledAt   string `json:"installed_at"`
}

func normalizeUnmanagedReleaseChannel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "release":
		return "release"
	case "beta":
		return "beta"
	default:
		return "auto"
	}
}

func unmanagedTargetAliases(target string) ([]string, []string) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "aarch64-3.10":
		return []string{"arm64", "aarch64"}, []string{"armv7", "armhf", "arm32", "x86_64", "amd64", "x64", "i386", "i686", "mips", "mipsel", "mipsle"}
	case "mipsel-3.4":
		return []string{"mipsel", "mipsle"}, []string{"mips64", "mips64el", "mips64le", "arm64", "aarch64", "armv7", "x86_64", "amd64"}
	case "mips-3.4":
		return []string{"mips"}, []string{"mipsel", "mipsle", "mips64", "mips64el", "mips64le", "arm64", "aarch64", "armv7", "x86_64", "amd64"}
	default:
		return nil, nil
	}
}

func unmanagedRawBinaryAssetScore(name, target string) int {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return -1
	}

	for _, suffix := range []string{
		".zip", ".tar", ".tar.gz", ".tgz", ".tar.xz", ".txz", ".7z",
		".deb", ".rpm", ".apk", ".ipk", ".dmg", ".exe", ".msi",
		".sha256", ".sha256sum", ".sig", ".asc",
	} {
		if strings.HasSuffix(lower, suffix) {
			return -1
		}
	}

	for _, token := range []string{
		"windows", "win32", "win64", "_win", "-win",
		"darwin", "macos", "osx", "android", "ios",
	} {
		if strings.Contains(lower, token) {
			return -1
		}
	}

	aliases, conflicts := unmanagedTargetAliases(target)
	if len(aliases) == 0 {
		return -1
	}
	for _, token := range conflicts {
		if strings.Contains(lower, token) {
			return -1
		}
	}

	score := -1
	for _, alias := range aliases {
		if strings.Contains(lower, alias) {
			score = 100 + len(alias)
			break
		}
	}
	if score < 0 {
		return -1
	}
	if strings.Contains(lower, "linux") {
		score += 50
	}
	if strings.Contains(lower, "static") {
		score += 10
	}
	return score
}

func unmanagedReleaseAssetForTarget(release githubReleaseAPI, target, channel string) (*catalogUnmanagedGitHubAsset, bool) {
	bestScore := -1
	var best githubReleaseAssetAPI
	for _, asset := range release.Assets {
		score := unmanagedRawBinaryAssetScore(asset.Name, target)
		if score <= bestScore {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(asset.BrowserDownloadURL), "https://") {
			continue
		}
		bestScore = score
		best = asset
	}
	if bestScore < 0 {
		return nil, false
	}
	version := strings.TrimSpace(release.TagName)
	if strings.HasPrefix(strings.ToLower(version), "v") && len(version) > 1 {
		version = version[1:]
	}
	return &catalogUnmanagedGitHubAsset{
		Channel:      channel,
		Tag:          strings.TrimSpace(release.TagName),
		Version:      version,
		Prerelease:   release.Prerelease,
		ReleaseID:    release.ID,
		AssetID:      best.ID,
		Asset:        best.Name,
		URL:          best.BrowserDownloadURL,
		SizeBytes:    best.Size,
		Architecture: target,
		PublishedAt:  release.PublishedAt,
	}, true
}

func discoverUnmanagedGitHubReleases(ctx context.Context, owner, repo, target string) *catalogUnmanagedGitHub {
	meta := &catalogUnmanagedGitHub{
		Owner:            strings.TrimSpace(owner),
		Repo:             strings.TrimSpace(repo),
		Target:           strings.TrimSpace(target),
		RequestedChannel: "auto",
	}

	aliases, _ := unmanagedTargetAliases(target)
	if len(aliases) == 0 {
		meta.ManualReason = "RouterForge cannot map the current runtime architecture to a supported GitHub Release target. Manual installation only."
		return meta
	}

	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/releases?per_page=20",
		url.PathEscape(owner),
		url.PathEscape(repo),
	)
	data, err := appSourceFetchBytes(ctx, apiURL)
	if err != nil {
		meta.ManualReason = "GitHub Releases metadata is unavailable right now. Refresh the source later or use manual installation."
		return meta
	}

	var releases []githubReleaseAPI
	if err := json.Unmarshal(data, &releases); err != nil {
		meta.ManualReason = "GitHub Releases metadata is invalid. Manual installation only."
		return meta
	}

	for _, release := range releases {
		if release.Draft {
			continue
		}
		if release.Prerelease {
			if meta.Beta == nil {
				if candidate, ok := unmanagedReleaseAssetForTarget(release, target, "beta"); ok {
					meta.Beta = candidate
				}
			}
			continue
		}
		if meta.Stable == nil {
			if candidate, ok := unmanagedReleaseAssetForTarget(release, target, "release"); ok {
				meta.Stable = candidate
			}
		}
		if meta.Stable != nil && meta.Beta != nil {
			break
		}
	}

	switch {
	case meta.Stable == nil && meta.Beta != nil:
		meta.ManualReason = fmt.Sprintf(
			"No compatible stable raw Linux binary for %s. A compatible beta asset is available; select Beta explicitly in App Center Sources.",
			target,
		)
	case meta.Stable == nil && meta.Beta == nil:
		meta.ManualReason = fmt.Sprintf(
			"No compatible raw Linux binary for %s was found in GitHub Releases. Manual installation only.",
			target,
		)
	}
	return meta
}

func unmanagedGitHubAppPaths(item catalogItem) (root, binary, metadata, launcher string, err error) {
	sourceID := strings.TrimSpace(item.RegistrySource)
	appID := strings.ToLower(strings.TrimSpace(item.ManifestID))
	if !validAppSourceRecordID(sourceID) {
		return "", "", "", "", fmt.Errorf("invalid unmanaged source id")
	}
	if !safeCatalogPackageName(appID) || appID == "opkg" || strings.HasPrefix(appID, "routerforge-") {
		return "", "", "", "", fmt.Errorf("unmanaged app id is not safe for a launcher")
	}
	root = filepath.Join(unmanagedGitHubRoot, sourceID, appID)
	binary = filepath.Join(root, "app")
	metadata = filepath.Join(root, "metadata.json")
	launcher = filepath.Join("/opt/bin", appID)
	return root, binary, metadata, launcher, nil
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func selectUnmanagedGitHubRelease(item *catalogItem, channel string) {
	if item == nil || item.UnmanagedGitHub == nil {
		return
	}

	copyMeta := *item.UnmanagedGitHub
	item.UnmanagedGitHub = &copyMeta
	meta := item.UnmanagedGitHub

	meta.RequestedChannel = normalizeUnmanagedReleaseChannel(channel)
	meta.SelectedChannel = ""
	meta.Selected = nil

	switch meta.RequestedChannel {
	case "beta":
		meta.Selected = meta.Beta
	case "release", "auto":
		meta.Selected = meta.Stable
	}
	if meta.Selected != nil {
		meta.SelectedChannel = meta.Selected.Channel
	}

	item.Install = catalogInstallPlan{}
	item.Update = catalogInstallPlan{}
	item.Remove = catalogInstallPlan{Method: "github-release-binary"}
	item.Release = catalogRelease{}
	item.AvailableVersion = ""
	item.PackageMeta = nil
	item.UpdateAvailable = false

	// P20 deliberately removes the old "repo name == opkg package" guess for
	// manifestless GitHub sources. A compatible GitHub Release binary is the
	// only automatic unmanaged path.
	item.Detection.Packages = nil
	item.VersionSource = "github-release"

	if _, binary, _, _, pathErr := unmanagedGitHubAppPaths(*item); pathErr == nil {
		item.Detection.Paths = appendUniqueString(item.Detection.Paths, binary)
	}

	if meta.Selected == nil {
		item.Compatibility.Hints = appendUniqueString(item.Compatibility.Hints, unmanagedGitHubActionReason(*item))
		return
	}

	selected := meta.Selected
	item.Release = catalogRelease{
		Channel:      selected.Channel,
		Version:      selected.Version,
		Asset:        selected.Asset,
		URL:          selected.URL,
		Architecture: selected.Architecture,
		SizeBytes:    selected.SizeBytes,
	}
	item.AvailableVersion = selected.Version
	item.PackageMeta = &catalogPackageMetadata{
		Architecture:      selected.Architecture,
		DownloadSizeBytes: selected.SizeBytes,
		Source:            "github-release",
	}
	item.Compatibility.Targets = []string{selected.Architecture}
	item.Compatibility.Hints = appendUniqueString(
		item.Compatibility.Hints,
		"Architecture matched by GitHub Release asset name; runtime ABI compatibility is not guaranteed.",
	)

	notes := []string{
		"Unverified GitHub Release binary. Explicit unsafe-source confirmation is required.",
		"RouterForge downloads only the selected release asset and never executes upstream install scripts automatically.",
	}
	item.Install = catalogInstallPlan{
		Method:        "github-release-binary",
		RepositoryURL: item.ProjectURL,
		InstallerURL:  selected.URL,
		Notes:         append([]string(nil), notes...),
	}
	item.Update = catalogInstallPlan{
		Method:        "github-release-binary",
		RepositoryURL: item.ProjectURL,
		InstallerURL:  selected.URL,
		Notes:         append([]string(nil), notes...),
	}
	item.Remove = catalogInstallPlan{
		Method: "github-release-binary",
		Notes:  []string{"Removes only the RouterForge-managed binary, metadata and launcher for this source."},
	}
}

func applyUnmanagedGitHubInstalledMetadata(item *catalogItem) {
	if item == nil || item.UnmanagedGitHub == nil {
		return
	}
	_, binary, metadata, _, err := unmanagedGitHubAppPaths(*item)
	if err != nil {
		return
	}
	if info, statErr := os.Stat(binary); statErr != nil || info.IsDir() {
		return
	}
	data, readErr := os.ReadFile(metadata)
	if readErr != nil {
		item.Installed = false
		item.State = "available"
		return
	}
	var installed unmanagedGitHubInstallMetadata
	if err := json.Unmarshal(data, &installed); err != nil ||
		installed.SchemaVersion != 1 ||
		installed.SourceID != item.RegistrySource ||
		installed.AppID != item.ManifestID {
		item.Installed = false
		item.State = "available"
		return
	}

	item.Installed = true
	item.State = "installed_external"
	item.Version = installed.Version
	item.VersionSource = "github-release"
	if item.UnmanagedGitHub.Selected != nil && item.AvailableVersion != "" && item.Version != "" {
		item.UpdateAvailable = item.Version != item.AvailableVersion
	}
}

func unmanagedGitHubActionReason(item catalogItem) string {
	meta := item.UnmanagedGitHub
	if meta == nil {
		return ""
	}
	if meta.Selected != nil {
		return fmt.Sprintf(
			"Unverified GitHub %s asset %q matches %s. Explicit unsafe-source confirmation is required.",
			meta.Selected.Channel,
			meta.Selected.Asset,
			meta.Selected.Architecture,
		)
	}
	if meta.RequestedChannel == "beta" && meta.Beta == nil {
		return fmt.Sprintf("No compatible beta raw Linux binary for %s. Manual installation only.", meta.Target)
	}
	if meta.RequestedChannel == "release" && meta.Stable == nil {
		return fmt.Sprintf("No compatible stable raw Linux binary for %s. Manual installation only.", meta.Target)
	}
	if meta.RequestedChannel == "auto" && meta.Stable == nil && meta.Beta != nil {
		return fmt.Sprintf(
			"No compatible stable raw Linux binary for %s. Compatible beta %s (%s) is available; select Beta explicitly in App Center Sources.",
			meta.Target,
			meta.Beta.Tag,
			meta.Beta.Asset,
		)
	}
	if strings.TrimSpace(meta.ManualReason) != "" {
		return meta.ManualReason
	}
	return fmt.Sprintf("No compatible GitHub Release binary for %s. Manual installation only.", meta.Target)
}

func unverifiedGitHubReleasePlan(item catalogItem, action string) (catalogInstallPlan, bool) {
	if item.UnmanagedGitHub == nil || strings.ToLower(item.Trust.Status) != "unverified" {
		return catalogInstallPlan{}, false
	}
	switch action {
	case "install":
		if item.Installed || item.UnmanagedGitHub.Selected == nil || item.Install.Method != "github-release-binary" {
			return catalogInstallPlan{}, false
		}
		return item.Install, true
	case "update":
		if !item.Installed || !item.UpdateAvailable || item.UnmanagedGitHub.Selected == nil || item.Update.Method != "github-release-binary" {
			return catalogInstallPlan{}, false
		}
		return item.Update, true
	case "remove":
		if !item.Installed || item.Remove.Method != "github-release-binary" {
			return catalogInstallPlan{}, false
		}
		return item.Remove, true
	default:
		return catalogInstallPlan{}, false
	}
}

func validUnmanagedGitHubReleaseURL(item catalogItem, raw string) bool {
	if item.UnmanagedGitHub == nil {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Hostname(), "github.com") {
		return false
	}
	prefix := "/" + strings.Trim(item.UnmanagedGitHub.Owner, "/") +
		"/" + strings.Trim(item.UnmanagedGitHub.Repo, "/") +
		"/releases/download/"
	return strings.HasPrefix(strings.ToLower(u.Path), strings.ToLower(prefix))
}

func writeUnmanagedGitHubLauncher(launcher, binary string) error {
	if info, err := os.Lstat(launcher); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("launcher collision at %s", launcher)
		}
		target, readErr := os.Readlink(launcher)
		if readErr != nil || target != binary {
			return fmt.Errorf("launcher collision at %s", launcher)
		}
		if err := os.Remove(launcher); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	tmp := launcher + ".routerforge-tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(binary, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, launcher); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func removeUnmanagedGitHubInstall(item catalogItem, log *catalogActionLog) error {
	root, binary, _, launcher, err := unmanagedGitHubAppPaths(item)
	if err != nil {
		return err
	}

	if info, statErr := os.Lstat(launcher); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if target, readErr := os.Readlink(launcher); readErr == nil && target == binary {
				if err := os.Remove(launcher); err != nil {
					return err
				}
				log.EmitLine("Removed RouterForge-managed launcher " + launcher)
			} else {
				log.EmitLine("Skipped foreign launcher " + launcher)
			}
		} else {
			log.EmitLine("Skipped foreign launcher " + launcher)
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	}

	if err := os.RemoveAll(root); err != nil {
		return err
	}
	_ = os.Remove(filepath.Dir(root))
	log.EmitLine("Removed RouterForge-managed unmanaged app files")
	return nil
}

func runUnmanagedGitHubReleasePlan(
	ctx context.Context,
	item catalogItem,
	action string,
	plan catalogInstallPlan,
	result *catalogActionResult,
	log *catalogActionLog,
) error {
	if action == "remove" {
		return removeUnmanagedGitHubInstall(item, log)
	}
	if action != "install" && action != "update" {
		return fmt.Errorf("unsupported unmanaged GitHub action %q", action)
	}
	if item.UnmanagedGitHub == nil || item.UnmanagedGitHub.Selected == nil {
		return fmt.Errorf("no compatible GitHub Release asset selected")
	}

	selected := item.UnmanagedGitHub.Selected
	currentTarget := normalizedReleaseTarget()
	if selected.Architecture == "" || currentTarget == "" || selected.Architecture != currentTarget {
		return fmt.Errorf("release architecture %q does not match current target %q", selected.Architecture, currentTarget)
	}
	if plan.InstallerURL == "" || plan.InstallerURL != selected.URL || !validUnmanagedGitHubReleaseURL(item, selected.URL) {
		return fmt.Errorf("unmanaged GitHub release URL failed source binding")
	}

	root, binary, metadataPath, launcher, err := unmanagedGitHubAppPaths(item)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}

	if info, statErr := os.Lstat(launcher); statErr == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("launcher collision at %s", launcher)
		}
		target, readErr := os.Readlink(launcher)
		if readErr != nil || target != binary {
			return fmt.Errorf("launcher collision at %s", launcher)
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, selected.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "RouterForge-AppCenter/"+version)

	resp, err := newAppSourceHTTPClient(false).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub release asset HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > unmanagedGitHubMaxBytes {
		return fmt.Errorf("GitHub release asset exceeds %d bytes", unmanagedGitHubMaxBytes)
	}

	atomicFile, err := safety.NewAtomicFile(root, ".github-release-*")
	if err != nil {
		return err
	}
	defer atomicFile.Cleanup()

	file := atomicFile.File()
	if err := file.Chmod(0755); err != nil {
		return err
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, unmanagedGitHubMaxBytes+1))
	if err != nil {
		return err
	}
	if written > unmanagedGitHubMaxBytes {
		return fmt.Errorf("GitHub release asset exceeds %d bytes", unmanagedGitHubMaxBytes)
	}
	if selected.SizeBytes > 0 && uint64(written) != selected.SizeBytes {
		return fmt.Errorf("GitHub release asset size changed: expected %d got %d", selected.SizeBytes, written)
	}
	if err := atomicFile.Sync(); err != nil {
		return err
	}
	if err := atomicFile.Close(); err != nil {
		return err
	}
	if err := atomicFile.Publish(binary); err != nil {
		return err
	}

	actualSHA := hex.EncodeToString(hash.Sum(nil))
	metadata := unmanagedGitHubInstallMetadata{
		SchemaVersion: 1,
		SourceID:      item.RegistrySource,
		AppID:         item.ManifestID,
		Owner:         item.UnmanagedGitHub.Owner,
		Repo:          item.UnmanagedGitHub.Repo,
		Channel:       selected.Channel,
		Tag:           selected.Tag,
		Version:       selected.Version,
		Asset:         selected.Asset,
		URL:           selected.URL,
		SHA256:        actualSHA,
		SizeBytes:     uint64(written),
		InstalledAt:   time.Now().UTC().Format(time.RFC3339),
	}
	metaBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	metaBytes = append(metaBytes, '\n')
	if err := safety.WriteFileAtomic(metadataPath, metaBytes, 0644); err != nil {
		return err
	}
	if err := writeUnmanagedGitHubLauncher(launcher, binary); err != nil {
		_ = os.Remove(metadataPath)
		return err
	}

	log.EmitLine(fmt.Sprintf(
		"Installed unverified GitHub %s asset %s (%d bytes, sha256=%s)",
		selected.Channel,
		selected.Asset,
		written,
		actualSHA,
	))
	result.Sources = append(result.Sources, selected.URL+"#sha256="+actualSHA)
	return nil
}

func setAppSourceReleaseChannel(id, channel string) (appSourceRecord, error) {
	channel = normalizeUnmanagedReleaseChannel(channel)
	if !validAppSourceRecordID(id) {
		return appSourceRecord{}, fmt.Errorf("source not found")
	}

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
		if !cfg.Sources[i].Manifestless {
			return appSourceRecord{}, fmt.Errorf("release channel is available only for manifestless GitHub sources")
		}
		cfg.Sources[i].ReleaseChannel = channel
		if err := saveAppSourcesConfigUnlocked(cfg); err != nil {
			return appSourceRecord{}, err
		}
		return cfg.Sources[i], nil
	}
	return appSourceRecord{}, fmt.Errorf("source not found")
}

func unmanagedReleaseCandidatesSorted(meta *catalogUnmanagedGitHub) []*catalogUnmanagedGitHubAsset {
	if meta == nil {
		return nil
	}
	out := make([]*catalogUnmanagedGitHubAsset, 0, 2)
	if meta.Stable != nil {
		out = append(out, meta.Stable)
	}
	if meta.Beta != nil {
		out = append(out, meta.Beta)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Channel < out[j].Channel
	})
	return out
}
