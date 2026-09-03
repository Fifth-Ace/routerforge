package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	marketplaceTestInstallMarker       = "/opt/etc/routerforge/package-management.enabled"
	legacyMarketplaceTestInstallMarker = "/opt/etc/dns-monitor/marketplace-test-install.enabled"
	marketplaceDownloadDir             = "/opt/tmp/routerforge-marketplace"
	marketplaceDownloadMaxBytes        = 64 << 20
)

var marketplaceInstallMu sync.Mutex

type catalogActionResult struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Action           string    `json:"action"`
	Packages         []string  `json:"packages"`
	Sources          []string  `json:"sources,omitempty"`
	Installed        bool      `json:"installed"`
	AlreadyInstalled bool      `json:"already_installed,omitempty"`
	Output           string    `json:"output,omitempty"`
	CompletedAt      time.Time `json:"completed_at"`
}

type catalogInstallResult = catalogActionResult

type catalogInstallFailure struct {
	Status  int
	Message string
	Detail  string
}

func (e *catalogInstallFailure) Error() string {
	if e.Detail != "" {
		return e.Message + ": " + e.Detail
	}
	return e.Message
}

func marketplaceTestInstallEnabled() bool {
	return pathExists(marketplaceTestInstallMarker) || pathExists(legacyMarketplaceTestInstallMarker)
}

func catalogItemByID(id string) (catalogItem, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return catalogItem{}, false
	}
	snapshot := readCatalog()
	for _, item := range snapshot.Modules {
		if item.ID == id {
			return item, true
		}
	}
	for _, item := range snapshot.Integrations {
		if item.ID == id {
			return item, true
		}
	}
	return catalogItem{}, false
}

func catalogModuleByID(id string) (catalogItem, bool) {
	item, ok := catalogItemByID(id)
	return item, ok && item.Kind == "module"
}

func safeCatalogPackageName(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-', r == '+', r == '.', r == '_':
		default:
			return false
		}
	}
	return true
}

func opkgExecutable() (string, error) {
	if info, err := os.Stat("/opt/bin/opkg"); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
		return "/opt/bin/opkg", nil
	}
	path, err := exec.LookPath("opkg")
	if err != nil {
		return "", fmt.Errorf("opkg not found")
	}
	return path, nil
}

func installCatalogModuleTest(ctx context.Context, id string) (catalogInstallResult, error) {
	return runCatalogModuleAction(ctx, id, "install", "")
}

func runCatalogModuleAction(ctx context.Context, id, action, confirmation string) (catalogActionResult, error) {
	marketplaceInstallMu.Lock()
	defer marketplaceInstallMu.Unlock()

	if !marketplaceTestInstallEnabled() {
		return catalogActionResult{}, &catalogInstallFailure{
			Status: 403, Message: "RouterForge package management is disabled",
			Detail: marketplaceTestInstallMarker + " is missing",
		}
	}

	item, ok := catalogItemByID(id)
	if !ok {
		return catalogActionResult{}, &catalogInstallFailure{Status: 404, Message: "catalog item not found"}
	}

	action = strings.TrimSpace(strings.ToLower(action))
	if action != "install" && action != "update" && action != "remove" {
		return catalogActionResult{}, &catalogInstallFailure{Status: 400, Message: "unsupported catalog action"}
	}
	if !catalogActionAllowed(item, action) {
		reason := item.Actions.Reason
		if reason == "" {
			reason = "lifecycle action is not approved or not declared"
		}
		return catalogActionResult{}, &catalogInstallFailure{Status: 403, Message: "catalog action is not allowed", Detail: reason}
	}
	if action == "remove" && confirmation != item.Name {
		return catalogActionResult{}, &catalogInstallFailure{Status: 400, Message: "typed removal confirmation does not match item name"}
	}

	plan := catalogPlanForAction(item, action)
	result := catalogActionResult{
		ID: item.ID, Name: item.Name, Action: action,
		Packages: append([]string(nil), plan.Packages...),
	}
	if len(result.Packages) == 0 {
		result.Packages = append([]string(nil), item.Detection.Packages...)
	}

	var log strings.Builder
	var err error
	switch plan.Method {
	case "routerforge-release":
		err = runRouterForgeReleasePlan(ctx, item, action, plan, &result, &log)
	case "opkg":
		err = runDirectOpkgPlan(ctx, action, plan, &result, &log)
	case "structured":
		err = runStructuredCatalogPlan(ctx, item, action, plan, &result, &log)
	default:
		err = fmt.Errorf("unsupported executable lifecycle method %q", plan.Method)
	}
	if err != nil {
		result.Output = truncateCatalogInstallOutput(log.String(), 16000)
		return result, &catalogInstallFailure{Status: 500, Message: action + " failed", Detail: truncateCatalogInstallOutput(err.Error(), 4000)}
	}

	updated, found := catalogItemByID(id)
	result.Installed = found && updated.Installed
	result.Output = truncateCatalogInstallOutput(log.String(), 16000)
	result.CompletedAt = time.Now()
	if action == "remove" && result.Installed {
		return result, &catalogInstallFailure{Status: 500, Message: "lifecycle completed but catalog still reports item as installed"}
	}
	if action != "remove" && !result.Installed {
		return result, &catalogInstallFailure{Status: 500, Message: "lifecycle completed but catalog still reports item as not installed"}
	}
	return result, nil
}

func catalogActionAllowed(item catalogItem, action string) bool {
	switch action {
	case "install":
		return item.Actions.Install
	case "update":
		return item.Actions.Update
	case "remove":
		return item.Actions.Remove
	default:
		return false
	}
}

func catalogPlanForAction(item catalogItem, action string) catalogInstallPlan {
	switch action {
	case "install":
		return item.Install
	case "update":
		return item.Update
	case "remove":
		return item.Remove
	default:
		return catalogInstallPlan{}
	}
}

func runRouterForgeReleasePlan(ctx context.Context, item catalogItem, action string, plan catalogInstallPlan, result *catalogActionResult, log *strings.Builder) error {
	if action == "remove" {
		return fmt.Errorf("routerforge-release does not implement remove")
	}
	release := item.Release
	if release.Version == "" || release.Package == "" || release.Asset == "" || release.SHA256 == "" || release.URL == "" {
		return fmt.Errorf("release index has no executable release for %s", item.ID)
	}
	if len(plan.Packages) > 0 {
		matched := false
		for _, pkg := range plan.Packages {
			if pkg == release.Package {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("release package %s is not declared by lifecycle plan", release.Package)
		}
	}

	opkg, err := opkgExecutable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(marketplaceDownloadDir, 0755); err != nil {
		return err
	}

	var (
		local     string
		actual    string
		sourceURL string
	)
	var downloadErrors []string
	for _, candidateURL := range routerForgeReleaseDownloadURLs(release) {
		local, actual, err = downloadVerifiedAsset(ctx, candidateURL, release.Asset, release.SHA256)
		if err == nil {
			sourceURL = candidateURL
			break
		}
		downloadErrors = append(downloadErrors, candidateURL+": "+err.Error())
	}
	if sourceURL == "" {
		return fmt.Errorf("verified RouterForge download failed: %s", strings.Join(downloadErrors, "; "))
	}
	defer os.Remove(local)

	result.Packages = []string{release.Package}
	result.Sources = append(result.Sources, "routerforge-"+release.Channel+":"+sourceURL+"#sha256="+actual)

	args := []string{"install", local}
	if action == "update" {
		args = []string{"--force-reinstall", "install", local}
	}
	output, runErr := exec.CommandContext(ctx, opkg, args...).CombinedOutput()
	fmt.Fprintf(log, "$ %s %s\n%s", opkg, strings.Join(args, " "), string(output))
	if runErr != nil {
		return fmt.Errorf("opkg %s: %s", action, strings.TrimSpace(string(output)))
	}
	return nil
}

func runDirectOpkgPlan(ctx context.Context, action string, plan catalogInstallPlan, result *catalogActionResult, log *strings.Builder) error {
	opkg, err := opkgExecutable()
	if err != nil {
		return err
	}
	if len(plan.Packages) == 0 {
		return fmt.Errorf("opkg lifecycle has no packages")
	}

	var verb string
	switch action {
	case "install":
		verb = "install"
	case "update":
		verb = "upgrade"
	case "remove":
		verb = "remove"
	default:
		return fmt.Errorf("unsupported opkg action %q", action)
	}
	args := append([]string{verb}, plan.Packages...)
	output, runErr := exec.CommandContext(ctx, opkg, args...).CombinedOutput()
	fmt.Fprintf(log, "$ %s %s\n%s", opkg, strings.Join(args, " "), string(output))
	result.Sources = append(result.Sources, "opkg:"+strings.Join(plan.Packages, ","))
	if runErr != nil {
		return fmt.Errorf("opkg %s: %s", action, strings.TrimSpace(string(output)))
	}
	return nil
}

func runStructuredCatalogPlan(ctx context.Context, item catalogItem, action string, plan catalogInstallPlan, result *catalogActionResult, log *strings.Builder) error {
	if len(plan.Steps) == 0 {
		return fmt.Errorf("structured lifecycle has no steps")
	}
	if item.ID == "nfqws2" && action == "install" {
		installed := readInstalledPackages()
		if _, legacy := installed["nfqws-keenetic"]; legacy {
			return fmt.Errorf("official nfqws2 instructions require legacy nfqws-keenetic to be removed before installation")
		}
	}

	opkg, err := opkgExecutable()
	if err != nil {
		return err
	}
	for _, step := range plan.Steps {
		stepErr := executeStructuredStep(ctx, opkg, step, log)
		if stepErr != nil && !step.IgnoreFailure {
			return stepErr
		}
		if stepErr != nil {
			fmt.Fprintf(log, "[ignored] %v\n", stepErr)
		}
	}
	result.Sources = append(result.Sources, "verified-manifest:"+item.ManifestSHA256)
	return nil
}

func executeStructuredStep(ctx context.Context, opkg string, step catalogLifecycleStep, log *strings.Builder) error {
	switch step.Type {
	case "write-opkg-feed":
		if !strings.HasPrefix(step.Path, "/opt/etc/opkg/") || strings.Contains(step.Path, "..") {
			return fmt.Errorf("unsafe opkg feed path")
		}
		content := strings.TrimSpace(step.Content)
		if !strings.HasPrefix(content, "src/gz ") || !strings.Contains(content, "https://") {
			return fmt.Errorf("invalid opkg feed content")
		}
		if err := os.MkdirAll(filepath.Dir(step.Path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(step.Path, []byte(content+"\n"), 0644); err != nil {
			return err
		}
		fmt.Fprintf(log, "$ write %s\n%s\n", step.Path, content)
		return nil
	case "opkg-update":
		return runOpkgStep(ctx, opkg, []string{"update"}, log)
	case "opkg-install", "opkg-upgrade", "opkg-remove":
		verb := strings.TrimPrefix(step.Type, "opkg-")
		args := []string{verb}
		for _, arg := range step.Args {
			if arg != "--autoremove" {
				return fmt.Errorf("unsupported opkg argument %q", arg)
			}
			args = append(args, arg)
		}
		for _, pkg := range step.Packages {
			if !safeCatalogPackageName(pkg) {
				return fmt.Errorf("unsafe package name %q", pkg)
			}
			args = append(args, pkg)
		}
		return runOpkgStep(ctx, opkg, args, log)
	default:
		return fmt.Errorf("unsupported structured step %q", step.Type)
	}
}

func runOpkgStep(ctx context.Context, opkg string, args []string, log *strings.Builder) error {
	output, err := exec.CommandContext(ctx, opkg, args...).CombinedOutput()
	fmt.Fprintf(log, "$ %s %s\n%s", opkg, strings.Join(args, " "), string(output))
	if err != nil {
		return fmt.Errorf("opkg %s: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return nil
}

func expandAssetTemplate(template, pkg, runtimeVersion string) string {
	if template == "" {
		template = "{package}_{version}_aarch64-3.10.ipk"
	}
	if !safeCatalogPackageName(pkg) || runtimeVersion == "" || strings.Contains(runtimeVersion, "/") || strings.Contains(runtimeVersion, "..") {
		return ""
	}
	value := strings.ReplaceAll(template, "{package}", pkg)
	value = strings.ReplaceAll(value, "{version}", runtimeVersion)
	if strings.Contains(value, "/") || strings.Contains(value, "..") || !strings.HasSuffix(value, ".ipk") {
		return ""
	}
	return value
}

func fetchChecksumList(ctx context.Context, url string) (map[string]string, error) {
	data, err := fetchSmallHTTPS(ctx, url, 1<<20)
	if err != nil {
		return nil, err
	}
	return parseChecksumList(string(data))
}

func parseChecksumList(value string) (map[string]string, error) {
	out := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(value))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || len(fields[0]) != 64 {
			continue
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if strings.Contains(name, "/") || strings.Contains(name, "..") {
			continue
		}
		out[name] = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("checksum list contains no usable entries")
	}
	return out, nil
}

func fetchSmallHTTPS(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("HTTPS required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "RouterForge/"+version)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response too large")
	}
	return data, nil
}

func downloadVerifiedAsset(ctx context.Context, url, asset, expected string) (string, string, error) {
	if !validRouterForgeReleaseURL(url) {
		return "", "", fmt.Errorf("RouterForge release asset must belong to the RouterForge GitHub repository")
	}
	if err := os.MkdirAll(marketplaceDownloadDir, 0755); err != nil {
		return "", "", err
	}
	path := filepath.Join(marketplaceDownloadDir, asset)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", "", err
	}
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cleanup()
		return "", "", err
	}
	req.Header.Set("User-Agent", "RouterForge/"+version)
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		cleanup()
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cleanup()
		return "", "", fmt.Errorf("download %s: HTTP %d", asset, resp.StatusCode)
	}

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, marketplaceDownloadMaxBytes+1))
	if err != nil {
		cleanup()
		return "", "", err
	}
	if written > marketplaceDownloadMaxBytes {
		cleanup()
		return "", "", fmt.Errorf("download %s exceeds size limit", asset)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", "", err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		_ = os.Remove(path)
		return "", actual, fmt.Errorf("SHA256 mismatch for %s", asset)
	}
	return path, actual, nil
}

func truncateCatalogInstallOutput(value string, max int) string {
	value = strings.TrimSpace(value)
	if max < 1 || len(value) <= max {
		return value
	}
	return value[:max] + "\n… output truncated …"
}
