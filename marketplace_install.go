package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	marketplaceTestInstallMarker = "/opt/etc/dns-monitor/marketplace-test-install.enabled"
	marketplaceLocalPackageDir   = "/opt/tmp"
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
	return pathExists(marketplaceTestInstallMarker)
}

func catalogModuleByID(id string) (catalogItem, bool) {
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
	return catalogItem{}, false
}

func catalogItemTestInstallable(item catalogItem) bool {
	if item.Kind != "module" || !item.Managed || item.Builtin {
		return false
	}
	if item.Install.Method != "opkg-feed" || item.Install.Repository != "dns-monitor" {
		return false
	}
	if len(item.Install.Packages) == 0 {
		return false
	}
	for _, pkg := range item.Install.Packages {
		if !safeCatalogPackageName(pkg) || !strings.HasPrefix(pkg, "dns-monitor-") {
			return false
		}
	}
	return true
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

type localIPKCandidate struct {
	path    string
	modTime time.Time
}

func findNewestLocalIPK(dir, pkg string) string {
	if !safeCatalogPackageName(pkg) {
		return ""
	}
	matches, _ := filepath.Glob(filepath.Join(dir, pkg+"_*.ipk"))
	candidates := make([]localIPKCandidate, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || info.IsDir() {
			continue
		}
		candidates = append(candidates, localIPKCandidate{path: match, modTime: info.ModTime()})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].modTime.Equal(candidates[j].modTime) {
			return candidates[i].path > candidates[j].path
		}
		return candidates[i].modTime.After(candidates[j].modTime)
	})
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0].path
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
			Status: 403, Message: "marketplace test package management is disabled",
			Detail: marketplaceTestInstallMarker + " is missing",
		}
	}

	item, ok := catalogModuleByID(id)
	if !ok {
		return catalogActionResult{}, &catalogInstallFailure{Status: 404, Message: "catalog module not found"}
	}
	if !catalogItemTestInstallable(item) {
		return catalogActionResult{}, &catalogInstallFailure{
			Status: 403, Message: "catalog item is not allowlisted for package management",
		}
	}

	action = strings.TrimSpace(strings.ToLower(action))
	switch action {
	case "install":
		if item.Installed {
			return catalogActionResult{
				ID: item.ID, Name: item.Name, Action: action,
				Packages:  append([]string(nil), item.Install.Packages...),
				Installed: true, AlreadyInstalled: true, CompletedAt: time.Now(),
			}, nil
		}
	case "update":
		if !item.Installed {
			return catalogActionResult{}, &catalogInstallFailure{Status: 409, Message: "module is not installed"}
		}
	case "remove":
		if !item.Installed {
			return catalogActionResult{
				ID: item.ID, Name: item.Name, Action: action,
				Packages:  append([]string(nil), item.Install.Packages...),
				Installed: false, CompletedAt: time.Now(),
			}, nil
		}
		if confirmation != item.Name {
			return catalogActionResult{}, &catalogInstallFailure{
				Status: 400, Message: "typed removal confirmation does not match module name",
			}
		}
	default:
		return catalogActionResult{}, &catalogInstallFailure{Status: 400, Message: "unsupported catalog action"}
	}

	opkg, err := opkgExecutable()
	if err != nil {
		return catalogActionResult{}, &catalogInstallFailure{Status: 500, Message: "opkg unavailable", Detail: err.Error()}
	}

	result := catalogActionResult{
		ID: item.ID, Name: item.Name, Action: action,
		Packages: append([]string(nil), item.Install.Packages...),
	}
	var log strings.Builder

	for _, pkg := range item.Install.Packages {
		args := make([]string, 0, 4)
		source := pkg
		sourceKind := "opkg-feed"
		local := findNewestLocalIPK(marketplaceLocalPackageDir, pkg)

		switch action {
		case "install":
			if local != "" {
				source = local
				sourceKind = "local-ipk"
			}
			args = []string{"install", source}
			result.Sources = append(result.Sources, sourceKind+":"+source)
		case "update":
			if local != "" {
				source = local
				sourceKind = "local-ipk"
				args = []string{"--force-reinstall", "install", source}
			} else {
				args = []string{"upgrade", pkg}
			}
			result.Sources = append(result.Sources, sourceKind+":"+source)
		case "remove":
			args = []string{"remove", pkg}
		}

		cmd := exec.CommandContext(ctx, opkg, args...)
		output, runErr := cmd.CombinedOutput()
		if log.Len() > 0 {
			log.WriteString("\n")
		}
		fmt.Fprintf(&log, "$ %s %s\n%s", opkg, strings.Join(args, " "), string(output))
		if runErr != nil {
			result.Output = truncateCatalogInstallOutput(log.String(), 16000)
			return result, &catalogInstallFailure{
				Status: 500, Message: "opkg " + action + " failed",
				Detail: truncateCatalogInstallOutput(strings.TrimSpace(string(output)), 4000),
			}
		}
	}

	updated, found := catalogModuleByID(id)
	result.Installed = found && updated.Installed
	result.Output = truncateCatalogInstallOutput(log.String(), 16000)
	result.CompletedAt = time.Now()

	if action == "remove" && result.Installed {
		return result, &catalogInstallFailure{Status: 500, Message: "package command completed but catalog still reports module as installed"}
	}
	if action != "remove" && !result.Installed {
		return result, &catalogInstallFailure{Status: 500, Message: "package command completed but catalog still reports module as not installed"}
	}
	return result, nil
}

func truncateCatalogInstallOutput(value string, max int) string {
	value = strings.TrimSpace(value)
	if max < 1 || len(value) <= max {
		return value
	}
	return value[:max] + "\n… output truncated …"
}
